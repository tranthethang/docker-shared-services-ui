package env

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

func TestCheckCreateAndMergeEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte("DOMAIN=sandbox.localhost\nDSS_SHARED_PASSWORD=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	redisDir := filepath.Join(dir, "redis")
	if err := os.MkdirAll(redisDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(redisDir, ".env.example"), []byte("REDIS_PORT=6379\nNEW_KEY=abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(redisDir, ".env"), []byte("REDIS_PORT=9999\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	statuses := CheckEnv(dir, []string{"redis"})
	if len(statuses) != 2 || !statuses[0].Missing || statuses[1].Missing {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}

	created, err := CreateMissingEnv(dir, []string{"redis"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 || created[0] != "." {
		t.Fatalf("expected only root created, got %v", created)
	}

	added, err := MergeMissingEnv(dir, []string{"redis"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Fatalf("expected 1 key merged, got %d", added)
	}
	val, ok := stack.ReadEnvValue(filepath.Join(redisDir, ".env"), "REDIS_PORT")
	if !ok || val != "9999" {
		t.Fatalf("existing key clobbered: got %q", val)
	}
	newVal, ok := stack.ReadEnvValue(filepath.Join(redisDir, ".env"), "NEW_KEY")
	if !ok || newVal != "abc" {
		t.Fatalf("missing key not merged: got %q", newVal)
	}
}

func TestFillSharedPasswordAndForce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DSS_SHARED_PASSWORD=\nPOSTGRES16_PASSWORD=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pgDir := filepath.Join(dir, "postgres")
	if err := os.MkdirAll(pgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pgDir, ".env"), []byte("POSTGRES16_PASSWORD=CustomKeep123!@#\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := FillSharedPassword(dir, LegacyPlaceholderPassword, false, nil); err == nil {
		t.Fatal("expected legacy Password102! to be rejected")
	}

	pw, err := FillSharedPassword(dir, "", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reason := ValidatePasswordStrength(pw); reason != "" {
		t.Fatalf("generated password weak: %s", reason)
	}
	kept, _ := stack.ReadEnvValue(filepath.Join(pgDir, ".env"), "POSTGRES16_PASSWORD")
	if kept != "CustomKeep123!@#" {
		t.Fatalf("custom password overwritten without force: got %q", kept)
	}

	if _, err := FillSharedPassword(dir, "NewStrongPass!12345", true, nil); err != nil {
		t.Fatal(err)
	}
	forced, _ := stack.ReadEnvValue(filepath.Join(pgDir, ".env"), "POSTGRES16_PASSWORD")
	if forced != "NewStrongPass!12345" {
		t.Fatalf("expected forced password update, got %q", forced)
	}
}

func TestFillSecretsAndDozzleUsers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DSS_SHARED_PASSWORD=StrongPass!123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	supaDir := filepath.Join(dir, "supabase")
	if err := os.MkdirAll(supaDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(supaDir, ".env"), []byte("JWT_SECRET=\nANON_KEY=\nSERVICE_ROLE_KEY=\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	filled, err := FillSecrets(dir, []string{"supabase"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filled != 9 {
		t.Fatalf("expected 9 secrets filled, got %d", filled)
	}
	anon, _ := stack.ReadEnvValue(filepath.Join(supaDir, ".env"), "ANON_KEY")
	if strings.Count(anon, ".") != 2 {
		t.Fatalf("expected JWT format for ANON_KEY, got %q", anon)
	}

	if err := EnsureDozzleUsersWith(context.Background(), dir, nil, func(_ context.Context, pw string) ([]byte, error) {
		return []byte("users:\n  admin:\n    password: " + pw + "\n"), nil
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "dozzle", "data", "users.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "StrongPass!123456") {
		t.Fatalf("unexpected users.yml content: %s", string(data))
	}
}
