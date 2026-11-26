package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldMask(t *testing.T) {
	cases := []struct {
		key   string
		value string
		want  bool
	}{
		{"POSTGRES16_PASSWORD", "abc", true},
		{"POSTGRES16_PASSWORD", "ab", false},
		{"DEBUG", "true", false},
		{"DEBUG", "FALSE", false},
		{"PORT", "5432", false},
		{"HOST", "postgres", false},
		{"DATABASE_URL_X", "postgres://host/db", true},
		{"EMPTY_SECRET", "", false},
	}

	for _, tc := range cases {
		if got := ShouldMask(tc.key, tc.value); got != tc.want {
			t.Errorf("ShouldMask(%q, %q) = %v, want %v", tc.key, tc.value, got, tc.want)
		}
	}
}

func TestMaskKnownValuesLongestFirst(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, filepath.Join(dir, ".env"), "SHORT_PASS=hunter2\nLONG_PASS=hunter2-long\nPORT=5432\nENABLED=true\n")
	writeEnv(t, filepath.Join(dir, "redis", ".env"), "REDIS_PASSWORD=redis-secret\n")

	values := ValuesToMask(dir, []string{"redis"})
	got := MaskKnownValues("pw=hunter2-long x=hunter2 redis=redis-secret port=5432 flag=true", values)
	want := "pw=*** x=*** redis=*** port=5432 flag=true"
	if got != want {
		t.Fatalf("MaskKnownValues() = %q, want %q", got, want)
	}
}

func writeEnv(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
