// Package env provides environment secret masking and prerequisite validation checks.
package env

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// MaskToken is the replacement string used for redacted secret values.
	MaskToken      = "***"
	longValueLen   = 12
	minSecretValue = 3
)

var secretWords = []string{
	"PASS",
	"SECRET",
	"TOKEN",
	"KEY",
	"CREDENTIAL",
	"PRIVATE",
	"AUTH",
	"SALT",
	"DSN",
	"COOKIE",
}

// ShouldMask reports whether a key/value pair looks like a secret or long token that should be redacted.
func ShouldMask(key, value string) bool {
	if value == "" || strings.EqualFold(value, "true") || strings.EqualFold(value, "false") || isASCIIDigits(value) {
		return false
	}
	upper := strings.ToUpper(key)
	for _, w := range secretWords {
		if strings.Contains(upper, w) {
			return len(value) >= minSecretValue
		}
	}
	return len(value) >= longValueLen
}

func isASCIIDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

type assignment struct {
	key   string
	value string
}

func parseAssignments(text string) []assignment {
	lines := strings.Split(text, "\n")
	out := make([]assignment, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		val := strings.TrimSpace(v)
		val = strings.Trim(val, `"'`)
		out = append(out, assignment{key: key, value: val})
	}
	return out
}

// ValuesToMask collects secret-looking values from root .env and each service's .env, sorted longest-first.
func ValuesToMask(repoPath string, services []string) []string {
	paths := make([]string, 0, 1+len(services))
	paths = append(paths, filepath.Join(repoPath, ".env"))
	for _, svc := range services {
		if svc == "" || svc == "." {
			continue
		}
		paths = append(paths, filepath.Join(repoPath, svc, ".env"))
	}

	seen := make(map[string]struct{})
	values := make([]string, 0)
	for _, p := range paths {
		data, err := os.ReadFile(p) //nolint:gosec // p is derived from validated repoPath and service names
		if err != nil {
			continue
		}
		for _, a := range parseAssignments(string(data)) {
			if !ShouldMask(a.key, a.value) {
				continue
			}
			if _, exists := seen[a.value]; !exists {
				seen[a.value] = struct{}{}
				values = append(values, a.value)
			}
		}
	}

	// Longest first so a value that contains a shorter secret is masked whole.
	sort.Slice(values, func(i, j int) bool {
		if len(values[i]) != len(values[j]) {
			return len(values[i]) > len(values[j])
		}
		return values[i] < values[j]
	})
	return values
}

// MaskKnownValues replaces all occurrences of values in text with MaskToken.
func MaskKnownValues(text string, values []string) string {
	out := text
	for _, v := range values {
		if v == "" {
			continue
		}
		out = strings.ReplaceAll(out, v, MaskToken)
	}
	return out
}
