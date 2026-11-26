package env

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

// FileStatus describes the .env and .env.example state for one service (or "." for root).
type FileStatus struct {
	Service    string `json:"service"`
	HasEnv     bool   `json:"hasEnv"`
	HasExample bool   `json:"hasExample"`
	Missing    bool   `json:"missing"`
}

// Assignment represents a KEY=VALUE entry parsed from an env file.
type Assignment struct {
	Key   string
	Value string
}

// ServiceEnvTargets returns "." followed by each valid service name.
func ServiceEnvTargets(services []string) []string {
	out := make([]string, 0, 1+len(services))
	out = append(out, ".")
	for _, s := range services {
		if s != "" && s != "." && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// CheckEnv inspects .env and .env.example existence for root and each service.
func CheckEnv(repoPath string, services []string) []FileStatus {
	targets := ServiceEnvTargets(services)
	statuses := make([]FileStatus, 0, len(targets))
	for _, s := range targets {
		hasEnv := fileExists(filepath.Join(repoPath, s, stack.RootEnvFile))
		hasExample := fileExists(filepath.Join(repoPath, s, ".env.example"))
		statuses = append(statuses, FileStatus{
			Service:    s,
			HasEnv:     hasEnv,
			HasExample: hasExample,
			Missing:    !hasEnv && hasExample,
		})
	}
	return statuses
}

// CreateMissingEnv copies .env.example to .env where .env is missing. Returns created service dirs.
func CreateMissingEnv(repoPath string, services []string, say func(string)) ([]string, error) {
	var created []string
	for _, s := range ServiceEnvTargets(services) {
		src := filepath.Join(repoPath, s, ".env.example")
		dst := filepath.Join(repoPath, s, stack.RootEnvFile)
		if !fileExists(src) || fileExists(dst) {
			continue
		}
		data, err := os.ReadFile(src) //nolint:gosec // src is derived from validated repoPath and service names
		if err != nil {
			return created, fmt.Errorf("read %s: %w", src, err)
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil { //nolint:gosec // dst is derived from validated repoPath and service names
			return created, fmt.Errorf("write %s: %w", dst, err)
		}
		created = append(created, s)
		emit(say, fmt.Sprintf("Created %s/.env", s))
	}
	return created, nil
}

// ParseRawAssignments extracts KEY=VALUE pairs from env file text without stripping quotes.
func ParseRawAssignments(text string) []Assignment {
	lines := strings.Split(text, "\n")
	out := make([]Assignment, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		k, v, ok := strings.Cut(line, "=")
		if !ok || !isValidEnvKey(k) {
			continue
		}
		out = append(out, Assignment{Key: k, Value: v})
	}
	return out
}

func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
		isDigit := c >= '0' && c <= '9'
		if i == 0 && !isAlpha {
			return false
		}
		if !isAlpha && !isDigit {
			return false
		}
	}
	return true
}

// MissingKeys returns (key, value) pairs present in example that current does not define.
func MissingKeys(exampleText, currentText string) []Assignment {
	have := make(map[string]struct{})
	for _, a := range ParseRawAssignments(currentText) {
		have[a.Key] = struct{}{}
	}
	seen := make(map[string]struct{})
	var missing []Assignment
	for _, a := range ParseRawAssignments(exampleText) {
		if _, dup := seen[a.Key]; dup {
			continue
		}
		seen[a.Key] = struct{}{}
		if _, ok := have[a.Key]; !ok {
			missing = append(missing, a)
		}
	}
	return missing
}

// MergeMissingEnv appends keys from .env.example that .env lacks without overwriting existing values.
func MergeMissingEnv(repoPath string, services []string, say func(string)) (int, error) {
	added := 0
	for _, s := range ServiceEnvTargets(services) {
		exPath := filepath.Join(repoPath, s, ".env.example")
		envPath := filepath.Join(repoPath, s, stack.RootEnvFile)
		exData, err := os.ReadFile(exPath) //nolint:gosec // exPath is derived from validated repoPath and service names
		if err != nil {
			continue
		}
		curData, err := os.ReadFile(envPath) //nolint:gosec // envPath is derived from validated repoPath and service names
		if err != nil {
			continue
		}
		missing := MissingKeys(string(exData), string(curData))
		if len(missing) == 0 {
			continue
		}
		content := string(curData)
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if strings.TrimSpace(content) != "" {
			content += "\n"
		}
		var sb strings.Builder
		sb.WriteString(content)
		for _, m := range missing {
			sb.WriteString(m.Key)
			sb.WriteByte('=')
			sb.WriteString(m.Value)
			sb.WriteByte('\n')
			added++
			emit(say, fmt.Sprintf("%s: added %s", s, m.Key))
		}
		if err := os.WriteFile(envPath, []byte(sb.String()), 0o600); err != nil {
			return added, fmt.Errorf("write %s: %w", envPath, err)
		}
	}
	return added, nil
}

// SetEnvValue replaces KEY=... in place or appends it when absent.
func SetEnvValue(envPath, key, value string) error {
	data, err := os.ReadFile(envPath) //nolint:gosec // envPath is derived from validated repoPath and service names
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(data) == 0 {
		lines = nil
	}
	var out []string
	replaced := false
	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		parsed := ParseRawAssignments(line)
		isKey := len(parsed) > 0 && parsed[0].Key == key
		if isKey && !replaced {
			out = append(out, key+"="+value)
			replaced = true
		} else if !isKey {
			out = append(out, line)
		}
	}
	if !replaced {
		out = append(out, key+"="+value)
	}
	return os.WriteFile(envPath, []byte(strings.Join(out, "\n")+"\n"), 0o600) //nolint:gosec // envPath is derived from validated repoPath and service names
}

func emit(say func(string), msg string) {
	if say != nil {
		say(msg)
	}
}
