package stack

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseIdentity(t *testing.T) {
	t.Run("unnamed stack keeps historic names", func(t *testing.T) {
		dir := t.TempDir()
		id, err := ParseIdentity(dir)
		if err != nil {
			t.Fatalf("ParseIdentity() error = %v", err)
		}
		if id.Name != "" || id.Prefix != "" || id.SubnetBase != 0 {
			t.Fatalf("unexpected identity: %+v", id)
		}
		wantPairs := []string{
			"traefik_proxy:10.0.0.0/16",
			"backend_net:10.1.0.0/16",
			"data_net:10.2.0.0/16",
		}
		for i, want := range wantPairs {
			if id.Networks[i].Pair != want {
				t.Fatalf("network[%d] = %q, want %q", i, id.Networks[i].Pair, want)
			}
		}
		if got := id.ProjectName("redis"); got != "redis" {
			t.Fatalf("ProjectName(redis) = %q, want redis", got)
		}
	})

	t.Run("named stack dev with base 10", func(t *testing.T) {
		dir := t.TempDir()
		envContent := "SANDBOX_NAME='dev'\nSANDBOX_SUBNET_BASE=\"10\"\n"
		if err := os.WriteFile(filepath.Join(dir, RootEnvFile), []byte(envContent), 0o600); err != nil {
			t.Fatal(err)
		}
		id, err := ParseIdentity(dir)
		if err != nil {
			t.Fatalf("ParseIdentity() error = %v", err)
		}
		if id.Prefix != "dev-" {
			t.Fatalf("Prefix = %q, want dev-", id.Prefix)
		}
		if got := id.Networks[2].Pair; got != "dev_data_net:10.12.0.0/16" {
			t.Fatalf("data_net = %q, want dev_data_net:10.12.0.0/16", got)
		}
		if got := id.ProjectName("redis"); got != "dev-redis" {
			t.Fatalf("ProjectName(redis) = %q, want dev-redis", got)
		}
	})

	t.Run("rejects invalid names and subnet bases", func(t *testing.T) {
		invalidEnvs := []string{
			"SANDBOX_NAME=BadName\n",
			"SANDBOX_NAME=-lead\n",
			"SANDBOX_SUBNET_BASE=abc\n",
			"SANDBOX_SUBNET_BASE=-1\n",
			"SANDBOX_SUBNET_BASE=254\n",
		}
		for _, content := range invalidEnvs {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, RootEnvFile), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ParseIdentity(dir); err == nil {
				t.Fatalf("expected error for %q", content)
			}
		}
	})

	t.Run("accepts boundary valid name and subnet base", func(t *testing.T) {
		dir := t.TempDir()
		envContent := "SANDBOX_NAME=ok_name-1\nSANDBOX_SUBNET_BASE=253\n"
		if err := os.WriteFile(filepath.Join(dir, RootEnvFile), []byte(envContent), 0o600); err != nil {
			t.Fatal(err)
		}
		id, err := ParseIdentity(dir)
		if err != nil {
			t.Fatalf("ParseIdentity() error = %v", err)
		}
		if id.SubnetBase != 253 || id.Networks[2].Subnet != "10.255.0.0/16" {
			t.Fatalf("unexpected identity: %+v", id)
		}
	})
}

func TestLoadActiveServices(t *testing.T) {
	known := []string{"adminer", "postgres", "redis", "traefik"}

	t.Run("returns all known services when manifest absent", func(t *testing.T) {
		dir := t.TempDir()
		got, err := LoadActiveServices(dir, known)
		if err != nil {
			t.Fatalf("LoadActiveServices() error = %v", err)
		}
		if !reflect.DeepEqual(got, known) {
			t.Fatalf("got %v, want %v", got, known)
		}
	})

	t.Run("filters and preserves known order when manifest present", func(t *testing.T) {
		dir := t.TempDir()
		manifest := `{"services": ["redis", "traefik", "unknown", "adminer"]}`
		if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := LoadActiveServices(dir, known)
		if err != nil {
			t.Fatalf("LoadActiveServices() error = %v", err)
		}
		want := []string{"adminer", "redis", "traefik"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("returns error on malformed manifest", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte("{bad"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadActiveServices(dir, known); err == nil {
			t.Fatal("expected error for malformed manifest")
		}
	})
}
