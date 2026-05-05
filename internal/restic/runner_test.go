package restic

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dmundt/rist/internal/config"
)

func TestBuildInitArgs(t *testing.T) {
	repo := config.Repo{ID: "main", Path: `C:\\Backups\\restic`}
	got := BuildInitArgs(repo, `"C:\\Apps\\rist.exe" pw main`)
	want := []string{"-r", `C:\\Backups\\restic`, "--password-command", `"C:\\Apps\\rist.exe" pw main`, "init"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildInitArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildBackupArgs(t *testing.T) {
	repo := config.Repo{
		ID:      "main",
		Path:    `C:\\Backups\\restic`,
		Include: []string{`C:\\Users\\me\\Documents`, `C:\\Users\\me\\Pictures`},
		Exclude: []string{`C:\\Users\\me\\Documents\\tmp`},
		Options: config.Options{OneFileSystem: true, Verbose: true},
	}

	got, err := BuildBackupArgs(repo, `"C:\\Apps\\rist.exe" pw main`)
	if err != nil {
		t.Fatalf("BuildBackupArgs returned error: %v", err)
	}

	want := []string{
		"-r", `C:\\Backups\\restic`,
		"--password-command", `"C:\\Apps\\rist.exe" pw main`,
		"-v",
		"backup",
		"--one-file-system",
		"--exclude", `C:\\Users\\me\\Documents\\tmp`,
		`C:\\Users\\me\\Documents`,
		`C:\\Users\\me\\Pictures`,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildBackupArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildBackupArgsRequiresInclude(t *testing.T) {
	repo := config.Repo{ID: "main", Path: `C:\\Backups\\restic`}
	if _, err := BuildBackupArgs(repo, `"C:\\Apps\\rist.exe" pw main`); err == nil {
		t.Fatal("expected error for empty include list")
	}
}

func TestBuildSnapshotsArgs(t *testing.T) {
	repo := config.Repo{ID: "main", Path: `C:\\Backups\\restic`}
	got := BuildSnapshotsArgs(repo, `"C:\\Apps\\rist.exe" pw main`)
	want := []string{"-r", `C:\\Backups\\restic`, "--password-command", `"C:\\Apps\\rist.exe" pw main`, "snapshots"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildSnapshotsArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildSnapshotsJSONArgs(t *testing.T) {
	repo := config.Repo{ID: "main", Path: `C:\\Backups\\restic`}
	got := BuildSnapshotsJSONArgs(repo, `"C:\\Apps\\rist.exe" pw main`)
	want := []string{"-r", `C:\\Backups\\restic`, "--password-command", `"C:\\Apps\\rist.exe" pw main`, "snapshots", "--json"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildSnapshotsJSONArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildForgetArgs(t *testing.T) {
	repo := config.Repo{ID: "main", Path: `C:\\Backups\\restic`}
	got, err := BuildForgetArgs(repo, `"C:\\Apps\\rist.exe" pw main`, ForgetPolicy{KeepDaily: 7, KeepWeekly: 4, Prune: true})
	if err != nil {
		t.Fatalf("BuildForgetArgs returned error: %v", err)
	}

	want := []string{
		"-r", `C:\\Backups\\restic`,
		"--password-command", `"C:\\Apps\\rist.exe" pw main`,
		"forget",
		"--keep-daily", "7",
		"--keep-weekly", "4",
		"--prune",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildForgetArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildForgetArgsWithDryRun(t *testing.T) {
	repo := config.Repo{ID: "main", Path: `C:\\Backups\\restic`}
	got, err := BuildForgetArgs(repo, `"C:\\Apps\\rist.exe" pw main`, ForgetPolicy{KeepLast: 2, DryRun: true})
	if err != nil {
		t.Fatalf("BuildForgetArgs returned error: %v", err)
	}

	want := []string{
		"-r", `C:\\Backups\\restic`,
		"--password-command", `"C:\\Apps\\rist.exe" pw main`,
		"forget",
		"--keep-last", "2",
		"--dry-run",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildForgetArgs dry-run mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildForgetArgsRequiresPolicy(t *testing.T) {
	repo := config.Repo{ID: "main", Path: `C:\\Backups\\restic`}
	if _, err := BuildForgetArgs(repo, `"C:\\Apps\\rist.exe" pw main`, ForgetPolicy{}); err == nil {
		t.Fatal("expected error when no keep policy is set")
	}
}

func TestBuildRestoreArgs(t *testing.T) {
	repo := config.Repo{ID: "main", Path: `C:\\Backups\\restic`}
	got, err := BuildRestoreArgs(repo, `"C:\\Apps\\rist.exe" pw main`, "latest", `C:\\restore-target`)
	if err != nil {
		t.Fatalf("BuildRestoreArgs returned error: %v", err)
	}

	want := []string{
		"-r", `C:\\Backups\\restic`,
		"--password-command", `"C:\\Apps\\rist.exe" pw main`,
		"restore", "latest",
		"--target", `C:\\restore-target`,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildRestoreArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildRestoreArgsWithDryRun(t *testing.T) {
	repo := config.Repo{ID: "main", Path: `C:\\Backups\\restic`}
	got, err := BuildRestoreArgsWithOptions(repo, `"C:\\Apps\\rist.exe" pw main`, RestoreRunOptions{Snapshot: "latest", Target: `C:\\restore-target`, DryRun: true})
	if err != nil {
		t.Fatalf("BuildRestoreArgsWithOptions returned error: %v", err)
	}

	want := []string{
		"-r", `C:\\Backups\\restic`,
		"--password-command", `"C:\\Apps\\rist.exe" pw main`,
		"restore", "latest",
		"--target", `C:\\restore-target`,
		"--dry-run",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildRestoreArgsWithOptions mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildBackupJSONArgs(t *testing.T) {
	repo := config.Repo{
		ID:      "main",
		Path:    `C:\\Backups\\restic`,
		Include: []string{`C:\\Users\\me\\Documents`},
		Options: config.Options{Verbose: true},
	}

	got, err := BuildBackupJSONArgs(repo, `"C:\\Apps\\rist.exe" pw main`)
	if err != nil {
		t.Fatalf("BuildBackupJSONArgs returned error: %v", err)
	}

	want := []string{
		"-r", `C:\\Backups\\restic`,
		"--password-command", `"C:\\Apps\\rist.exe" pw main`,
		"-v",
		"backup",
		"--json",
		`C:\\Users\\me\\Documents`,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildBackupJSONArgs mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestBuildBackupJSONArgsWithDryRun(t *testing.T) {
	repo := config.Repo{
		ID:      "main",
		Path:    `C:\\Backups\\restic`,
		Include: []string{`C:\\Users\\me\\Documents`},
	}

	got, err := BuildBackupJSONArgsWithOptions(repo, `"C:\\Apps\\rist.exe" pw main`, BackupRunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("BuildBackupJSONArgsWithOptions returned error: %v", err)
	}

	want := []string{
		"-r", `C:\\Backups\\restic`,
		"--password-command", `"C:\\Apps\\rist.exe" pw main`,
		"backup",
		"--json",
		"--dry-run",
		`C:\\Users\\me\\Documents`,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildBackupJSONArgsWithOptions mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestParseBackupResult(t *testing.T) {
	stdout := []byte("{\"message_type\":\"status\",\"percent_done\":42.5,\"files_done\":3}\n{\"message_type\":\"summary\",\"snapshot_id\":\"abc123\",\"files_new\":2,\"files_changed\":1}\n")
	stderr := []byte("{\"message_type\":\"error\",\"during\":\"scan\",\"item\":\"C:/tmp/missing\",\"error\":{\"message\":\"access denied\"}}\n")

	result, err := parseBackupResult(stdout, stderr)
	if err != nil {
		t.Fatalf("parseBackupResult returned error: %v", err)
	}
	if len(result.Status) != 1 {
		t.Fatalf("expected 1 status message, got %d", len(result.Status))
	}
	if result.Summary == nil || result.Summary.SnapshotID != "abc123" {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if len(result.Errors) != 1 || result.Errors[0].Error.Message != "access denied" {
		t.Fatalf("unexpected errors: %#v", result.Errors)
	}
}

func TestParseRestoreResult(t *testing.T) {
	stdout := []byte("{\"message_type\":\"status\",\"files_restored\":4}\n{\"message_type\":\"summary\",\"files_restored\":4,\"files_skipped\":1}\n")

	result, err := parseRestoreResult(stdout, nil)
	if err != nil {
		t.Fatalf("parseRestoreResult returned error: %v", err)
	}
	if len(result.Status) != 1 {
		t.Fatalf("expected 1 status message, got %d", len(result.Status))
	}
	if result.Summary == nil || result.Summary.FilesSkipped != 1 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
}

func TestParseRestoreJSONLine(t *testing.T) {
	result := RestoreResult{}
	line := []byte("{\"message_type\":\"status\",\"percent_done\":67.3,\"files_restored\":4}")

	status, err := parseRestoreJSONLine(line, &result)
	if err != nil {
		t.Fatalf("parseRestoreJSONLine returned error: %v", err)
	}
	if status == nil {
		t.Fatal("expected status payload")
	}
	if len(result.Status) != 1 || result.Status[0].FilesRestored != 4 {
		t.Fatalf("unexpected restore status list: %#v", result.Status)
	}
}

func TestParseRestoreJSONLineInvalid(t *testing.T) {
	result := RestoreResult{}
	if _, err := parseRestoreJSONLine([]byte("not-json"), &result); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseCheckResult(t *testing.T) {
	stdout := []byte("{\"message_type\":\"summary\",\"num_errors\":2,\"suggest_prune\":true}\n")
	stderr := []byte("{\"message_type\":\"error\",\"message\":\"pack verification failed\"}\n")

	result, err := parseCheckResult(stdout, stderr)
	if err != nil {
		t.Fatalf("parseCheckResult returned error: %v", err)
	}
	if result.Summary == nil || result.Summary.NumErrors != 2 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if len(result.Errors) != 1 || result.Errors[0].Message != "pack verification failed" {
		t.Fatalf("unexpected errors: %#v", result.Errors)
	}
}

func TestParseExitError(t *testing.T) {
	payload, ok := parseExitError("{\"message_type\":\"exit_error\",\"code\":12,\"message\":\"wrong password\"}\n")
	if !ok {
		t.Fatal("expected exit_error payload to be parsed")
	}
	if payload.Code != 12 || payload.Message != "wrong password" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestBuildCommandErrorUsesJSONExitError(t *testing.T) {
	err := buildCommandError(&exec.ExitError{}, "{\"message_type\":\"exit_error\",\"code\":12,\"message\":\"wrong password\"}\n")
	var commandErr CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("expected CommandError, got %T", err)
	}
	if commandErr.Code != 12 || commandErr.Kind != ErrorKindWrongPassword {
		t.Fatalf("unexpected command error: %#v", commandErr)
	}
}

func TestRunnerCommandExecutionPaths(t *testing.T) {
	setupFakeRestic(t)
	runner := Runner{ResticPath: "restic", CLIPath: `C:\Apps\rist.exe`}
	repo := config.Repo{ID: "main", Path: `C:\Backups\restic`, Include: []string{`C:\Users\me\Documents`}, Options: config.Options{Verbose: true}}

	if got, err := runner.RunInitJSON(context.Background(), repo); err != nil || got.Repository != "fake-repo" {
		t.Fatalf("RunInitJSON = %#v, %v", got, err)
	}
	if got, err := runner.RunSnapshotsJSON(context.Background(), repo); err != nil || len(got) != 1 {
		t.Fatalf("RunSnapshotsJSON = %#v, %v", got, err)
	}
	if got, err := runner.RunStatsJSON(context.Background(), repo); err != nil || got.SnapshotsCount != 2 {
		t.Fatalf("RunStatsJSON = %#v, %v", got, err)
	}
	if got, err := runner.RunCheckJSON(context.Background(), repo); err != nil || got.Summary == nil || got.Summary.NumErrors != 0 {
		t.Fatalf("RunCheckJSON = %#v, %v", got, err)
	}
	if got, err := runner.RunBackupJSONWithOptions(context.Background(), repo, BackupRunOptions{}); err != nil || got.Summary == nil || got.Summary.SnapshotID != "snap123" {
		t.Fatalf("RunBackupJSONWithOptions = %#v, %v", got, err)
	}
	if got, err := runner.RunBackupJSON(context.Background(), repo); err != nil || got.Summary == nil || got.Summary.SnapshotID != "snap123" {
		t.Fatalf("RunBackupJSON = %#v, %v", got, err)
	}
	if got, err := runner.RunRestoreJSONWithOptions(context.Background(), repo, RestoreRunOptions{Snapshot: "latest", Target: `C:\restore`}); err != nil || got.Summary == nil || got.Summary.FilesRestored != 4 {
		t.Fatalf("RunRestoreJSONWithOptions = %#v, %v", got, err)
	}
	if got, err := runner.RunRestoreJSON(context.Background(), repo, "latest", `C:\restore`); err != nil || got.Summary == nil || got.Summary.FilesRestored != 4 {
		t.Fatalf("RunRestoreJSON = %#v, %v", got, err)
	}

	var out bytes.Buffer
	if err := runner.RunInit(context.Background(), repo, &out, io.Discard); err != nil || !bytes.Contains(out.Bytes(), []byte("repository=fake-repo")) {
		t.Fatalf("RunInit output = %q, err=%v", out.String(), err)
	}
	out.Reset()
	if err := runner.RunBackup(context.Background(), repo, &out, io.Discard); err != nil || !bytes.Contains(out.Bytes(), []byte("Backup Completed")) {
		t.Fatalf("RunBackup output = %q, err=%v", out.String(), err)
	}
	out.Reset()
	if err := runner.RunBackupWithOptions(context.Background(), repo, BackupRunOptions{}, &out, io.Discard); err != nil || !bytes.Contains(out.Bytes(), []byte("Backup Completed")) {
		t.Fatalf("RunBackupWithOptions output = %q, err=%v", out.String(), err)
	}
	out.Reset()
	backupStatusCalls := 0
	if err := runner.RunBackupWithProgress(context.Background(), repo, BackupRunOptions{}, func(status BackupStatus) {
		backupStatusCalls++
	}, &out, io.Discard); err != nil {
		t.Fatalf("RunBackupWithProgress returned error: %v", err)
	}
	if backupStatusCalls == 0 {
		t.Fatal("expected backup status callback to be invoked")
	}
	out.Reset()
	if err := runner.RunSnapshots(context.Background(), repo, &out, io.Discard); err != nil || !bytes.Contains(out.Bytes(), []byte("snap1")) {
		t.Fatalf("RunSnapshots output = %q, err=%v", out.String(), err)
	}
	out.Reset()
	if err := runner.RunCheck(context.Background(), repo, &out, io.Discard); err != nil || !bytes.Contains(out.Bytes(), []byte("Check Summary")) {
		t.Fatalf("RunCheck output = %q, err=%v", out.String(), err)
	}
	out.Reset()
	if err := runner.RunStats(context.Background(), repo, &out, io.Discard); err != nil || !bytes.Contains(out.Bytes(), []byte("Repository Stats")) {
		t.Fatalf("RunStats output = %q, err=%v", out.String(), err)
	}
	out.Reset()
	if err := runner.RunRestoreWithOptions(context.Background(), repo, RestoreRunOptions{Snapshot: "latest", Target: `C:\restore`}, &out, io.Discard); err != nil || !bytes.Contains(out.Bytes(), []byte("Restore Summary")) {
		t.Fatalf("RunRestoreWithOptions output = %q, err=%v", out.String(), err)
	}
	out.Reset()
	if err := runner.RunRestore(context.Background(), repo, "latest", `C:\restore`, &out, io.Discard); err != nil || !bytes.Contains(out.Bytes(), []byte("Restore Summary")) {
		t.Fatalf("RunRestore output = %q, err=%v", out.String(), err)
	}
	out.Reset()
	restoreStatusCalls := 0
	if err := runner.RunRestoreWithProgress(context.Background(), repo, RestoreRunOptions{Snapshot: "latest", Target: `C:\restore`}, func(status RestoreStatus) {
		restoreStatusCalls++
	}, &out, io.Discard); err != nil {
		t.Fatalf("RunRestoreWithProgress returned error: %v", err)
	}
	if restoreStatusCalls == 0 {
		t.Fatal("expected restore status callback to be invoked")
	}
	out.Reset()
	if err := runner.RunPrune(context.Background(), repo, &out, io.Discard); err != nil {
		t.Fatalf("RunPrune returned error: %v", err)
	}
	out.Reset()
	if err := runner.RunForget(context.Background(), repo, ForgetPolicy{KeepLast: 2, Prune: true}, &out, io.Discard); err != nil {
		t.Fatalf("RunForget returned error: %v", err)
	}

	defaultRunner, err := NewDefaultRunner()
	if err != nil || defaultRunner.ResticPath == "" || defaultRunner.CLIPath == "" {
		t.Fatalf("NewDefaultRunner = %#v, %v", defaultRunner, err)
	}
	checkArgs := BuildCheckArgs(repo, "pw")
	if checkArgs[len(checkArgs)-1] != "check" {
		t.Fatal("BuildCheckArgs did not include check command")
	}
	pruneArgs := BuildPruneArgs(repo, "pw")
	if pruneArgs[len(pruneArgs)-1] != "prune" {
		t.Fatal("BuildPruneArgs did not include prune command")
	}
	statsArgs := BuildStatsArgs(repo, "pw")
	if statsArgs[len(statsArgs)-1] != "stats" {
		t.Fatal("BuildStatsArgs did not include stats command")
	}
	if initJSONArgs := BuildInitJSONArgs(repo, "pw"); initJSONArgs[len(initJSONArgs)-1] != "--json" {
		t.Fatal("BuildInitJSONArgs did not include --json")
	}
	if statsJSONArgs := BuildStatsJSONArgs(repo, "pw"); statsJSONArgs[len(statsJSONArgs)-1] != "--json" {
		t.Fatal("BuildStatsJSONArgs did not include --json")
	}
	if checkJSONArgs := BuildCheckJSONArgs(repo, "pw"); checkJSONArgs[len(checkJSONArgs)-1] != "--json" {
		t.Fatal("BuildCheckJSONArgs did not include --json")
	}
	restoreJSONArgs, err := BuildRestoreJSONArgs(repo, "pw", "latest", `C:\restore`)
	if err != nil {
		t.Fatalf("BuildRestoreJSONArgs returned error: %v", err)
	}
	if restoreJSONArgs[len(restoreJSONArgs)-1] != "--json" {
		t.Fatal("BuildRestoreJSONArgs did not include --json")
	}
	if decodeSingleJSONLine([]byte("{}\n{}\n"), &struct{}{}) == nil {
		t.Fatal("expected decodeSingleJSONLine to reject multiple lines")
	}
	if !strings.Contains(annotateJSONParseError("backup", nil, nil, errors.New("boom")).Error(), "parse restic backup json") {
		t.Fatal("annotateJSONParseError did not include command name")
	}
	commandErr := CommandError{Err: errors.New("x"), Code: 7, Kind: ErrorKindRuntime}
	if commandErr.Error() == "" || commandErr.Unwrap() == nil || commandErr.ExitCode() != 7 || commandErr.Category() != string(ErrorKindRuntime) {
		t.Fatalf("unexpected CommandError helpers: %#v", commandErr)
	}
	if escapeQuotes(`a"b`) != `a\"b` {
		t.Fatalf("unexpected escapeQuotes output: %q", escapeQuotes(`a"b`))
	}
	if !strings.Contains(runner.passwordCommand("main"), "pw main") {
		t.Fatalf("unexpected passwordCommand output: %q", runner.passwordCommand("main"))
	}
}

func TestRunnerExecutableMissing(t *testing.T) {
	runner := Runner{ResticPath: "definitely-not-a-real-restic-binary", CLIPath: `C:\Apps\rist.exe`}
	repo := config.Repo{ID: "main", Path: `C:\Backups\restic`, Include: []string{`C:\Users\me\Documents`}}
	err := runner.RunPrune(context.Background(), repo, io.Discard, io.Discard)
	var commandErr CommandError
	if !errors.As(err, &commandErr) || commandErr.Kind != ErrorKindExecutableMissing {
		t.Fatalf("expected executable missing CommandError, got %v", err)
	}
}

func setupFakeRestic(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "restic.cmd")
	content := "@echo off\r\n" +
		"set args=%*\r\n" +
		"echo %args% | findstr /C:\" init --json\" >nul && (echo {\"id\":\"repoid\",\"repository\":\"fake-repo\"} & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" snapshots --json\" >nul && (echo [{\"id\":\"snap1\",\"time\":\"2024-01-01T00:00:00Z\"}] & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" check --json\" >nul && (echo {\"message_type\":\"summary\",\"num_errors\":0,\"suggest_prune\":false,\"suggest_repair_index\":false} & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" stats --json\" >nul && (echo {\"total_size\":123,\"total_file_count\":5,\"snapshots_count\":2} & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" backup --json\" >nul && (echo {\"message_type\":\"status\",\"percent_done\":0.5,\"files_done\":3} & echo {\"message_type\":\"summary\",\"snapshot_id\":\"snap123\",\"files_new\":1,\"files_changed\":2,\"dry_run\":false} & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" restore \" >nul && echo %args% | findstr /C:\" --json\" >nul && (echo {\"message_type\":\"status\",\"percent_done\":0.5,\"files_restored\":2} & echo {\"message_type\":\"summary\",\"files_restored\":4,\"files_skipped\":1} & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" prune\" >nul && (echo prune complete & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" forget\" >nul && (echo forget complete & exit /b 0)\r\n" +
		"exit /b 0\r\n"
	if err := os.WriteFile(script, []byte(content), 0o700); err != nil {
		t.Fatalf("write fake restic: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestClassifyResticError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   ErrorKind
	}{
		{name: "wrong password", stderr: "Fatal: wrong password or no key found", want: ErrorKindWrongPassword},
		{name: "damaged", stderr: "Fatal: config or key abc is damaged: ciphertext verification failed", want: ErrorKindRepositoryDamaged},
		{name: "locked", stderr: "Fatal: repository is already locked", want: ErrorKindRepositoryLocked},
		{name: "missing", stderr: "Fatal: repository does not exist", want: ErrorKindRepositoryMissing},
		{name: "other", stderr: "Fatal: something unexpected", want: ErrorKindUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyResticError(tc.stderr)
			if got != tc.want {
				t.Fatalf("classifyResticError() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyResticExit(t *testing.T) {
	tests := []struct {
		name   string
		code   int
		stderr string
		want   ErrorKind
	}{
		{name: "partial read", code: 3, stderr: "some file could not be read", want: ErrorKindPartialRead},
		{name: "repo missing", code: 10, stderr: "repository does not exist", want: ErrorKindRepositoryMissing},
		{name: "repo locked", code: 11, stderr: "repository is already locked", want: ErrorKindRepositoryLocked},
		{name: "wrong password", code: 12, stderr: "whatever", want: ErrorKindWrongPassword},
		{name: "interrupted", code: 130, stderr: "interrupted", want: ErrorKindInterrupted},
		{name: "runtime", code: 2, stderr: "runtime panic", want: ErrorKindRuntime},
		{name: "generic falls back to stderr", code: 1, stderr: "Fatal: config or key damaged: ciphertext verification failed", want: ErrorKindRepositoryDamaged},
		{name: "unknown code remains unknown", code: 99, stderr: "repository is already locked", want: ErrorKindUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyResticExit(tc.code, tc.stderr)
			if got != tc.want {
				t.Fatalf("classifyResticExit(%d) = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}

func TestCommandErrorUnknownKindFormattingAndCategory(t *testing.T) {
	err := CommandError{Err: errors.New("boom"), Code: 1}
	if !strings.Contains(err.Error(), "exit code 1") {
		t.Fatalf("unexpected CommandError.Error output: %q", err.Error())
	}
	if err.Category() != string(ErrorKindUnknown) {
		t.Fatalf("Category() = %q, want %q", err.Category(), ErrorKindUnknown)
	}
}

func TestBuildCommandErrorWithoutJSONPayload(t *testing.T) {
	err := buildCommandError(&exec.ExitError{}, "not json\n")
	var commandErr CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("expected CommandError, got %T", err)
	}
	if commandErr.Code == 0 {
		t.Fatalf("unexpected command error code: %#v", commandErr)
	}
}

func TestParseExitErrorNotFound(t *testing.T) {
	if payload, ok := parseExitError("{\"message_type\":\"status\"}\n"); ok || payload.Code != 0 {
		t.Fatalf("expected no exit_error payload, got %#v ok=%t", payload, ok)
	}
}

func TestParseBackupJSONLineVerboseAndUnknown(t *testing.T) {
	result := BackupResult{}
	status, err := parseBackupJSONLine([]byte("{\"message_type\":\"verbose_status\",\"item\":\"C:/x\",\"action\":\"scan\"}"), &result)
	if err != nil {
		t.Fatalf("parseBackupJSONLine verbose returned error: %v", err)
	}
	if status != nil {
		t.Fatal("expected nil status for verbose_status")
	}
	if len(result.Verbose) != 1 {
		t.Fatalf("expected one verbose entry, got %d", len(result.Verbose))
	}

	status, err = parseBackupJSONLine([]byte("{\"message_type\":\"other\"}"), &result)
	if err != nil {
		t.Fatalf("parseBackupJSONLine unknown returned error: %v", err)
	}
	if status != nil {
		t.Fatal("expected nil status for unknown message type")
	}
}

func TestParseRestoreJSONLineVerboseAndUnknown(t *testing.T) {
	result := RestoreResult{}
	status, err := parseRestoreJSONLine([]byte("{\"message_type\":\"verbose_status\",\"item\":\"C:/x\",\"action\":\"restore\"}"), &result)
	if err != nil {
		t.Fatalf("parseRestoreJSONLine verbose returned error: %v", err)
	}
	if status != nil {
		t.Fatal("expected nil status for verbose_status")
	}
	if len(result.Verbose) != 1 {
		t.Fatalf("expected one verbose entry, got %d", len(result.Verbose))
	}

	status, err = parseRestoreJSONLine([]byte("{\"message_type\":\"other\"}"), &result)
	if err != nil {
		t.Fatalf("parseRestoreJSONLine unknown returned error: %v", err)
	}
	if status != nil {
		t.Fatal("expected nil status for unknown message type")
	}
}

func TestRunnerRunDirectModes(t *testing.T) {
	runner := Runner{ResticPath: "cmd", CLIPath: `C:\Apps\rist.exe`}

	if err := runner.run(context.Background(), []string{"/c", "echo", "ok"}, nil, nil); err != nil {
		t.Fatalf("runner.run success returned error: %v", err)
	}

	var stderr bytes.Buffer
	err := runner.run(context.Background(), []string{"/c", "echo", "wrong password 1>&2", "&&", "exit", "12"}, io.Discard, &stderr)
	var commandErr CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("expected CommandError, got %v", err)
	}
	if commandErr.Code != 12 || commandErr.Kind != ErrorKindWrongPassword {
		t.Fatalf("unexpected command error: %#v stderr=%q", commandErr, stderr.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = runner.run(ctx, []string{"/c", "echo", "ok"}, nil, nil)
	if !errors.As(err, &commandErr) || commandErr.Kind != ErrorKindUnknown || commandErr.Code != 1 {
		t.Fatalf("expected unknown CommandError for canceled context, got %v", err)
	}
}

func TestJSONRunnerParseErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resticbad.cmd")
	content := "@echo off\r\n" +
		"set args=%*\r\n" +
		"echo %args% | findstr /C:\" init --json\" >nul && (echo {bad & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" snapshots --json\" >nul && (echo not-json & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" stats --json\" >nul && (echo not-json & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" check --json\" >nul && (echo not-json & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" backup --json\" >nul && (echo not-json & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" restore \" >nul && echo %args% | findstr /C:\" --json\" >nul && (echo not-json & exit /b 0)\r\n" +
		"echo {}\r\n"
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write fake resticbad: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	runner := Runner{ResticPath: "resticbad", CLIPath: `C:\Apps\rist.exe`}
	repo := config.Repo{ID: "main", Path: `C:\Backups\restic`, Include: []string{`C:\Users\me\Documents`}}

	if _, err := runner.RunInitJSON(context.Background(), repo); err == nil || !strings.Contains(err.Error(), "parse restic init json") {
		t.Fatalf("RunInitJSON parse error = %v", err)
	}
	if _, err := runner.RunSnapshotsJSON(context.Background(), repo); err == nil || !strings.Contains(err.Error(), "parse restic snapshots json") {
		t.Fatalf("RunSnapshotsJSON parse error = %v", err)
	}
	if _, err := runner.RunStatsJSON(context.Background(), repo); err == nil || !strings.Contains(err.Error(), "parse restic stats json") {
		t.Fatalf("RunStatsJSON parse error = %v", err)
	}
	if _, err := runner.RunCheckJSON(context.Background(), repo); err == nil || !strings.Contains(err.Error(), "parse restic check json") {
		t.Fatalf("RunCheckJSON parse error = %v", err)
	}
	if _, err := runner.RunBackupJSONWithOptions(context.Background(), repo, BackupRunOptions{}); err == nil || !strings.Contains(err.Error(), "parse restic backup json") {
		t.Fatalf("RunBackupJSONWithOptions parse error = %v", err)
	}
	if _, err := runner.RunRestoreJSONWithOptions(context.Background(), repo, RestoreRunOptions{Snapshot: "latest", Target: `C:\restore`}); err == nil || !strings.Contains(err.Error(), "parse restic restore json") {
		t.Fatalf("RunRestoreJSONWithOptions parse error = %v", err)
	}
}

func TestJSONStreamAndCaptureErrorBranches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resticstreambad.cmd")
	content := "@echo off\r\n" +
		"set args=%*\r\n" +
		"echo %args% | findstr /C:\" backup --json\" >nul && (echo not-json & exit /b 0)\r\n" +
		"echo %args% | findstr /C:\" restore \" >nul && echo %args% | findstr /C:\" --json\" >nul && (echo not-json & exit /b 0)\r\n" +
		"exit /b 0\r\n"
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write fake resticstreambad: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	runner := Runner{ResticPath: "resticstreambad", CLIPath: `C:\Apps\rist.exe`}
	repo := config.Repo{ID: "main", Path: `C:\Backups\restic`, Include: []string{`C:\Users\me\Documents`}}

	if _, err := runner.RunBackupJSONStream(context.Background(), repo, BackupRunOptions{}, nil); err == nil || !strings.Contains(err.Error(), "parse restic backup json") {
		t.Fatalf("RunBackupJSONStream parse error = %v", err)
	}
	if _, err := runner.RunRestoreJSONStream(context.Background(), repo, RestoreRunOptions{Snapshot: "latest", Target: `C:\restore`}, nil); err == nil || !strings.Contains(err.Error(), "parse restic restore json") {
		t.Fatalf("RunRestoreJSONStream parse error = %v", err)
	}

	runner = Runner{ResticPath: "definitely-not-a-real-restic-binary", CLIPath: `C:\Apps\rist.exe`}
	if _, _, err := runner.runCapture(context.Background(), []string{"version"}); err == nil {
		t.Fatal("expected runCapture to return executable missing error")
	} else {
		var commandErr CommandError
		if !errors.As(err, &commandErr) || commandErr.Kind != ErrorKindExecutableMissing {
			t.Fatalf("expected executable missing CommandError, got %v", err)
		}
	}
}

func TestBuildJSONOptionBranchesAndValidation(t *testing.T) {
	repo := config.Repo{
		ID:      "main",
		Path:    `C:\Backups\restic`,
		Include: []string{`C:\Users\me\Documents`},
		Exclude: []string{`C:\Users\me\Documents\tmp`},
		Options: config.Options{OneFileSystem: true, Verbose: true},
	}

	args, err := BuildBackupJSONArgsWithOptions(repo, `"C:\Apps\rist.exe" pw main`, BackupRunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("BuildBackupJSONArgsWithOptions returned error: %v", err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--one-file-system") || !strings.Contains(joined, "--dry-run") || !strings.Contains(joined, "--exclude") {
		t.Fatalf("BuildBackupJSONArgsWithOptions missing expected options: %v", args)
	}

	if _, err := BuildBackupJSONArgsWithOptions(config.Repo{ID: "main", Path: `C:\Backups\restic`}, `pw`, BackupRunOptions{}); err == nil {
		t.Fatal("expected BuildBackupJSONArgsWithOptions to fail with empty include list")
	}

	restoreArgs, err := BuildRestoreJSONArgsWithOptions(repo, `pw`, RestoreRunOptions{Snapshot: "latest", Target: `C:\restore`, DryRun: true})
	if err != nil {
		t.Fatalf("BuildRestoreJSONArgsWithOptions returned error: %v", err)
	}
	if !strings.Contains(strings.Join(restoreArgs, " "), "--dry-run") {
		t.Fatalf("BuildRestoreJSONArgsWithOptions missing --dry-run: %v", restoreArgs)
	}

	if _, err := BuildRestoreJSONArgsWithOptions(repo, `pw`, RestoreRunOptions{Snapshot: " ", Target: `C:\restore`}); err == nil || !strings.Contains(err.Error(), "snapshot is required") {
		t.Fatalf("expected snapshot validation error, got %v", err)
	}
	if _, err := BuildRestoreJSONArgsWithOptions(repo, `pw`, RestoreRunOptions{Snapshot: "latest", Target: " "}); err == nil || !strings.Contains(err.Error(), "target is required") {
		t.Fatalf("expected target validation error, got %v", err)
	}
}

func TestParseOperationAndCheckResultEdgeCases(t *testing.T) {
	errs := parseOperationErrors([]byte("not-json\n{\"message_type\":\"status\"}\n{\"message_type\":\"error\",\"during\":\"scan\",\"item\":\"C:/x\",\"error\":{\"message\":\"denied\"}}\n"))
	if len(errs) != 1 || errs[0].Error.Message != "denied" {
		t.Fatalf("unexpected parseOperationErrors result: %#v", errs)
	}

	result, err := parseCheckResult([]byte("{\"message_type\":\"status\"}\n"), []byte("not-json\n"))
	if err != nil {
		t.Fatalf("parseCheckResult returned unexpected error: %v", err)
	}
	if result.Summary != nil || len(result.Errors) != 0 {
		t.Fatalf("unexpected parseCheckResult output: %#v", result)
	}

	if _, err := parseCheckResult([]byte("not-json\n"), nil); err == nil || !strings.Contains(err.Error(), "parse restic check json") {
		t.Fatalf("expected parseCheckResult stdout parse error, got %v", err)
	}
}

func TestBuildCommandErrorGenericFallback(t *testing.T) {
	err := buildCommandError(errors.New("boom"), "")
	var commandErr CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("expected CommandError, got %T", err)
	}
	if commandErr.Code != 1 || commandErr.Kind != ErrorKindUnknown {
		t.Fatalf("unexpected generic fallback command error: %#v", commandErr)
	}
}
