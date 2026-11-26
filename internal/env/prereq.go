package env

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

const (
	// LegacyPlaceholderPassword is the old repository default password rejected by setup and lifecycle checks.
	LegacyPlaceholderPassword = "Password102!"
	// PasswordMinLength is the minimum required length for strong passwords (e.g. Zitadel).
	PasswordMinLength = 15
)

var invalidPasswordValues = []string{LegacyPlaceholderPassword}

var validationRules = map[string][]string{
	"garage":       {"GARAGE_RPC_SECRET", "GARAGE_ADMIN_TOKEN"},
	"garage_ui":    {"GARAGE_ADMIN_TOKEN", "GARAGE_UI_AUTH_USER_PASS"},
	"hasura":       {"HASURA_GRAPHQL_ADMIN_SECRET"},
	"hindsight":    {"HINDSIGHT_API_LLM_API_KEY"},
	"inngest":      {"INNGEST_EVENT_KEY", "INNGEST_SIGNING_KEY"},
	"litellm":      {"LITELLM_MASTER_KEY", "LITELLM_SALT_KEY", "POSTGRES16_PASSWORD"},
	"minio":        {"MINIO_ROOT_PASSWORD"},
	"mongodb":      {"MONGO_ROOT_PASSWORD"},
	"mysql8":       {"MYSQL_ROOT_PASSWORD"},
	"oauth2-proxy": {"OAUTH2_PROXY_COOKIE_SECRET"},
	"pgvector":     {"PGVECTOR_PASSWORD"},
	"postgres":     {"POSTGRES16_PASSWORD"},
	"rabbitmq":     {"RABBITMQ_PASSWORD"},
	"traefik":      {"TRAEFIK_DASHBOARD_AUTH"},
	"verdaccio":    {"VERDACCIO_PASSWORD"},
	"woodpecker":   {"WOODPECKER_AGENT_SECRET"},
	"zitadel":      {"ZITADEL_MASTERKEY"},
}

var strongPasswordRequired = map[string][]string{
	"zitadel": {"ZITADEL_ADMIN_PASSWORD"},
}

// Problem describes a prerequisite failure blocking service startup.
type Problem struct {
	Service     string `json:"service"`
	Code        string `json:"code"`
	Key         string `json:"key,omitempty"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

// ValidatePasswordStrength returns a reason if password fails strength rules, or empty string when valid.
func ValidatePasswordStrength(password string) string {
	if utf8.RuneCountInString(password) < PasswordMinLength {
		return fmt.Sprintf("Password must be at least %d characters.", PasswordMinLength)
	}
	var hasLower, hasUpper, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			hasSpecial = true
		}
	}
	switch {
	case !hasLower:
		return "Password must contain a lowercase letter."
	case !hasUpper:
		return "Password must contain an uppercase letter."
	case !hasDigit:
		return "Password must contain a digit."
	case !hasSpecial:
		return "Password must contain a special character."
	default:
		return ""
	}
}

// PrerequisiteProblems checks root .env, service .env files, mandatory vars, and strong password rules.
func PrerequisiteProblems(repoPath string, services []string) []Problem {
	rootEnv := filepath.Join(repoPath, stack.RootEnvFile)
	if !fileExists(rootEnv) {
		return []Problem{{
			Service:     ".",
			Code:        "root_env_missing",
			Message:     "Root environment file (.env) not found.",
			Remediation: "Run 'make setup' to initialize .env from .env.example.",
		}}
	}

	var problems []Problem
	for _, svc := range services {
		if svc == "" || svc == "." {
			continue
		}
		svcEnv := filepath.Join(repoPath, svc, ".env")
		if !fileExists(svcEnv) {
			problems = append(problems, Problem{
				Service:     svc,
				Code:        "service_env_missing",
				Message:     fmt.Sprintf("%s: Environment file not found (%s).", svc, svcEnv),
				Remediation: fmt.Sprintf("Run 'make setup' or copy %s/.env.example to %s.", svc, svcEnv),
			})
			continue
		}

		for _, key := range validationRules[svc] {
			val, _ := stack.ReadEnvValue(svcEnv, key)
			if val == "" || slices.Contains(invalidPasswordValues, val) {
				problems = append(problems, Problem{
					Service:     svc,
					Code:        "invalid_env_var",
					Key:         key,
					Message:     fmt.Sprintf("%s: Mandatory variable %s is unset or contains placeholder in %s.", svc, key, svcEnv),
					Remediation: fmt.Sprintf("Run 'make setup' to generate credentials or configure %s in %s.", key, svcEnv),
				})
			}
		}

		for _, key := range strongPasswordRequired[svc] {
			val, _ := stack.ReadEnvValue(svcEnv, key)
			if val == "" {
				continue
			}
			if reason := ValidatePasswordStrength(val); reason != "" {
				problems = append(problems, Problem{
					Service:     svc,
					Code:        "weak_password",
					Key:         key,
					Message:     fmt.Sprintf("%s: %s in %s is too weak. %s", svc, key, svcEnv, reason),
					Remediation: "Rotate the shared password in .env and re-run setup.",
				})
			}
		}
	}
	return problems
}

// LabelOnlyReason returns a structured code and human-readable reason if a service is locked/unconfigured.
func LabelOnlyReason(repoPath, service string) (string, string) {
	if service != "oauth2-proxy" {
		return "", ""
	}
	envPath := filepath.Join(repoPath, "oauth2-proxy", ".env")
	if !fileExists(envPath) {
		return "oauth2_env_missing", "oauth2-proxy/.env is missing — run make setup"
	}
	id, okID := stack.ReadEnvValue(envPath, "OAUTH2_PROXY_CLIENT_ID")
	secret, okSecret := stack.ReadEnvValue(envPath, "OAUTH2_PROXY_CLIENT_SECRET")
	if !okID || strings.TrimSpace(id) == "" || !okSecret || strings.TrimSpace(secret) == "" {
		return "oauth2_unconfigured", "OIDC client not set — fill OAUTH2_PROXY_CLIENT_ID and OAUTH2_PROXY_CLIENT_SECRET in oauth2-proxy/.env"
	}
	return "", ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
