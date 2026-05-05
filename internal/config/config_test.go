package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsNonFolderBackend(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Repos = []Repo{{
		ID:             "main",
		Path:           "relative/path",
		PasswordTarget: "rist-repo-main",
		Include:        []string{},
		Exclude:        []string{},
	}}

	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for non-folder backend")
	}
}

func TestNormalizeAssignsDefaultPasswordTarget(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Repos = []Repo{{
		ID:      "main",
		Path:    `C:\\Backups\\restic`,
		Include: []string{},
		Exclude: []string{},
	}}

	n := normalize(cfg)
	if n.Repos[0].PasswordTarget != "rist-repo-main" {
		t.Fatalf("unexpected password target: %s", n.Repos[0].PasswordTarget)
	}
}

func TestMigrateToCurrentNoChange(t *testing.T) {
	cfg := DefaultConfig()
	migrated, changed, err := MigrateToCurrent(cfg)
	if err != nil {
		t.Fatalf("unexpected migration error: %v", err)
	}
	if changed {
		t.Fatal("expected no migration change")
	}
	if migrated.Version != CurrentVersion() {
		t.Fatalf("expected current version %d, got %d", CurrentVersion(), migrated.Version)
	}
}

func TestMigrateToCurrentUnsupportedVersion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Version = 2
	if _, _, err := MigrateToCurrent(cfg); err == nil {
		t.Fatal("expected migration error for unsupported version")
	}
}

func TestConfigPathAndDir(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)

	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath returned error: %v", err)
	}
	if want := filepath.Join(appData, "Rist", "config.yaml"); path != want {
		t.Fatalf("ConfigPath = %q, want %q", path, want)
	}

	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir returned error: %v", err)
	}
	if want := filepath.Join(appData, "Rist"); dir != want {
		t.Fatalf("ConfigDir = %q, want %q", dir, want)
	}
}

func TestConfigPathRequiresAppData(t *testing.T) {
	t.Setenv("APPDATA", "")
	if _, err := ConfigPath(); err == nil {
		t.Fatal("expected error when APPDATA is unset")
	}
}

func TestSaveLoadAndHelpers(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)
	prependFakeIcaclsToPath(t)

	cfg := DefaultConfig()
	cfg.Repos = []Repo{{
		ID:             "main",
		Path:           `C:\Backups\restic`,
		PasswordTarget: "rist-repo-main",
		Include:        []string{`C:\Users\me\Documents`},
		Exclude:        []string{},
	}}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !RepoExists(loaded, "main") {
		t.Fatal("expected repo to exist after Save/Load")
	}
	repo, ok := FindRepoByID(loaded, "main")
	if !ok || repo.ID != "main" {
		t.Fatalf("FindRepoByID returned %#v, %t", repo, ok)
	}
	if yaml := MustMarshalYAML(loaded); !strings.Contains(yaml, "passwordTarget: rist-repo-main") {
		t.Fatalf("MustMarshalYAML missing expected password target: %q", yaml)
	}

	raw, err := LoadRaw()
	if err != nil {
		t.Fatalf("LoadRaw returned error: %v", err)
	}
	if raw.Version != CurrentVersion() {
		t.Fatalf("LoadRaw version = %d, want %d", raw.Version, CurrentVersion())
	}
}

func TestLoadOrDefaultWhenMissing(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)

	cfg, err := LoadOrDefault()
	if err != nil {
		t.Fatalf("LoadOrDefault returned error: %v", err)
	}
	if cfg.Version != CurrentVersion() || len(cfg.Repos) != 0 {
		t.Fatalf("unexpected default config: %#v", cfg)
	}
}

func TestLoadRawAndLoadOrDefaultErrors(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)
	if err := os.MkdirAll(filepath.Join(appData, "Rist"), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	path := filepath.Join(appData, "Rist", "config.yaml")
	if err := os.WriteFile(path, []byte("not: [yaml"), 0o600); err != nil {
		t.Fatalf("write bad yaml: %v", err)
	}
	if _, err := LoadRaw(); err == nil || !strings.Contains(err.Error(), "parse config yaml") {
		t.Fatalf("expected LoadRaw parse error, got %v", err)
	}
	if _, err := LoadOrDefault(); err == nil || !strings.Contains(err.Error(), "parse config yaml") {
		t.Fatalf("expected LoadOrDefault parse error passthrough, got %v", err)
	}
}

func TestEnsureConfigDirErrors(t *testing.T) {
	t.Setenv("APPDATA", "")
	if err := EnsureConfigDir(); err == nil || !strings.Contains(err.Error(), "APPDATA is not set") {
		t.Fatalf("expected APPDATA error, got %v", err)
	}

	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)
	dir := t.TempDir()
	script := filepath.Join(dir, "icacls.cmd")
	if err := os.WriteFile(script, []byte("@echo off\r\nexit /b 1\r\n"), 0o700); err != nil {
		t.Fatalf("write fake icacls: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := EnsureConfigDir(); err == nil || !strings.Contains(err.Error(), "harden config dir ACL") {
		t.Fatalf("expected ACL hardening error, got %v", err)
	}
}

func TestHasEmptyPathAndRootCause(t *testing.T) {
	if !hasEmptyPath([]string{"ok", " "}) {
		t.Fatal("expected hasEmptyPath to detect blank entry")
	}
	if hasEmptyPath([]string{"ok", "also-ok"}) {
		t.Fatal("did not expect hasEmptyPath to report false positive")
	}

	inner := os.ErrNotExist
	err := errors.New("outer: " + inner.Error())
	wrapped := &wrapErr{err: err, unwrap: inner}
	if got := rootCause(wrapped); !errors.Is(got, os.ErrNotExist) {
		t.Fatalf("rootCause = %v, want not-exist", got)
	}
}

func TestValidateRepoErrorCases(t *testing.T) {
	tests := []struct {
		name string
		repo Repo
		want string
	}{
		{name: "missing id", repo: Repo{Path: `C:\Backups\restic`, PasswordTarget: "x"}, want: "id is required"},
		{name: "invalid id", repo: Repo{ID: "bad id", Path: `C:\Backups\restic`, PasswordTarget: "x"}, want: "id must match"},
		{name: "missing path", repo: Repo{ID: "main", PasswordTarget: "x"}, want: "path is required"},
		{name: "missing password target", repo: Repo{ID: "main", Path: `C:\Backups\restic`}, want: "passwordTarget is required"},
		{name: "empty include path", repo: Repo{ID: "main", Path: `C:\Backups\restic`, PasswordTarget: "x", Include: []string{" "}}, want: "include contains empty path"},
		{name: "empty exclude path", repo: Repo{ID: "main", Path: `C:\Backups\restic`, PasswordTarget: "x", Exclude: []string{" "}}, want: "exclude contains empty path"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(Config{Version: CurrentVersion(), Repos: []Repo{tc.repo}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate error = %v, want contains %q", err, tc.want)
			}
		})
	}

	err := Validate(Config{Version: CurrentVersion(), Repos: []Repo{{ID: "main", Path: `C:\Backups\restic`, PasswordTarget: "x"}, {ID: "main", Path: `C:\Backups\restic2`, PasswordTarget: "y"}}})
	if err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("expected duplicate id validation error, got %v", err)
	}
}

func TestNormalizeSortsReposAndInitializesLists(t *testing.T) {
	cfg := Config{
		Version: CurrentVersion(),
		Repos: []Repo{
			{ID: "z", Path: `C:\Backups\z`, PasswordTarget: "  "},
			{ID: "a", Path: `C:\Backups\a`, PasswordTarget: ""},
		},
	}

	n := normalize(cfg)
	if len(n.Repos) != 2 || n.Repos[0].ID != "a" || n.Repos[1].ID != "z" {
		t.Fatalf("normalize did not sort repos by ID: %#v", n.Repos)
	}
	for _, repo := range n.Repos {
		if repo.Include == nil || repo.Exclude == nil {
			t.Fatalf("normalize did not initialize path lists: %#v", repo)
		}
		if repo.PasswordTarget == "" {
			t.Fatalf("normalize did not assign password target: %#v", repo)
		}
	}
}

func TestRepoExistsAndFindRepoByIDMiss(t *testing.T) {
	cfg := Config{Version: CurrentVersion(), Repos: []Repo{{ID: "main", Path: `C:\Backups\restic`, PasswordTarget: "x"}}}
	if RepoExists(cfg, "missing") {
		t.Fatal("RepoExists returned true for missing repo")
	}
	if repo, ok := FindRepoByID(cfg, "missing"); ok || repo.ID != "" {
		t.Fatalf("FindRepoByID missing returned %#v, %t", repo, ok)
	}
}

func TestSaveAndLoadValidationBranches(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)
	prependFakeIcaclsToPath(t)

	if err := Save(Config{Version: 0}); err == nil || !strings.Contains(err.Error(), "unsupported config version") {
		t.Fatalf("expected Save validation error, got %v", err)
	}

	if err := os.MkdirAll(filepath.Join(appData, "Rist", "config.yaml"), 0o700); err != nil {
		t.Fatalf("mkdir config.yaml dir: %v", err)
	}
	valid := Config{Version: CurrentVersion(), Repos: []Repo{{ID: "main", Path: `C:\Backups\restic`, PasswordTarget: "x"}}}
	if err := Save(valid); err == nil || !strings.Contains(err.Error(), "replace config atomically") {
		t.Fatalf("expected Save rename failure, got %v", err)
	}

	if err := os.RemoveAll(filepath.Join(appData, "Rist", "config.yaml")); err != nil {
		t.Fatalf("remove blocking config.yaml dir: %v", err)
	}
	badConfig := "version: 1\nrepos:\n  - id: main\n    path: C:\\Backups\\restic\n"
	if err := os.WriteFile(filepath.Join(appData, "Rist", "config.yaml"), []byte(badConfig), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "passwordTarget is required") {
		t.Fatalf("expected Load validation error, got %v", err)
	}
}

func TestAllowedFolderBackendHelpers(t *testing.T) {
	if !isAllowedFolderBackend(`\\server\share`) {
		t.Fatal("expected UNC path to be allowed")
	}
	if !isAllowedFolderBackend(`C:/Backups/restic`) {
		t.Fatal("expected slash absolute path to be allowed")
	}
	if isAllowedFolderBackend("relative/path") {
		t.Fatal("expected relative path to be rejected")
	}
}

type wrapErr struct {
	err    error
	unwrap error
}

func (w *wrapErr) Error() string { return w.err.Error() }
func (w *wrapErr) Unwrap() error { return w.unwrap }

func prependFakeIcaclsToPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "icacls.cmd")
	if err := os.WriteFile(script, []byte("@echo off\r\nexit /b 0\r\n"), 0o700); err != nil {
		t.Fatalf("write fake icacls: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
