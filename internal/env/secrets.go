package env

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

const (
	legacyDashboardHash   = "admin:$apr1$dsslocal$d6fWuAuh2eC9xV6eu7kMP."
	garageUIAuthHolder    = "N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	defaultPasswordLength = 32
)

type secretMethod int

const (
	methodHex secretMethod = iota
	methodBase64
	methodHtpasswd
	methodJwtHS256
)

type autoSecretSpec struct {
	key    string
	method secretMethod
	nbytes int
}

var autoSecrets = map[string][]autoSecretSpec{
	".":            {{"TRAEFIK_DASHBOARD_AUTH", methodHtpasswd, 16}},
	"bugsink":      {{"BUGSINK_SECRET_KEY", methodBase64, 50}},
	"garage":       {{"GARAGE_RPC_SECRET", methodHex, 32}, {"GARAGE_ADMIN_TOKEN", methodBase64, 32}},
	"inngest":      {{"INNGEST_EVENT_KEY", methodHex, 8}, {"INNGEST_SIGNING_KEY", methodHex, 8}},
	"litellm":      {{"LITELLM_MASTER_KEY", methodHex, 32}, {"LITELLM_SALT_KEY", methodHex, 32}},
	"oauth2-proxy": {{"OAUTH2_PROXY_COOKIE_SECRET", methodHex, 16}},
	"supabase": {
		{"JWT_SECRET", methodHex, 32}, {"SECRET_KEY_BASE", methodBase64, 48},
		{"REALTIME_DB_ENC_KEY", methodHex, 8}, {"PG_META_CRYPTO_KEY", methodHex, 32},
		{"S3_PROTOCOL_ACCESS_KEY_ID", methodHex, 16}, {"S3_PROTOCOL_ACCESS_KEY_SECRET", methodHex, 32},
		{"ANON_KEY", methodJwtHS256, 0}, {"SERVICE_ROLE_KEY", methodJwtHS256, 0},
	},
	"traefik":    {{"TRAEFIK_DASHBOARD_AUTH", methodHtpasswd, 16}},
	"woodpecker": {{"WOODPECKER_AGENT_SECRET", methodHex, 32}},
	"zitadel":    {{"ZITADEL_MASTERKEY", methodHex, 16}},
}

var sharedPasswordPlaceholders = []string{"", "change-me-generate-with-openssl-rand-base64-50", "replace_with_openssl_rand_hex_32"}

var knownSecretPlaceholders = []string{
	"MasterkeyNeedsToHave32Characters", "change-me-generate-with-openssl-rand-base64-50",
	"00000000000000000000000000000000", "your-super-secret-jwt-token-with-at-least-32-characters-long",
	"UpNVntn3cDxHJpq99YMc1T1AQgQpc8kfYTuRgBiYa15BLrx8etQoXz3gZv1/u2oq", "supabaserealtime",
	"your-encryption-key-32-chars-min", "625729a08b95bf1b7ff351a663f3a23c",
	"850181e4652dd023b7a98c58ae0d2d34bd487ee0cc3254aed6eda37307425907",
}

var sharedPasswordKeys = map[string][]string{
	".": {
		"DSS_SHARED_PASSWORD", "POSTGRES16_PASSWORD", "PGVECTOR_PASSWORD", "MYSQL_PASSWORD",
		"MYSQL_ROOT_PASSWORD", "MARIADB_PASSWORD", "MARIADB_ROOT_PASSWORD", "MONGO_ROOT_PASSWORD",
		"REDIS_PASSWORD", "RABBITMQ_PASSWORD", "MINIO_ROOT_PASSWORD", "GRAFANA_ADMIN_PASSWORD",
		"QDRANT_API_KEY", "JWT_SIGNING_KEY", "CENTRIFUGO_API_KEY",
	},
	"bugsink": {"POSTGRES16_PASSWORD"}, "centrifugo": {"CENTRIFUGO_API_KEY", "JWT_SIGNING_KEY"},
	"concourse": {"POSTGRES16_PASSWORD"}, "gitea": {"POSTGRES16_PASSWORD"},
	"hasura": {"POSTGRES16_PASSWORD", "HASURA_GRAPHQL_ADMIN_SECRET"}, "hindsight": {"PGVECTOR_PASSWORD"},
	"inngest": {"POSTGRES16_PASSWORD", "REDIS_PASSWORD"}, "jenkins": {"POSTGRES16_PASSWORD"},
	"litellm": {"POSTGRES16_PASSWORD"}, "mariadb": {"MARIADB_PASSWORD", "MARIADB_ROOT_PASSWORD"},
	"minio": {"MINIO_ROOT_PASSWORD"}, "mongodb": {"MONGO_ROOT_PASSWORD"},
	"mysql8": {"MYSQL_PASSWORD", "MYSQL_ROOT_PASSWORD"}, "otel": {"GRAFANA_ADMIN_PASSWORD"},
	"pgvector": {"PGVECTOR_PASSWORD"}, "postgres": {"POSTGRES16_PASSWORD"},
	"qdrant": {"QDRANT_API_KEY"}, "rabbitmq": {"RABBITMQ_PASSWORD"},
	"redis": {"REDIS_PASSWORD"}, "sonarqube": {"POSTGRES16_PASSWORD"},
	"supabase": {"POSTGRES_PASSWORD", "DASHBOARD_PASSWORD"}, "temporal": {"POSTGRES16_PASSWORD", "TEMPORAL_S3_SECRET_KEY"},
	"verdaccio": {"VERDACCIO_PASSWORD"}, "zitadel": {"ZITADEL_ADMIN_PASSWORD", "POSTGRES16_PASSWORD"},
}

var sharedPasswordTemplates = [][3]string{
	{".", "CONCOURSE_ADD_LOCAL_USER", "admin:{password}"},
	{"concourse", "CONCOURSE_ADD_LOCAL_USER", "admin:{password}"},
	{"zitadel", "ZITADEL_CACHES_CONNECTORS_REDIS_URL", "redis://:{password}@redis:6379/0"},
}

// FillSharedPassword applies one shared password across root and service .env files.
func FillSharedPassword(repoPath, explicit string, force bool, say func(string)) (string, error) {
	resolved, err := resolveSharedPassword(repoPath, explicit, say)
	if err != nil {
		return "", err
	}
	if slices.Contains(invalidPasswordValues, resolved) {
		return "", fmt.Errorf("%s is an insecure placeholder; choose a secure password", LegacyPlaceholderPassword)
	}
	rootEnv := filepath.Join(repoPath, stack.RootEnvFile)
	if !fileExists(rootEnv) {
		_, _ = CreateMissingEnv(repoPath, nil, say)
	}
	updated, skipped := 0, 0
	for _, target := range sharedPasswordTargets(repoPath, resolved) {
		p := filepath.Join(repoPath, target[0], stack.RootEnvFile)
		cur, ok := stack.ReadEnvValue(p, target[1])
		if !shouldOverwritePassword(target[1], cur, ok, force) {
			skipped++
			continue
		}
		if err := SetEnvValue(p, target[1], target[2]); err != nil {
			return "", err
		}
		updated++
		emit(say, fmt.Sprintf("%s/.env: updated %s", target[0], target[1]))
	}
	emit(say, fmt.Sprintf("Shared password sync complete (updated=%d, skipped_custom=%d).", updated, skipped))
	return resolved, nil
}

func resolveSharedPassword(repoPath, explicit string, say func(string)) (string, error) {
	if p := strings.TrimSpace(explicit); p != "" {
		if reason := ValidatePasswordStrength(p); reason != "" {
			return "", errors.New(reason)
		}
		return p, nil
	}
	existing, _ := stack.ReadEnvValue(filepath.Join(repoPath, stack.RootEnvFile), "DSS_SHARED_PASSWORD")
	if existing != "" && !slices.Contains(invalidPasswordValues, existing) {
		if reason := ValidatePasswordStrength(existing); reason != "" {
			emit(say, fmt.Sprintf("Warning: root .env DSS_SHARED_PASSWORD: %s (kept as-is)", reason))
		}
		return existing, nil
	}
	return GeneratePassword(defaultPasswordLength)
}

func shouldOverwritePassword(key, cur string, ok, force bool) bool {
	if key == "GARAGE_UI_AUTH_USER_PASS" && ok && strings.Contains(cur, garageUIAuthHolder) {
		return true
	}
	if force || !ok {
		return true
	}
	return slices.Contains(sharedPasswordPlaceholders, cur) || strings.Contains(cur, LegacyPlaceholderPassword)
}

func sharedPasswordTargets(repoPath, password string) [][3]string {
	var out [][3]string
	for svc, keys := range sharedPasswordKeys {
		if !fileExists(filepath.Join(repoPath, svc, stack.RootEnvFile)) {
			continue
		}
		for _, k := range keys {
			out = append(out, [3]string{svc, k, password})
		}
	}
	for _, tpl := range sharedPasswordTemplates {
		if fileExists(filepath.Join(repoPath, tpl[0], stack.RootEnvFile)) {
			out = append(out, [3]string{tpl[0], tpl[1], strings.ReplaceAll(tpl[2], "{password}", password)})
		}
	}
	if fileExists(filepath.Join(repoPath, "garage_ui", stack.RootEnvFile)) {
		if hashed, err := BcryptAuth("admin", password); err == nil {
			out = append(out, [3]string{"garage_ui", "GARAGE_UI_AUTH_USER_PASS", hashed})
		}
	}
	return out
}

// FillSecrets populates empty auto-generated secrets across root and service .env files.
func FillSecrets(repoPath string, services []string, say func(string)) (int, error) {
	filled := 0
	for _, svc := range ServiceEnvTargets(services) {
		envPath := filepath.Join(repoPath, svc, stack.RootEnvFile)
		if !fileExists(envPath) {
			continue
		}
		for _, spec := range autoSecrets[svc] {
			wrote, err := fillOneSecret(repoPath, svc, envPath, spec, say)
			if err != nil {
				return filled, err
			}
			if wrote {
				filled++
			}
		}
	}
	syncGarageUIToken(repoPath, say)
	return filled, nil
}

func fillOneSecret(repoPath, svc, envPath string, spec autoSecretSpec, say func(string)) (bool, error) {
	cur, _ := stack.ReadEnvValue(envPath, spec.key)
	isDash := spec.key == "TRAEFIK_DASHBOARD_AUTH"
	if isDash && cur != "" && cur != legacyDashboardHash && cur != strings.ReplaceAll(legacyDashboardHash, "$", "$$") {
		if escaped := EscapeDollars(cur); escaped != cur {
			if err := SetEnvValue(envPath, spec.key, escaped); err != nil {
				return false, err
			}
			emit(say, fmt.Sprintf("%s: escaped %s for Docker Compose", svc, spec.key))
		}
		return false, nil
	}
	if cur != "" && !isDash {
		if slices.Contains(knownSecretPlaceholders, cur) {
			emit(say, fmt.Sprintf("Warning: %s: %s still holds a public placeholder (left unchanged)", svc, spec.key))
		}
		return false, nil
	}
	val, err := generateSecretValue(repoPath, envPath, spec)
	if err != nil {
		return false, err
	}
	if err := SetEnvValue(envPath, spec.key, val); err != nil {
		return false, err
	}
	emit(say, fmt.Sprintf("%s: generated %s", svc, spec.key))
	return true, nil
}

func generateSecretValue(repoPath, envPath string, spec autoSecretSpec) (string, error) {
	switch spec.method {
	case methodHex:
		buf, err := randomBytes(spec.nbytes)
		if err != nil {
			return "", err
		}
		return hex.EncodeToString(buf), nil
	case methodBase64:
		buf, err := randomBytes(spec.nbytes)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(buf), nil
	case methodHtpasswd:
		shared, _ := stack.ReadEnvValue(filepath.Join(repoPath, stack.RootEnvFile), "DSS_SHARED_PASSWORD")
		if shared == "" || slices.Contains(invalidPasswordValues, shared) {
			return "", errors.New("set a secure DSS_SHARED_PASSWORD first")
		}
		return BcryptAuth("admin", shared)
	case methodJwtHS256:
		secret, _ := stack.ReadEnvValue(envPath, "JWT_SECRET")
		if secret == "" {
			return "", fmt.Errorf("%s: set JWT_SECRET before generating %s", envPath, spec.key)
		}
		role := "service_role"
		if spec.key == "ANON_KEY" {
			role = "anon"
		}
		return GenerateHS256JWT(secret, role, time.Now().Unix()), nil
	default:
		return "", fmt.Errorf("unsupported secret method %d", spec.method)
	}
}

func syncGarageUIToken(repoPath string, say func(string)) {
	uiPath := filepath.Join(repoPath, "garage_ui", stack.RootEnvFile)
	garagePath := filepath.Join(repoPath, "garage", stack.RootEnvFile)
	if !fileExists(uiPath) || !fileExists(garagePath) {
		return
	}
	token, _ := stack.ReadEnvValue(garagePath, "GARAGE_ADMIN_TOKEN")
	curUI, _ := stack.ReadEnvValue(uiPath, "GARAGE_ADMIN_TOKEN")
	if token == "" || curUI != "" {
		return
	}
	if err := SetEnvValue(uiPath, "GARAGE_ADMIN_TOKEN", token); err == nil {
		emit(say, "garage_ui: copied GARAGE_ADMIN_TOKEN from garage/.env")
	}
}
