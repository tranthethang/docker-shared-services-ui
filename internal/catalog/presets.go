package catalog

// Preset describes a curated group of services for quick selection.
type Preset struct {
	Name     string   `json:"name"`
	Summary  string   `json:"summary"`
	Services []string `json:"services"`
}

var builtInPresets = []Preset{
	{
		Name:     "Core Infra",
		Summary:  "Gateway, Postgres, Redis, and live log viewer",
		Services: []string{"traefik", "postgres", "redis", "dozzle"},
	},
	{
		Name:     "AI Dev",
		Summary:  "Vector stores, LLM proxy, and web crawler",
		Services: []string{"traefik", "pgvector", "qdrant", "chromadb", "litellm", "crawl4ai", "hindsight"},
	},
	{
		Name:     "Observability",
		Summary:  "OpenTelemetry, Dozzle logs, Bugsink, and Mailpit",
		Services: []string{"traefik", "otel", "dozzle", "bugsink", "mailpit"},
	},
	{
		Name:     "CI/CD",
		Summary:  "Gitea, runner, Woodpecker CI, and Verdaccio",
		Services: []string{"traefik", "gitea", "act_runner", "woodpecker", "verdaccio"},
	},
}

// FilterPresets returns built-in presets with their services filtered to activeServices.
func FilterPresets(activeServices []string) []Preset {
	active := make(map[string]struct{}, len(activeServices))
	for _, name := range activeServices {
		active[name] = struct{}{}
	}

	out := make([]Preset, 0, len(builtInPresets))
	for _, p := range builtInPresets {
		services := make([]string, 0, len(p.Services))
		for _, svc := range p.Services {
			if _, ok := active[svc]; ok {
				services = append(services, svc)
			}
		}
		out = append(out, Preset{
			Name:     p.Name,
			Summary:  p.Summary,
			Services: services,
		})
	}
	return out
}
