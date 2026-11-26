// Package catalog loads the service catalog, substitutes domains, and filters active services.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

const (
	// DefaultDomain is used when DOMAIN is missing or empty in .env.
	DefaultDomain = "sandbox.localhost"
	// CatalogRelPath is the relative path to service_catalog.json inside the repo.
	CatalogRelPath    = "cli/service_catalog.json"
	domainPlaceholder = "{domain}"
)

var orderedCategories = []string{
	"Database",
	"Cache & Queue",
	"Storage",
	"Auth & Gateway",
	"CI/CD",
	"Workflow & Automation",
	"Observability",
	"Admin UI",
	"AI & Search",
	"Dev Tools",
}

// Categories returns the 10 ordered service categories.
func Categories() []string {
	out := make([]string, len(orderedCategories))
	copy(out, orderedCategories)
	return out
}

// Entry describes one service from service_catalog.json with {domain} substituted.
type Entry struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Containers  []string `json:"containers"`
	WebUI       []string `json:"web_ui"`
	Info        []string `json:"info"`
}

// Loaded holds the parsed, domain-substituted, and manifest-filtered service catalog.
type Loaded struct {
	Domain      string           `json:"domain"`
	Categories  []string         `json:"categories"`
	ActiveNames []string         `json:"activeNames"`
	Entries     map[string]Entry `json:"entries"`
}

// LoadDomain reads DOMAIN from <repoPath>/.env, falling back to DefaultDomain.
func LoadDomain(repoPath string) string {
	val, ok := stack.ReadEnvValue(filepath.Join(repoPath, stack.RootEnvFile), "DOMAIN")
	if !ok || val == "" {
		return DefaultDomain
	}
	return val
}

// Parse unmarshals service_catalog.json and replaces {domain} in web_ui and info.
func Parse(rawJSON []byte, domain string) (map[string]Entry, error) {
	if domain == "" {
		domain = DefaultDomain
	}
	var raw map[string]Entry
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", CatalogRelPath, err)
	}
	out := make(map[string]Entry, len(raw))
	for name, entry := range raw {
		entry.Name = name
		entry.WebUI = replaceDomain(entry.WebUI, domain)
		entry.Info = replaceDomain(entry.Info, domain)
		out[name] = entry
	}
	return out, nil
}

func replaceDomain(items []string, domain string) []string {
	if len(items) == 0 {
		return []string{}
	}
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = strings.ReplaceAll(item, domainPlaceholder, domain)
	}
	return out
}

// Load reads cli/service_catalog.json from repoPath, substitutes DOMAIN, and applies dss-package.json.
func Load(repoPath string) (*Loaded, error) {
	catalogPath := filepath.Join(repoPath, CatalogRelPath)
	raw, err := os.ReadFile(catalogPath) //nolint:gosec // catalogPath is derived from validated repo root
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", CatalogRelPath, err)
	}

	domain := LoadDomain(repoPath)
	allEntries, err := Parse(raw, domain)
	if err != nil {
		return nil, err
	}

	known := make([]string, 0, len(allEntries))
	for name := range allEntries {
		known = append(known, name)
	}
	sort.Strings(known)

	active, err := stack.LoadActiveServices(repoPath, known)
	if err != nil {
		return nil, err
	}

	filtered := make(map[string]Entry, len(active))
	for _, name := range active {
		if entry, ok := allEntries[name]; ok {
			filtered[name] = entry
		}
	}

	return &Loaded{
		Domain:      domain,
		Categories:  Categories(),
		ActiveNames: active,
		Entries:     filtered,
	}, nil
}
