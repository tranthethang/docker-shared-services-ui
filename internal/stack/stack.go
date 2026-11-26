// Package stack parses stack identity, shared network names/subnets, and package manifests.
package stack

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// RootEnvFile is the root environment filename inside a dss repository.
	RootEnvFile = ".env"
	// ManifestFile is the optional exported-package manifest filename.
	ManifestFile = "dss-package.json"
	// MaxSubnetBase is the maximum allowed third-to-last /16 base octet (253 + 2 = 255).
	MaxSubnetBase = 253
)

var networkBases = []string{"traefik_proxy", "backend_net", "data_net"}

// NetworkPair describes a shared Docker network name and its CIDR subnet.
type NetworkPair struct {
	Name   string `json:"name"`
	Subnet string `json:"subnet"`
	Pair   string `json:"pair"`
}

// Identity describes the stack name, subnet base, and shared networks for a repository.
type Identity struct {
	Name       string        `json:"name"`
	SubnetBase int           `json:"subnetBase"`
	Prefix     string        `json:"prefix"`
	Networks   []NetworkPair `json:"networks"`
}

// ProjectName returns the Docker Compose project name for a service.
func (id Identity) ProjectName(service string) string {
	return id.Prefix + service
}

// ReadEnvValue reads the unquoted value for key from an env file.
func ReadEnvValue(envPath, key string) (string, bool) {
	data, err := os.ReadFile(envPath) //nolint:gosec // envPath is derived from validated repo root
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		name, rawVal, ok := strings.Cut(line, "=")
		if !ok || name != key {
			continue
		}
		val := strings.TrimSpace(rawVal)
		val = strings.Trim(val, `"'`)
		return val, true
	}
	return "", false
}

// ValidName reports whether name is empty or a valid stack name.
func ValidName(name string) bool {
	if name == "" {
		return true
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if i == 0 {
			if !isAlphaNum {
				return false
			}
			continue
		}
		if !isAlphaNum && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

// ParseIdentity reads SANDBOX_NAME and SANDBOX_SUBNET_BASE from <repoPath>/.env.
func ParseIdentity(repoPath string) (Identity, error) {
	envPath := filepath.Join(repoPath, RootEnvFile)
	name, _ := ReadEnvValue(envPath, "SANDBOX_NAME")
	if !ValidName(name) {
		return Identity{}, fmt.Errorf("SANDBOX_NAME=%q is invalid; use lowercase letters, digits, '_' or '-'", name)
	}

	rawBase, _ := ReadEnvValue(envPath, "SANDBOX_SUBNET_BASE")
	base, err := parseSubnetBase(rawBase)
	if err != nil {
		return Identity{}, err
	}

	prefix := ""
	if name != "" {
		prefix = name + "-"
	}

	return Identity{
		Name:       name,
		SubnetBase: base,
		Prefix:     prefix,
		Networks:   buildNetworks(name, base),
	}, nil
}

func parseSubnetBase(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	base, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("SANDBOX_SUBNET_BASE=%q is invalid; use an integer", raw)
	}
	if base < 0 || base > MaxSubnetBase {
		return 0, fmt.Errorf("SANDBOX_SUBNET_BASE=%d is invalid; use 0 to %d", base, MaxSubnetBase)
	}
	return base, nil
}

func buildNetworks(stackName string, base int) []NetworkPair {
	pairs := make([]NetworkPair, 0, len(networkBases))
	for i, baseName := range networkBases {
		netName := baseName
		if stackName != "" {
			netName = stackName + "_" + baseName
		}
		subnet := fmt.Sprintf("10.%d.0.0/16", base+i)
		pairs = append(pairs, NetworkPair{
			Name:   netName,
			Subnet: subnet,
			Pair:   netName + ":" + subnet,
		})
	}
	return pairs
}

type manifestPayload struct {
	Services []string `json:"services"`
}

// SelectServices filters known services to those listed in manifestJSON, preserving known order.
func SelectServices(manifestJSON []byte, known []string) ([]string, error) {
	var m manifestPayload
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", ManifestFile, err)
	}
	allowed := make(map[string]struct{}, len(m.Services))
	for _, s := range m.Services {
		allowed[s] = struct{}{}
	}
	selected := make([]string, 0, len(allowed))
	for _, s := range known {
		if _, ok := allowed[s]; ok {
			selected = append(selected, s)
		}
	}
	return selected, nil
}

// LoadActiveServices returns the subset of known services from dss-package.json if present,
// or a copy of all known services when the manifest file does not exist.
func LoadActiveServices(repoPath string, known []string) ([]string, error) {
	manifestPath := filepath.Join(repoPath, ManifestFile)
	data, err := os.ReadFile(manifestPath) //nolint:gosec // manifestPath is derived from validated repo root
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			out := make([]string, len(known))
			copy(out, known)
			return out, nil
		}
		return nil, fmt.Errorf("read %s: %w", ManifestFile, err)
	}
	return SelectServices(data, known)
}
