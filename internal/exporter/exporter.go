// Package exporter packages selected services into a self-contained stack under <repo>/dist/<name>/.
package exporter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

const maxStackNameLen = 48

var skipDirNames = []string{"data", "volumes", "__pycache__", ".venv", "target"}

// PreviewResult describes the selected services and auto-expanded dependencies.
type PreviewResult struct {
	Checked  []string `json:"checked"`
	Added    []string `json:"added"`
	Resolved []string `json:"resolved"`
}

// ExportResult describes a completed package export.
type ExportResult struct {
	Name       string   `json:"name"`
	OutputPath string   `json:"outputPath"`
	Services   []string `json:"services"`
	Added      []string `json:"added"`
}

// ValidateStackName validates that name is lowercase alphanumeric with single hyphens/underscores, 1-48 chars.
func ValidateStackName(name string) error {
	if name == "" || len(name) > maxStackNameLen {
		return fmt.Errorf("stack name must be 1-%d characters", maxStackNameLen)
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return errors.New("stack name cannot contain path separators or traversal")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		isAlpha := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		if i == 0 && !isAlpha {
			return errors.New("stack name must start with a lowercase letter")
		}
		if !isAlpha && !isDigit && c != '-' && c != '_' {
			return errors.New("stack name must contain only lowercase letters, digits, '-' or '_'")
		}
	}
	return nil
}

// Preview expands transitive PACKAGE_REQUIRES dependencies and traefik when required.
func Preview(repoPath string, selected, available []string) (PreviewResult, error) {
	if len(selected) == 0 {
		return PreviewResult{}, errors.New("no services selected")
	}
	for _, svc := range selected {
		if !slices.Contains(available, svc) {
			return PreviewResult{}, fmt.Errorf("unknown service: %s", svc)
		}
	}
	chosen := slices.Clone(selected)
	for {
		var added []string
		for backend, users := range docker.PackageRequires {
			if slices.Contains(chosen, backend) || !slices.Contains(available, backend) {
				continue
			}
			for _, u := range users {
				if slices.Contains(chosen, u) {
					added = append(added, backend)
					break
				}
			}
		}
		if !slices.Contains(chosen, "traefik") && slices.Contains(available, "traefik") && anyNeedsTraefik(repoPath, chosen) {
			added = append(added, "traefik")
		}
		if len(added) == 0 {
			break
		}
		chosen = append(chosen, added...)
	}

	resolved := docker.StartupQueue(chosen)
	var checked, autoAdded []string
	for _, svc := range resolved {
		if slices.Contains(selected, svc) {
			checked = append(checked, svc)
		} else {
			autoAdded = append(autoAdded, svc)
		}
	}
	return PreviewResult{Checked: checked, Added: autoAdded, Resolved: resolved}, nil
}

func anyNeedsTraefik(repoPath string, services []string) bool {
	for _, svc := range services {
		data, err := os.ReadFile(filepath.Join(repoPath, svc, "docker-compose.yml")) //nolint:gosec // svc is validated
		if err == nil && strings.Contains(string(data), "traefik.enable=true") {
			return true
		}
	}
	return false
}

// Export writes a self-contained stack directory to <repoPath>/dist/<name>/.
func Export(repoPath, name string, selected, available []string) (ExportResult, error) {
	if err := ValidateStackName(name); err != nil {
		return ExportResult{}, err
	}
	preview, err := Preview(repoPath, selected, available)
	if err != nil {
		return ExportResult{}, err
	}
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return ExportResult{}, err
	}
	distRoot := filepath.Join(absRepo, "dist")
	dest := filepath.Join(distRoot, name)
	rel, err := filepath.Rel(distRoot, dest)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ExportResult{}, errors.New("invalid export destination path")
	}
	if _, err := os.Stat(dest); err == nil {
		return ExportResult{}, fmt.Errorf("%s already exists", dest)
	}
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return ExportResult{}, err
	}
	if err := buildExport(absRepo, dest, name, preview.Resolved); err != nil {
		_ = os.RemoveAll(dest)
		return ExportResult{}, err
	}
	return ExportResult{
		Name:       name,
		OutputPath: dest,
		Services:   preview.Resolved,
		Added:      preview.Added,
	}, nil
}

func buildExport(repoRoot, dest, name string, services []string) error {
	for _, svc := range services {
		src := filepath.Join(repoRoot, svc)
		info, err := os.Stat(src)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("service directory not found: %s", svc)
		}
		if err := copyServiceTree(src, filepath.Join(dest, svc), svc, repoRoot, ""); err != nil {
			return err
		}
	}
	sharedSrc := filepath.Join(repoRoot, "docker-compose.shared.yml")
	if data, err := os.ReadFile(sharedSrc); err == nil { //nolint:gosec // sharedSrc is inside validated repoRoot
		if err := os.WriteFile(filepath.Join(dest, "docker-compose.shared.yml"), data, 0o600); err != nil { //nolint:gosec // dest is validated inside <repo>/dist/<name>
			return err
		}
	}
	if exData, err := os.ReadFile(filepath.Join(repoRoot, ".env.example")); err == nil { //nolint:gosec // repoRoot is validated
		if err := os.WriteFile(filepath.Join(dest, ".env.example"), exData, 0o600); err != nil { //nolint:gosec // dest is validated inside <repo>/dist/<name>
			return err
		}
	}
	manifest, err := json.MarshalIndent(map[string][]string{"services": services}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dest, stack.ManifestFile), append(manifest, '\n'), 0o600); err != nil {
		return err
	}
	readme := fmt.Sprintf("# %s\n\nPackaged stack with services: %s\n", name, strings.Join(services, ", "))
	return os.WriteFile(filepath.Join(dest, "README.md"), []byte(readme), 0o600)
}

func copyServiceTree(src, dst, svc, repoRoot, rel string) error {
	if err := os.MkdirAll(dst, 0o750); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		subRel := filepath.Join(rel, name)
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if entry.IsDir() {
			if slices.Contains(skipDirNames, name) || strings.HasSuffix(name, "_data") {
				continue
			}
			if err := copyServiceTree(srcPath, dstPath, svc, repoRoot, subRel); err != nil {
				return err
			}
			continue
		}
		if shouldSkipFile(svc, subRel, name) {
			continue
		}
		data, err := os.ReadFile(srcPath) //nolint:gosec // srcPath is inside validated repoRoot
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0o600); err != nil { //nolint:gosec // dstPath is inside validated dest directory
			return err
		}
	}
	return nil
}

func shouldSkipFile(svc, rel, name string) bool {
	if name == ".env" {
		return true
	}
	if svc == "traefik" && filepath.Dir(rel) == "certs" {
		ext := filepath.Ext(name)
		return ext == ".crt" || ext == ".key"
	}
	return false
}
