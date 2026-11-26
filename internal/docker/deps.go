package docker

import (
	"slices"
	"sort"
)

// StartOrder defines the infra-first startup order for batch operations.
var StartOrder = []string{
	"traefik",
	"postgres",
	"pgvector",
	"zitadel",
	"oauth2-proxy",
	"redis",
	"mysql8",
	"mariadb",
	"mongodb",
	"chromadb",
	"qdrant",
	"rabbitmq",
	"memcached",
	"minio",
	"garage",
	"kafka",
	"mailpit",
	"otel",
	"supabase",
	"hindsight",
	"hasura",
	"litellm",
}

// PackageRequires maps backend services to their direct dependent services.
var PackageRequires = map[string][]string{
	"postgres": {
		"bugsink",
		"concourse",
		"gitea",
		"hasura",
		"inngest",
		"jenkins",
		"litellm",
		"sonarqube",
		"temporal",
		"zitadel",
	},
	"pgvector": {"hindsight"},
	"redis":    {"inngest", "redisinsight"},
	"garage":   {"garage_ui"},
}

func orderIndex(name string) int {
	if idx := slices.Index(StartOrder, name); idx >= 0 {
		return idx
	}
	return 999
}

func sortByStartOrder(names []string) {
	sort.Slice(names, func(i, j int) bool {
		oi, oj := orderIndex(names[i]), orderIndex(names[j])
		if oi != oj {
			return oi < oj
		}
		return names[i] < names[j]
	})
}

// DirectDependencies returns the backends that service directly depends on.
// When available is non-nil, only backends present in available are returned.
func DirectDependencies(service string, available []string) []string {
	var found []string
	for backend, users := range PackageRequires {
		if backend == service || !slices.Contains(users, service) {
			continue
		}
		if available != nil && !slices.Contains(available, backend) {
			continue
		}
		found = append(found, backend)
	}
	sortByStartOrder(found)
	return slices.Compact(found)
}

// DependencyQueue returns all transitive backends that must be ready before service, in startup order.
func DependencyQueue(service string, available []string) []string {
	var seen []string
	var walk func(name string)
	walk = func(name string) {
		for _, dep := range DirectDependencies(name, available) {
			if !slices.Contains(seen, dep) {
				walk(dep)
				seen = append(seen, dep)
			}
		}
	}
	walk(service)
	return StartupQueue(seen)
}

// StartupQueue orders services topologically by dependency, breaking ties using StartOrder.
func StartupQueue(services []string) []string {
	var remaining []string
	for _, s := range services {
		if s != "" && !slices.Contains(remaining, s) {
			remaining = append(remaining, s)
		}
	}

	deps := make(map[string][]string, len(remaining))
	for _, s := range remaining {
		deps[s] = DirectDependencies(s, remaining)
	}

	queued := make([]string, 0, len(remaining))
	for len(remaining) > 0 {
		var ready []string
		for _, s := range remaining {
			hasPendingDep := false
			for _, d := range deps[s] {
				if slices.Contains(remaining, d) {
					hasPendingDep = true
					break
				}
			}
			if !hasPendingDep {
				ready = append(ready, s)
			}
		}
		if len(ready) == 0 {
			sortByStartOrder(remaining)
			queued = append(queued, remaining...)
			break
		}
		sortByStartOrder(ready)
		chosen := ready[0]
		queued = append(queued, chosen)
		remaining = slices.DeleteFunc(remaining, func(s string) bool { return s == chosen })
	}
	return queued
}

// ShutdownQueue returns services in reverse startup order, with traefik always stopped last.
func ShutdownQueue(services []string) []string {
	sorted := make([]string, len(services))
	copy(sorted, services)

	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a == b {
			return false
		}
		if a == "traefik" {
			return true
		}
		if b == "traefik" {
			return false
		}
		oa, ob := min(orderIndex(a), 500), min(orderIndex(b), 500)
		if oa != ob {
			return oa < ob
		}
		return a < b
	})
	slices.Reverse(sorted)
	return sorted
}
