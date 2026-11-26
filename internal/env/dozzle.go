package env

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

const (
	bcryptCost         = 10
	jwtLifetimeSeconds = int64(10 * 365 * 24 * 3600)
	passwordSpecials   = "!@#$%^&*()-_=+[]{}:,.?"
)

// DozzleGenerator generates the users.yml content for a given admin password.
type DozzleGenerator func(ctx context.Context, password string) ([]byte, error)

// EnsureDozzleUsers writes dozzle/data/users.yml using the shared password in root .env when absent.
func EnsureDozzleUsers(ctx context.Context, repoPath string, say func(string)) error {
	return EnsureDozzleUsersWith(ctx, repoPath, say, defaultDozzleGenerator)
}

// EnsureDozzleUsersWith writes dozzle/data/users.yml using the provided generator when absent.
func EnsureDozzleUsersWith(ctx context.Context, repoPath string, say func(string), gen DozzleGenerator) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	usersFile := filepath.Join(repoPath, "dozzle", "data", "users.yml")
	if err := os.MkdirAll(filepath.Dir(usersFile), 0o750); err != nil {
		return fmt.Errorf("create dozzle/data: %w", err)
	}
	if info, err := os.Stat(usersFile); err == nil {
		if info.IsDir() {
			emit(say, "dozzle/data/users.yml is a directory; removing and regenerating...")
			if err := os.RemoveAll(usersFile); err != nil {
				return fmt.Errorf("remove directory %s: %w", usersFile, err)
			}
		} else {
			emit(say, "dozzle/data/users.yml already exists")
			return nil
		}
	}

	rootEnv := filepath.Join(repoPath, stack.RootEnvFile)
	password, _ := stack.ReadEnvValue(rootEnv, "DSS_SHARED_PASSWORD")
	if password == "" || slices.Contains(invalidPasswordValues, password) {
		return fmt.Errorf("DSS_SHARED_PASSWORD is missing or still %s; run setup before generating Dozzle users", LegacyPlaceholderPassword)
	}

	emit(say, "Generating dozzle/data/users.yml from shared password...")
	data, err := gen(ctx, password)
	if err != nil {
		return err
	}
	if err := os.WriteFile(usersFile, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", usersFile, err)
	}
	emit(say, "Generated dozzle/data/users.yml")
	return nil
}

func defaultDozzleGenerator(ctx context.Context, password string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-i", "amir20/dozzle", "generate", "admin", "--password", password, "--email", "admin@example.com", "--name", "Admin") //nolint:gosec // fixed image and args
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if ok := asExitError(err, &exitErr); ok {
			return nil, fmt.Errorf("docker run dozzle failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("docker run dozzle failed: %w", err)
	}
	return out, nil
}

func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}

// GeneratePassword creates a strong random password meeting Zitadel rules without a leading special char.
func GeneratePassword(length int) (string, error) {
	length = max(length, PasswordMinLength)
	pools := []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789", passwordSpecials}
	all := strings.Join(pools, "")
	chars := make([]byte, 0, length)
	for _, pool := range pools {
		ch, err := randChar(pool)
		if err != nil {
			return "", err
		}
		chars = append(chars, ch)
	}
	for len(chars) < length {
		ch, err := randChar(all)
		if err != nil {
			return "", err
		}
		chars = append(chars, ch)
	}
	for i := len(chars) - 1; i > 0; i-- {
		j, err := randIndex(i + 1)
		if err != nil {
			return "", err
		}
		chars[i], chars[j] = chars[j], chars[i]
	}
	if strings.ContainsRune(passwordSpecials, rune(chars[0])) {
		for i := 1; i < len(chars); i++ {
			if !strings.ContainsRune(passwordSpecials, rune(chars[i])) {
				chars[0], chars[i] = chars[i], chars[0]
				break
			}
		}
	}
	return string(chars), nil
}

func randIndex(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

func randChar(pool string) (byte, error) {
	idx, err := randIndex(len(pool))
	if err != nil {
		return 0, err
	}
	return pool[idx], nil
}

func randomBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := rand.Read(buf)
	return buf, err
}

// BcryptAuth formats user:hash with $2y$ prefix and doubled $ for Docker Compose .env files.
func BcryptAuth(username, password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	s := strings.Replace(string(hashed), "$2a$", "$2y$", 1)
	s = strings.Replace(s, "$2b$", "$2y$", 1)
	return strings.ReplaceAll(username+":"+s, "$", "$$"), nil
}

// GenerateHS256JWT signs a Supabase API key JWT for role.
func GenerateHS256JWT(secret, role string, now int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"role":%q,"iss":"supabase","iat":%d,"exp":%d}`, role, now, now+jwtLifetimeSeconds)))
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// EscapeDollars doubles lone $ characters for Docker Compose .env files.
func EscapeDollars(val string) string {
	var sb strings.Builder
	for i := 0; i < len(val); i++ {
		sb.WriteByte(val[i])
		if val[i] == '$' && (i == 0 || val[i-1] != '$') && (i+1 >= len(val) || val[i+1] != '$') {
			sb.WriteByte('$')
		}
	}
	return sb.String()
}
