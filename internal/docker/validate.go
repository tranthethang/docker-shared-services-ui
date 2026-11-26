package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

// ValidateComposeAll runs `docker compose config -q` for each active service and returns failing service names.
func ValidateComposeAll(ctx context.Context, runner ComposeRunner, repoPath string, identity stack.Identity, services []string, say func(string)) []string {
	emit(say, "Validating Docker Compose configuration...")
	sorted := slices.Clone(services)
	slices.Sort(sorted)

	var failed []string
	for _, svc := range sorted {
		if ctx.Err() != nil {
			failed = append(failed, svc)
			break
		}
		composePath := filepath.Join(repoPath, svc, "docker-compose.yml")
		info, err := os.Stat(composePath)
		if err != nil || info.IsDir() {
			emit(say, fmt.Sprintf("%s: missing %s/docker-compose.yml", svc, svc))
			failed = append(failed, svc)
			continue
		}
		if err := runner.RunCompose(ctx, repoPath, identity.ProjectName(svc), svc, []string{"config", "-q"}, say); err != nil {
			emit(say, fmt.Sprintf("%s: compose config failed: %v", svc, err))
			failed = append(failed, svc)
			continue
		}
		emit(say, fmt.Sprintf("%s: compose config valid", svc))
	}
	return failed
}
