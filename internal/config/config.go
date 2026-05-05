package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dmundt/rist/internal/security"
	"gopkg.in/yaml.v3"
)

const (
	currentVersion = 1
)

var repoIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type Config struct {
	Version int    `yaml:"version"`
	Repos   []Repo `yaml:"repos"`
}

type Repo struct {
	ID             string   `yaml:"id"`
	Path           string   `yaml:"path"`
	PasswordTarget string   `yaml:"passwordTarget"`
	Include        []string `yaml:"include"`
	Exclude        []string `yaml:"exclude"`
	Options        Options  `yaml:"options"`
}

type Options struct {
	OneFileSystem bool `yaml:"oneFileSystem"`
	Verbose       bool `yaml:"verbose"`
}

func DefaultConfig() Config {
	return Config{
		Version: currentVersion,
		Repos:   []Repo{},
	}
}

func CurrentVersion() int {
	return currentVersion
}

func DefaultPasswordTarget(repoID string) string {
	return fmt.Sprintf("rist-repo-%s", repoID)
}

func ConfigPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if strings.TrimSpace(appData) == "" {
		return "", errors.New("APPDATA is not set")
	}
	return filepath.Join(appData, "Rist", "config.yaml"), nil
}

func ConfigDir() (string, error) {
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

func EnsureConfigDir() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := security.RestrictDirectoryToCurrentUser(dir); err != nil {
		return fmt.Errorf("harden config dir ACL: %w", err)
	}
	return nil
}

func Load() (Config, error) {
	cfg, err := LoadRaw()
	if err != nil {
		return Config{}, err
	}

	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return normalize(cfg), nil
}

func LoadRaw() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config yaml: %w", err)
	}
	return cfg, nil
}

func MigrateToCurrent(cfg Config) (Config, bool, error) {
	switch cfg.Version {
	case currentVersion:
		return normalize(cfg), false, nil
	default:
		return Config{}, false, fmt.Errorf("no migration path from config version %d to %d", cfg.Version, currentVersion)
	}
}

func LoadOrDefault() (Config, error) {
	cfg, err := Load()
	if err == nil {
		return cfg, nil
	}
	if os.IsNotExist(rootCause(err)) {
		return DefaultConfig(), nil
	}
	return Config{}, err
}

func Save(cfg Config) error {
	cfg = normalize(cfg)
	if err := Validate(cfg); err != nil {
		return err
	}
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config yaml: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, b, 0o600); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace config atomically: %w", err)
	}
	return nil
}

func MustMarshalYAML(cfg Config) string {
	b, err := yaml.Marshal(normalize(cfg))
	if err != nil {
		panic(err)
	}
	return string(b)
}

func Validate(cfg Config) error {
	if cfg.Version != currentVersion {
		return fmt.Errorf("unsupported config version: %d", cfg.Version)
	}

	seenIDs := map[string]struct{}{}
	for i, repo := range cfg.Repos {
		if err := validateRepo(repo, seenIDs); err != nil {
			return fmt.Errorf("repos[%d]: %w", i, err)
		}
	}
	return nil
}

func RepoExists(cfg Config, id string) bool {
	for _, repo := range cfg.Repos {
		if repo.ID == id {
			return true
		}
	}
	return false
}

func FindRepoByID(cfg Config, id string) (Repo, bool) {
	for _, repo := range cfg.Repos {
		if repo.ID == id {
			return repo, true
		}
	}
	return Repo{}, false
}

func normalize(cfg Config) Config {
	if cfg.Repos == nil {
		cfg.Repos = []Repo{}
	}

	sort.Slice(cfg.Repos, func(i, j int) bool {
		return cfg.Repos[i].ID < cfg.Repos[j].ID
	})

	for i := range cfg.Repos {
		repo := &cfg.Repos[i]
		if repo.Include == nil {
			repo.Include = []string{}
		}
		if repo.Exclude == nil {
			repo.Exclude = []string{}
		}
		if strings.TrimSpace(repo.PasswordTarget) == "" && strings.TrimSpace(repo.ID) != "" {
			repo.PasswordTarget = DefaultPasswordTarget(repo.ID)
		}
	}

	return cfg
}

func validateRepo(repo Repo, seenIDs map[string]struct{}) error {
	id := strings.TrimSpace(repo.ID)
	if id == "" {
		return errors.New("id is required")
	}
	if !repoIDPattern.MatchString(id) {
		return errors.New("id must match [a-zA-Z0-9._-]+")
	}
	if _, ok := seenIDs[id]; ok {
		return fmt.Errorf("duplicate id: %s", id)
	}
	seenIDs[id] = struct{}{}

	path := strings.TrimSpace(repo.Path)
	if path == "" {
		return errors.New("path is required")
	}
	if !isAllowedFolderBackend(path) {
		return errors.New("path must be a local absolute path or UNC path")
	}

	if strings.TrimSpace(repo.PasswordTarget) == "" {
		return errors.New("passwordTarget is required")
	}

	if hasEmptyPath(repo.Include) {
		return errors.New("include contains empty path")
	}
	if hasEmptyPath(repo.Exclude) {
		return errors.New("exclude contains empty path")
	}

	return nil
}

func isAllowedFolderBackend(path string) bool {
	if strings.HasPrefix(path, `\\`) {
		return true
	}
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	return false
}

func hasEmptyPath(paths []string) bool {
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			return true
		}
	}
	return false
}

func rootCause(err error) error {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}
		err = next
	}
}
