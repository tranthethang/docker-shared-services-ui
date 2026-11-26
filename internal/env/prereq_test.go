package env

import (
	"path/filepath"
	"testing"
)

func TestPrerequisiteProblems(t *testing.T) {
	t.Run("missing root env", func(t *testing.T) {
		dir := t.TempDir()
		probs := PrerequisiteProblems(dir, []string{"redis"})
		if len(probs) != 1 || probs[0].Code != "root_env_missing" {
			t.Fatalf("got %+v, want root_env_missing", probs)
		}
	})

	t.Run("missing service env", func(t *testing.T) {
		dir := t.TempDir()
		writeEnv(t, filepath.Join(dir, ".env"), "DOMAIN=sandbox.localhost\n")
		probs := PrerequisiteProblems(dir, []string{"redis"})
		if len(probs) != 1 || probs[0].Code != "service_env_missing" {
			t.Fatalf("got %+v, want service_env_missing", probs)
		}
	})

	t.Run("legacy Password102! rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeEnv(t, filepath.Join(dir, ".env"), "DOMAIN=sandbox.localhost\n")
		writeEnv(t, filepath.Join(dir, "postgres", ".env"), "POSTGRES16_PASSWORD=Password102!\n")
		probs := PrerequisiteProblems(dir, []string{"postgres"})
		if len(probs) != 1 || probs[0].Code != "invalid_env_var" || probs[0].Key != "POSTGRES16_PASSWORD" {
			t.Fatalf("got %+v, want invalid_env_var for POSTGRES16_PASSWORD", probs)
		}
	})

	t.Run("zitadel weak vs strong password", func(t *testing.T) {
		weakDir := t.TempDir()
		writeEnv(t, filepath.Join(weakDir, ".env"), "DOMAIN=sandbox.localhost\n")
		writeEnv(t, filepath.Join(weakDir, "zitadel", ".env"), "ZITADEL_MASTERKEY=0123456789abcdef0123456789abcdef\nZITADEL_ADMIN_PASSWORD=short\n")
		probs := PrerequisiteProblems(weakDir, []string{"zitadel"})
		if len(probs) != 1 || probs[0].Code != "weak_password" {
			t.Fatalf("got %+v, want weak_password", probs)
		}

		strongDir := t.TempDir()
		writeEnv(t, filepath.Join(strongDir, ".env"), "DOMAIN=sandbox.localhost\n")
		writeEnv(t, filepath.Join(strongDir, "zitadel", ".env"), "ZITADEL_MASTERKEY=0123456789abcdef0123456789abcdef\nZITADEL_ADMIN_PASSWORD=Str0ng!ZitadelPW\n")
		probs = PrerequisiteProblems(strongDir, []string{"zitadel"})
		if len(probs) != 0 {
			t.Fatalf("got %+v, want no problems", probs)
		}
	})
}

func TestLabelOnlyReason(t *testing.T) {
	dir := t.TempDir()
	if code, _ := LabelOnlyReason(dir, "redis"); code != "" {
		t.Fatalf("expected empty reason for redis, got %q", code)
	}
	if code, _ := LabelOnlyReason(dir, "oauth2-proxy"); code != "oauth2_env_missing" {
		t.Fatalf("got %q, want oauth2_env_missing", code)
	}

	writeEnv(t, filepath.Join(dir, "oauth2-proxy", ".env"), "OAUTH2_PROXY_CLIENT_ID=\nOAUTH2_PROXY_CLIENT_SECRET=sec\n")
	if code, _ := LabelOnlyReason(dir, "oauth2-proxy"); code != "oauth2_unconfigured" {
		t.Fatalf("got %q, want oauth2_unconfigured", code)
	}

	writeEnv(t, filepath.Join(dir, "oauth2-proxy", ".env"), "OAUTH2_PROXY_CLIENT_ID=cid\nOAUTH2_PROXY_CLIENT_SECRET=sec\n")
	if code, reason := LabelOnlyReason(dir, "oauth2-proxy"); code != "" || reason != "" {
		t.Fatalf("got (%q, %q), want empty", code, reason)
	}
}
