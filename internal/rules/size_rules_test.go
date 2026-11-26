package rules_test

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const maxGoFileLines = 300

func TestGoFileLineLimits(t *testing.T) {
	t.Parallel()

	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" || name == "dist" || name == ".agent-relay" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		lines, countErr := countLines(path)
		if countErr != nil {
			t.Errorf("count lines in %s: %v", path, countErr)
			return nil
		}
		if lines > maxGoFileLines {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			t.Errorf("%s has %d lines (max %d)", rel, lines, maxGoFileLines)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fs.ErrNotExist
		}
		dir = parent
	}
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return lines, nil
}
