package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dmundt/rist/internal/config"
	"github.com/dmundt/rist/internal/credentials"
	"github.com/dmundt/rist/internal/restic"
)

func TestRunConfigInitAndRepoFlow(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	if err := os.Setenv("APPDATA", appData); err != nil {
		t.Fatalf("set APPDATA: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := Run([]string{"config", "init"}, &out, &errOut); err != nil {
		t.Fatalf("config init failed: %v", err)
	}
	if !strings.Contains(out.String(), "config initialized") {
		t.Fatalf("unexpected output: %q", out.String())
	}

	out.Reset()
	if err := Run([]string{"repo", "add", "--id", "main", "--path", `C:\\Backups\\restic`}, &out, &errOut); err != nil {
		t.Fatalf("repo add failed: %v", err)
	}

	out.Reset()
	if err := Run([]string{"repo", "include", "add", "main", `C:\\Users\\me\\Documents`}, &out, &errOut); err != nil {
		t.Fatalf("repo include add failed: %v", err)
	}

	out.Reset()
	if err := Run([]string{"repo", "include", "list", "main"}, &out, &errOut); err != nil {
		t.Fatalf("repo include list failed: %v", err)
	}
	if !strings.Contains(out.String(), `C:\\Users\\me\\Documents`) {
		t.Fatalf("expected include path in output, got %q", out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"does-not-exist"}, &out, &errOut); err == nil {
		t.Fatal("expected unknown command error")
	}
}

func TestRunUsageAndConfigCommands(t *testing.T) {
	setupTestEnvironment(t)

	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := Run(nil, &out, &errOut); err != nil {
		t.Fatalf("Run(nil) returned error: %v", err)
	}
	if !strings.Contains(out.String(), "rist backup run <id>") {
		t.Fatalf("usage output missing expected command: %q", out.String())
	}

	out.Reset()
	if err := Run([]string{"config", "init"}, &out, &errOut); err != nil {
		t.Fatalf("config init failed: %v", err)
	}
	out.Reset()
	if err := Run([]string{"config", "show"}, &out, &errOut); err != nil {
		t.Fatalf("config show failed: %v", err)
	}
	if !strings.Contains(out.String(), "version: 1") {
		t.Fatalf("unexpected config show output: %q", out.String())
	}
	out.Reset()
	if err := Run([]string{"config", "validate"}, &out, &errOut); err != nil {
		t.Fatalf("config validate failed: %v", err)
	}
	if !strings.Contains(out.String(), "config is valid") {
		t.Fatalf("unexpected validate output: %q", out.String())
	}
	out.Reset()
	if err := Run([]string{"config", "migrate"}, &out, &errOut); err != nil {
		t.Fatalf("config migrate failed: %v", err)
	}
	if !strings.Contains(out.String(), "config already at version") {
		t.Fatalf("unexpected migrate output: %q", out.String())
	}
}

func TestRunRepoCommandsAndHelpers(t *testing.T) {
	setupTestEnvironment(t)
	seedRepoConfig(t)

	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := Run([]string{"repo", "list"}, &out, &errOut); err != nil {
		t.Fatalf("repo list failed: %v", err)
	}
	if !strings.Contains(out.String(), "main") {
		t.Fatalf("unexpected repo list output: %q", out.String())
	}

	out.Reset()
	if err := Run([]string{"repo", "options", "show", "main"}, &out, &errOut); err != nil {
		t.Fatalf("repo options show failed: %v", err)
	}
	if !strings.Contains(out.String(), "verbose=false") {
		t.Fatalf("unexpected repo options show output: %q", out.String())
	}

	out.Reset()
	if err := Run([]string{"repo", "options", "set", "main", "--verbose=false", "--one-file-system=true"}, &out, &errOut); err != nil {
		t.Fatalf("repo options set failed: %v", err)
	}
	if !strings.Contains(out.String(), "repo options updated") {
		t.Fatalf("unexpected repo options set output: %q", out.String())
	}

	out.Reset()
	if err := Run([]string{"repo", "exclude", "add", "main", `C:\tmp`}, &out, &errOut); err != nil {
		t.Fatalf("repo exclude add failed: %v", err)
	}
	out.Reset()
	if err := Run([]string{"repo", "exclude", "remove", "main", `C:\tmp`}, &out, &errOut); err != nil {
		t.Fatalf("repo exclude remove failed: %v", err)
	}
	out.Reset()
	if err := Run([]string{"repo", "include", "list", "main"}, &out, &errOut); err != nil {
		t.Fatalf("repo include list failed: %v", err)
	}
	if !strings.Contains(out.String(), `C:\Users\me\Documents`) {
		t.Fatalf("unexpected include list output: %q", out.String())
	}

	out.Reset()
	if err := Run([]string{"repo", "remove", "main"}, &out, &errOut); err != nil {
		t.Fatalf("repo remove failed: %v", err)
	}
	if !strings.Contains(out.String(), "repo removed: main") {
		t.Fatalf("unexpected repo remove output: %q", out.String())
	}
}

func TestRunResticBackedCommands(t *testing.T) {
	setupTestEnvironment(t)
	seedRepoConfig(t)

	var out bytes.Buffer
	var errOut bytes.Buffer

	commands := [][]string{
		{"repo", "init", "main"},
		{"snapshots", "main"},
		{"maintenance", "check", "main"},
		{"maintenance", "stats", "main"},
		{"maintenance", "prune", "main"},
		{"maintenance", "forget", "main", "--keep-last", "2", "--prune"},
		{"restore", "run", "main", "--snapshot", "latest", "--target", `C:\restore`},
		{"backup", "run", "main", "--dry-run"},
	}

	for _, args := range commands {
		out.Reset()
		errOut.Reset()
		if err := Run(args, &out, &errOut); err != nil {
			t.Fatalf("Run(%v) failed: %v\nstderr=%q", args, err, errOut.String())
		}
	}

}

func TestRunHelpersAndUtilities(t *testing.T) {
	setupTestEnvironment(t)
	seedRepoConfig(t)

	if shouldRenderProgress(&bytes.Buffer{}) {
		t.Fatal("bytes.Buffer should not be treated as terminal")
	}

	bar := newBackupProgressBar(io.Discard)
	bar.Update(restic.BackupStatus{PercentDone: 0.99})
	bar.Finish()
	restoreBar := newRestoreProgressBar(io.Discard)
	restoreBar.Update(restic.RestoreStatus{PercentDone: 0.5})
	restoreBar.Finish()

	if got := getRepoPathList(config.Repo{Include: []string{"a"}, Exclude: []string{"b"}}, "exclude"); len(got) != 1 || got[0] != "b" {
		t.Fatalf("unexpected exclude list: %#v", got)
	}
	repo := config.Repo{}
	setRepoPathList(&repo, "exclude", []string{"x"})
	if len(repo.Exclude) != 1 || repo.Exclude[0] != "x" {
		t.Fatalf("unexpected repo exclude after setRepoPathList: %#v", repo)
	}
	if !containsString([]string{"a", "b"}, "b") {
		t.Fatal("expected containsString to find element")
	}
	items, removed := withoutString([]string{"a", "b"}, "a")
	if !removed || len(items) != 1 || items[0] != "b" {
		t.Fatalf("unexpected withoutString result: %#v removed=%t", items, removed)
	}
}

func TestRunUIAndPasswordCommands(t *testing.T) {
	setupTestEnvironment(t)
	seedRepoConfig(t)

	uniqueTarget := fmt.Sprintf("rist-test-cli-%d", os.Getpid())
	t.Cleanup(func() {
		_ = credentials.DeleteSecret(uniqueTarget)
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	for index := range cfg.Repos {
		if cfg.Repos[index].ID == "main" {
			cfg.Repos[index].PasswordTarget = uniqueTarget
		}
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save failed: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"ui", "nope"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "unknown ui subcommand") {
		t.Fatalf("unexpected ui unknown error: %v", err)
	}
	out.Reset()
	if err := Run([]string{"ui", "serve", "--addr", "127.0.0.1:-1"}, &out, &errOut); err == nil {
		t.Fatal("expected ui serve with invalid addr to fail")
	}
	if !strings.Contains(out.String(), "ui listening on http://127.0.0.1:-1") {
		t.Fatalf("unexpected ui output: %q", out.String())
	}

	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	if _, err := writer.WriteString("super-secret\n"); err != nil {
		t.Fatalf("write stdin pipe failed: %v", err)
	}
	_ = writer.Close()
	os.Stdin = reader

	out.Reset()
	if err := Run([]string{"pw", "set", "main"}, &out, &errOut); err != nil {
		t.Fatalf("pw set failed: %v", err)
	}
	if !strings.Contains(out.String(), "password stored for repo: main") {
		t.Fatalf("unexpected pw set output: %q", out.String())
	}
	_ = reader.Close()

	out.Reset()
	if err := Run([]string{"pw", "main"}, &out, &errOut); err != nil {
		t.Fatalf("pw get failed: %v", err)
	}
	if out.String() != "super-secret" {
		t.Fatalf("unexpected pw get output: %q", out.String())
	}

	out.Reset()
	if err := Run([]string{"pw", "clear", "main"}, &out, &errOut); err != nil {
		t.Fatalf("pw clear failed: %v", err)
	}
	if !strings.Contains(out.String(), "password cleared for repo: main") {
		t.Fatalf("unexpected pw clear output: %q", out.String())
	}

	out.Reset()
	if err := Run([]string{"pw", "main"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "password not found") {
		t.Fatalf("unexpected pw get missing error: %v", err)
	}
	if _, err := resolvePasswordTarget("main"); err != nil {
		t.Fatalf("resolvePasswordTarget failed: %v", err)
	}
}

func TestRunCommandUsageErrors(t *testing.T) {
	setupTestEnvironment(t)
	seedRepoConfig(t)

	var out bytes.Buffer
	var errOut bytes.Buffer

	tests := []struct {
		args     []string
		contains string
	}{
		{[]string{"config"}, "missing config subcommand"},
		{[]string{"config", "nope"}, "unknown config subcommand"},
		{[]string{"repo"}, "missing repo subcommand"},
		{[]string{"repo", "include"}, "missing repo include subcommand"},
		{[]string{"repo", "include", "nope"}, "unknown repo include subcommand"},
		{[]string{"repo", "include", "add", "main"}, "usage: rist repo include add <id> <path>"},
		{[]string{"backup"}, "missing backup subcommand"},
		{[]string{"backup", "run"}, "usage: rist backup run <id>"},
		{[]string{"backup", "run", "main", "--dry-run", "extra"}, "usage: rist backup run <id>"},
		{[]string{"snapshots"}, "usage: rist snapshots <id>"},
		{[]string{"maintenance"}, "missing maintenance subcommand"},
		{[]string{"maintenance", "check"}, "usage: rist maintenance check <id>"},
		{[]string{"maintenance", "prune"}, "usage: rist maintenance prune <id>"},
		{[]string{"maintenance", "stats"}, "usage: rist maintenance stats <id>"},
		{[]string{"maintenance", "forget"}, "usage: rist maintenance forget <id>"},
		{[]string{"maintenance", "forget", "main", "--keep-last", "2", "extra"}, "usage: rist maintenance forget <id>"},
		{[]string{"restore"}, "missing restore subcommand"},
		{[]string{"restore", "run"}, "usage: rist restore run <id>"},
		{[]string{"pw"}, "missing repo id or pw subcommand"},
		{[]string{"pw", "set"}, "usage: rist pw set <id>"},
		{[]string{"pw", "clear"}, "usage: rist pw clear <id>"},
	}

	for _, tc := range tests {
		out.Reset()
		errOut.Reset()
		err := Run(tc.args, &out, &errOut)
		if err == nil || !strings.Contains(err.Error(), tc.contains) {
			t.Fatalf("Run(%v) error = %v, want contains %q", tc.args, err, tc.contains)
		}
	}
}

func TestLoadRepoByIDErrors(t *testing.T) {
	setupTestEnvironment(t)

	if _, err := loadRepoByID(" "); err == nil || !strings.Contains(err.Error(), "repo id is required") {
		t.Fatalf("loadRepoByID empty error = %v", err)
	}

	seedRepoConfig(t)
	if _, err := loadRepoByID("missing"); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("loadRepoByID missing error = %v", err)
	}
}

func TestRepoPathAndListBranches(t *testing.T) {
	setupTestEnvironment(t)
	seedRepoConfig(t)

	var out bytes.Buffer
	if err := addRepoPath("main", " ", "include", &out); err == nil || !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("addRepoPath empty error = %v", err)
	}
	if err := addRepoPath("missing", `C:\X`, "include", &out); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("addRepoPath missing repo error = %v", err)
	}

	out.Reset()
	if err := addRepoPath("main", `C:\Users\me\Documents`, "include", &out); err != nil {
		t.Fatalf("addRepoPath duplicate returned error: %v", err)
	}
	if !strings.Contains(out.String(), "already exists for repo") {
		t.Fatalf("unexpected add duplicate output: %q", out.String())
	}

	out.Reset()
	if err := removeRepoPath("main", " ", "include", &out); err == nil || !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("removeRepoPath empty error = %v", err)
	}
	if err := removeRepoPath("missing", `C:\Users\me\Documents`, "include", &out); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("removeRepoPath missing repo error = %v", err)
	}

	out.Reset()
	if err := removeRepoPath("main", `C:\not-present`, "include", &out); err != nil {
		t.Fatalf("removeRepoPath not-present returned error: %v", err)
	}
	if !strings.Contains(out.String(), "not present for repo") {
		t.Fatalf("unexpected remove not-present output: %q", out.String())
	}

	out.Reset()
	if err := listRepoPaths("missing", "include", &out); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("listRepoPaths missing repo error = %v", err)
	}

	if err := addRepoPath("main", `C:\tmp\only-exclude`, "exclude", &out); err != nil {
		t.Fatalf("add exclude path failed: %v", err)
	}

	if err := updateRepo("main", func(repo *config.Repo) error {
		repo.Include = nil
		return nil
	}, nil); err != nil {
		t.Fatalf("updateRepo clear include failed: %v", err)
	}
	out.Reset()
	if err := listRepoPaths("main", "include", &out); err != nil {
		t.Fatalf("listRepoPaths include failed: %v", err)
	}
	if !strings.Contains(out.String(), "no include paths for repo") {
		t.Fatalf("unexpected include empty output: %q", out.String())
	}
}

func TestRepoOptionsAndUpdateRepoBranches(t *testing.T) {
	setupTestEnvironment(t)
	seedRepoConfig(t)

	var out bytes.Buffer
	if err := runRepoOptions([]string{}, &out); err == nil || !strings.Contains(err.Error(), "missing repo options subcommand") {
		t.Fatalf("runRepoOptions missing error = %v", err)
	}
	if err := runRepoOptions([]string{"show"}, &out); err == nil || !strings.Contains(err.Error(), "usage: rist repo options show <id>") {
		t.Fatalf("runRepoOptions show usage error = %v", err)
	}
	if err := runRepoOptions([]string{"nope"}, &out); err == nil || !strings.Contains(err.Error(), "unknown repo options subcommand") {
		t.Fatalf("runRepoOptions unknown error = %v", err)
	}

	if err := runRepoOptionsSet([]string{"main"}, &out); err == nil || !strings.Contains(err.Error(), "at least one option flag must be provided") {
		t.Fatalf("runRepoOptionsSet missing flags error = %v", err)
	}
	if err := runRepoOptionsSet([]string{"main", "--one-file-system=maybe"}, &out); err == nil || !strings.Contains(err.Error(), "--one-file-system must be true or false") {
		t.Fatalf("runRepoOptionsSet one-file-system parse error = %v", err)
	}
	if err := runRepoOptionsSet([]string{"main", "--verbose=maybe"}, &out); err == nil || !strings.Contains(err.Error(), "--verbose must be true or false") {
		t.Fatalf("runRepoOptionsSet verbose parse error = %v", err)
	}

	if err := updateRepo("missing", func(*config.Repo) error { return nil }, nil); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("updateRepo missing error = %v", err)
	}
	if err := updateRepo("main", func(*config.Repo) error { return errors.New("mutate boom") }, nil); err == nil || !strings.Contains(err.Error(), "mutate boom") {
		t.Fatalf("updateRepo mutate error = %v", err)
	}
}

func TestRepoInitListMaintenanceRestoreBranches(t *testing.T) {
	setupTestEnvironment(t)
	seedRepoConfig(t)

	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := runRepoInit([]string{}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "usage: rist repo init <id>") {
		t.Fatalf("runRepoInit usage error = %v", err)
	}
	out.Reset()
	if err := runRepoInit([]string{"main"}, &out, &errOut); err != nil {
		t.Fatalf("runRepoInit failed: %v", err)
	}

	if err := runMaintenanceCheck([]string{}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "usage: rist maintenance check <id>") {
		t.Fatalf("runMaintenanceCheck usage error = %v", err)
	}
	out.Reset()
	if err := runMaintenanceCheck([]string{"main"}, &out, &errOut); err != nil {
		t.Fatalf("runMaintenanceCheck failed: %v", err)
	}

	if err := runMaintenanceStats([]string{}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "usage: rist maintenance stats <id>") {
		t.Fatalf("runMaintenanceStats usage error = %v", err)
	}

	if err := runRestoreRun([]string{"main", "extra"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "usage: rist restore run <id>") {
		t.Fatalf("runRestoreRun usage extra error = %v", err)
	}
	out.Reset()
	if err := runRestoreRun([]string{"main", "--snapshot", "latest", "--target", `C:\restore`, "--dry-run"}, &out, &errOut); err != nil {
		t.Fatalf("runRestoreRun dry-run failed: %v", err)
	}
	if !strings.Contains(out.String(), "restore dry-run completed") {
		t.Fatalf("unexpected restore dry-run output: %q", out.String())
	}

	if err := updateRepo("main", func(repo *config.Repo) error {
		repo.Include = nil
		return nil
	}, nil); err != nil {
		t.Fatalf("updateRepo clear include failed: %v", err)
	}
	if err := runBackupRun([]string{"main"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "repo include list is empty") {
		t.Fatalf("runBackupRun expected empty include error, got %v", err)
	}
}

func TestRepoListNoReposAndProgressNilBranches(t *testing.T) {
	setupTestEnvironment(t)

	var out bytes.Buffer
	if err := Run([]string{"config", "init"}, &out, io.Discard); err != nil {
		t.Fatalf("config init failed: %v", err)
	}
	out.Reset()
	if err := runRepoList(&out); err != nil {
		t.Fatalf("runRepoList returned error: %v", err)
	}
	if !strings.Contains(out.String(), "no repos configured") {
		t.Fatalf("unexpected runRepoList output: %q", out.String())
	}

	var backupNil *backupProgressBar
	backupNil.Update(restic.BackupStatus{PercentDone: 0.1})
	backupNil.Finish()
	var restoreNil *restoreProgressBar
	restoreNil.Update(restic.RestoreStatus{PercentDone: 0.1})
	restoreNil.Finish()
	if shouldRenderProgress(nil) {
		t.Fatal("expected shouldRenderProgress(nil) to be false")
	}
}

func TestCommandBranchHelpers(t *testing.T) {
	backup := newBackupProgressBar(nil)
	backup.Update(restic.BackupStatus{PercentDone: 1.2})
	backup.Finish()
	restore := newRestoreProgressBar(nil)
	restore.Update(restic.RestoreStatus{PercentDone: -0.5})
	restore.Finish()
}

func TestRepoAddRemoveAndRestorePasswordErrorBranches(t *testing.T) {
	setupTestEnvironment(t)
	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := Run([]string{"config", "init"}, &out, &errOut); err != nil {
		t.Fatalf("config init failed: %v", err)
	}

	if err := runRepoAdd([]string{}, &out); err == nil || !strings.Contains(err.Error(), "--id is required") {
		t.Fatalf("runRepoAdd missing id error = %v", err)
	}
	if err := runRepoAdd([]string{"--id", "main"}, &out); err == nil || !strings.Contains(err.Error(), "--path is required") {
		t.Fatalf("runRepoAdd missing path error = %v", err)
	}
	if err := runRepoAdd([]string{"--id", "main", "--path", `C:\Backups\restic\main`}, &out); err != nil {
		t.Fatalf("runRepoAdd failed: %v", err)
	}
	if err := runRepoAdd([]string{"--id", "main", "--path", `C:\Backups\restic\main`}, &out); err == nil || !strings.Contains(err.Error(), "repo already exists") {
		t.Fatalf("runRepoAdd duplicate error = %v", err)
	}

	if err := runRepoRemove([]string{}, &out); err == nil || !strings.Contains(err.Error(), "usage: rist repo remove <id>") {
		t.Fatalf("runRepoRemove usage error = %v", err)
	}
	if err := runRepoRemove([]string{"missing"}, &out); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("runRepoRemove missing repo error = %v", err)
	}

	if err := runRestoreRun([]string{"main", "--target", `C:\restore`}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "snapshot is required") {
		t.Fatalf("runRestoreRun missing snapshot error = %v", err)
	}
	if err := runRestoreRun([]string{"main", "--snapshot", "latest"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "target is required") {
		t.Fatalf("runRestoreRun missing target error = %v", err)
	}

	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	if _, err := writer.WriteString("\n"); err != nil {
		t.Fatalf("write stdin failed: %v", err)
	}
	_ = writer.Close()
	os.Stdin = reader
	if err := runPasswordSet([]string{"main"}, &out); err == nil || !strings.Contains(err.Error(), "password from stdin is empty") {
		t.Fatalf("runPasswordSet empty stdin error = %v", err)
	}
	_ = reader.Close()

	if err := runPasswordGet("missing", &out); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("runPasswordGet missing repo error = %v", err)
	}
}

func TestRunConfigAndUIErrorBranches(t *testing.T) {
	var out bytes.Buffer

	t.Setenv("APPDATA", "")
	if err := runConfig([]string{"init"}, &out); err == nil {
		t.Fatal("expected runConfig init to fail when APPDATA is unset")
	}
	if err := runConfig([]string{"show"}, &out); err == nil {
		t.Fatal("expected runConfig show to fail when config is unavailable")
	}
	if err := runConfig([]string{"validate"}, &out); err == nil {
		t.Fatal("expected runConfig validate to fail when config is unavailable")
	}
	if err := runConfig([]string{"migrate"}, &out); err == nil {
		t.Fatal("expected runConfig migrate to fail when config is unavailable")
	}

	if err := runUI([]string{}, &out); err == nil || !strings.Contains(err.Error(), "missing ui subcommand") {
		t.Fatalf("runUI missing subcommand error = %v", err)
	}
	if err := runUI([]string{"serve", "--addr"}, &out); err == nil {
		t.Fatal("expected runUI to fail on invalid flag usage")
	}
}

func TestSnapshotsMaintenanceAndForgetValidationBranches(t *testing.T) {
	setupTestEnvironment(t)
	seedRepoConfig(t)

	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := runSnapshots([]string{"missing"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("runSnapshots missing repo error = %v", err)
	}
	if err := runMaintenanceCheck([]string{"missing"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("runMaintenanceCheck missing repo error = %v", err)
	}
	if err := runMaintenancePrune([]string{"missing"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("runMaintenancePrune missing repo error = %v", err)
	}
	if err := runMaintenanceStats([]string{"missing"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("runMaintenanceStats missing repo error = %v", err)
	}
	if err := runMaintenanceForget([]string{"main"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "at least one keep policy") {
		t.Fatalf("runMaintenanceForget expected policy validation error, got %v", err)
	}
}

func TestBackupRestoreAndPasswordAdditionalErrorBranches(t *testing.T) {
	setupTestEnvironment(t)
	seedRepoConfig(t)

	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := runBackupRun([]string{"main", "--bad-flag"}, &out, &errOut); err == nil {
		t.Fatal("expected runBackupRun to fail on invalid flag")
	}
	if err := runBackupRun([]string{"missing"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("runBackupRun missing repo error = %v", err)
	}
	if err := runBackupRun([]string{"main", "--dry-run"}, nil, &errOut); err != nil {
		t.Fatalf("runBackupRun with nil output returned error: %v", err)
	}

	if err := runRestoreRun([]string{"main", "--bad-flag"}, &out, &errOut); err == nil {
		t.Fatal("expected runRestoreRun to fail on invalid flag")
	}
	if err := runRestoreRun([]string{"missing", "--snapshot", "latest", "--target", `C:\restore`}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "repo not found") {
		t.Fatalf("runRestoreRun missing repo error = %v", err)
	}

	target, err := resolvePasswordTarget("main")
	if err != nil {
		t.Fatalf("resolvePasswordTarget failed: %v", err)
	}
	if err := credentials.SetSecret(target, "seed-secret"); err != nil {
		t.Fatalf("seed credential failed: %v", err)
	}

	errWriter := failingWriter{}
	if err := runPasswordGet("main", errWriter); err == nil || !strings.Contains(err.Error(), "write boom") {
		t.Fatalf("runPasswordGet write error = %v", err)
	}

	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	_ = r.Close()
	os.Stdin = r
	if err := runPasswordSet([]string{"main"}, &out); err == nil || !strings.Contains(err.Error(), "read password from stdin") {
		t.Fatalf("runPasswordSet read error = %v", err)
	}
	_ = w.Close()

	out.Reset()
	if err := runPasswordClear([]string{"main"}, &out); err != nil {
		t.Fatalf("first runPasswordClear returned error: %v", err)
	}
	out.Reset()
	if err := runPasswordClear([]string{"main"}, &out); err != nil {
		t.Fatalf("second runPasswordClear returned error: %v", err)
	}
	if !strings.Contains(out.String(), "password cleared for repo: main") {
		t.Fatalf("unexpected second clear output: %q", out.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write boom")
}

func setupTestEnvironment(t *testing.T) {
	t.Helper()
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)
	toolDir := t.TempDir()
	writeBatch(t, filepath.Join(toolDir, "icacls.cmd"), "@echo off\r\nexit /b 0\r\n")
	writeBatch(t, filepath.Join(toolDir, "restic.cmd"), fakeResticBatch())
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func seedRepoConfig(t *testing.T) {
	t.Helper()
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"config", "init"}, &out, &errOut); err != nil {
		t.Fatalf("config init failed: %v", err)
	}
	out.Reset()
	if err := Run([]string{"repo", "add", "--id", "main", "--path", `C:\Backups\restic\main`}, &out, &errOut); err != nil {
		t.Fatalf("repo add failed: %v", err)
	}
	out.Reset()
	if err := Run([]string{"repo", "include", "add", "main", `C:\Users\me\Documents`}, &out, &errOut); err != nil {
		t.Fatalf("repo include add failed: %v", err)
	}
}

func writeBatch(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write batch %s: %v", path, err)
	}
}

func fakeResticBatch() string {
	return "@echo off\r\n" +
		"set args=%*\r\n" +
		"echo %args% | findstr /C:\" init --json\" >nul && (echo {\"id\":\"repoid\",\"repository\":\"fake-repo\"} & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" snapshots --json\" >nul && (echo [{\"id\":\"snap1\",\"time\":\"2024-01-01T00:00:00Z\"}] & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" check --json\" >nul && (echo {\"message_type\":\"summary\",\"num_errors\":0,\"suggest_prune\":false,\"suggest_repair_index\":false} & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" stats --json\" >nul && (echo {\"total_size\":123,\"total_file_count\":5,\"snapshots_count\":2} & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" restore \" >nul && echo %args% | findstr /C:\" --json\" >nul && (echo {\"message_type\":\"summary\",\"files_restored\":4,\"files_skipped\":1} & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" backup --json\" >nul && (echo {\"message_type\":\"summary\",\"snapshot_id\":\"snap123\",\"files_new\":1,\"files_changed\":2,\"dry_run\":false} & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" prune\" >nul && (echo prune complete & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" forget\" >nul && (echo forget complete & exit /b 0)\r\n" +
		"exit /b 0\r\n"
}
