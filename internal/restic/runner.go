package restic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/dmundt/rist/internal/config"
)

type Runner struct {
	ResticPath string
	CLIPath    string
}

type ErrorKind string

const (
	ErrorKindUnknown           ErrorKind = "unknown"
	ErrorKindWrongPassword     ErrorKind = "wrong-password"
	ErrorKindRepositoryDamaged ErrorKind = "repository-damaged"
	ErrorKindRepositoryLocked  ErrorKind = "repository-locked"
	ErrorKindRepositoryMissing ErrorKind = "repository-missing"
	ErrorKindPartialRead       ErrorKind = "partial-read"
	ErrorKindInterrupted       ErrorKind = "interrupted"
	ErrorKindRuntime           ErrorKind = "runtime-error"
	ErrorKindExecutableMissing ErrorKind = "executable-missing"
)

type CommandError struct {
	Err  error
	Code int
	Kind ErrorKind
}

func (e CommandError) Error() string {
	if e.Kind == "" || e.Kind == ErrorKindUnknown {
		return fmt.Sprintf("restic command failed with exit code %d: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("restic command failed (%s) with exit code %d: %v", e.Kind, e.Code, e.Err)
}

func (e CommandError) Unwrap() error {
	return e.Err
}

func (e CommandError) ExitCode() int {
	return e.Code
}

func (e CommandError) Category() string {
	if e.Kind == "" {
		return string(ErrorKindUnknown)
	}
	return string(e.Kind)
}

func NewDefaultRunner() (Runner, error) {
	cliPath, err := os.Executable()
	if err != nil {
		return Runner{}, fmt.Errorf("resolve current executable: %w", err)
	}
	return Runner{ResticPath: "restic", CLIPath: cliPath}, nil
}

func (r Runner) RunInit(ctx context.Context, repo config.Repo, stdout, stderr io.Writer) error {
	result, err := r.RunInitJSON(ctx, repo)
	if err != nil {
		return err
	}
	if stdout != nil {
		_, _ = fmt.Fprintf(stdout, "repository=%s\nid=%s\n", result.Repository, result.ID)
	}
	return nil
}

func (r Runner) RunBackup(ctx context.Context, repo config.Repo, stdout, stderr io.Writer) error {
	return r.RunBackupWithOptions(ctx, repo, BackupRunOptions{}, stdout, stderr)
}

func (r Runner) RunBackupWithOptions(ctx context.Context, repo config.Repo, options BackupRunOptions, stdout, stderr io.Writer) error {
	return r.RunBackupWithProgress(ctx, repo, options, nil, stdout, stderr)
}

func (r Runner) RunBackupWithProgress(ctx context.Context, repo config.Repo, options BackupRunOptions, onStatus func(BackupStatus), stdout, stderr io.Writer) error {
	statusSeen := false
	statusHandler := onStatus
	if onStatus != nil {
		statusHandler = func(s BackupStatus) {
			statusSeen = true
			onStatus(s)
		}
	}

	result, err := r.RunBackupJSONStream(ctx, repo, options, statusHandler)
	if err != nil {
		return err
	}
	if onStatus != nil && statusSeen && stdout != nil {
		onStatus(BackupStatus{PercentDone: 1})
		_, _ = fmt.Fprintln(stdout)
	}
	if stdout != nil && result.Summary != nil {
		title := "backup completed"
		if result.Summary.DryRun {
			title = "backup dry-run completed"
		}
		_, _ = fmt.Fprintln(stdout, title)
		_, _ = fmt.Fprintf(stdout, "- repo: %s\n", repo.ID)
		_, _ = fmt.Fprintf(stdout, "- snapshot: %s\n", result.Summary.SnapshotID)
		_, _ = fmt.Fprintf(stdout, "- files: +%d new, ~%d changed\n", result.Summary.FilesNew, result.Summary.FilesChanged)
	}
	return nil
}

func (r Runner) RunSnapshots(ctx context.Context, repo config.Repo, stdout, stderr io.Writer) error {
	items, err := r.RunSnapshotsJSON(ctx, repo)
	if err != nil {
		return err
	}
	for _, item := range items {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\n", item.ID, item.Time)
	}
	return nil
}

func (r Runner) RunCheck(ctx context.Context, repo config.Repo, stdout, stderr io.Writer) error {
	result, err := r.RunCheckJSON(ctx, repo)
	if err != nil {
		return err
	}
	if stdout != nil && result.Summary != nil {
		_, _ = fmt.Fprintln(stdout, "check summary")
		_, _ = fmt.Fprintf(stdout, "- errors: %d\n", result.Summary.NumErrors)
		_, _ = fmt.Fprintf(stdout, "- suggest repair index: %t\n", result.Summary.SuggestRepairIndex)
		_, _ = fmt.Fprintf(stdout, "- suggest prune: %t\n", result.Summary.SuggestPrune)
	}
	return nil
}

func (r Runner) RunPrune(ctx context.Context, repo config.Repo, stdout, stderr io.Writer) error {
	args := BuildPruneArgs(repo, r.passwordCommand(repo.ID))
	return r.run(ctx, args, stdout, stderr)
}

func (r Runner) RunForget(ctx context.Context, repo config.Repo, policy ForgetPolicy, stdout, stderr io.Writer) error {
	args, err := BuildForgetArgs(repo, r.passwordCommand(repo.ID), policy)
	if err != nil {
		return err
	}
	return r.run(ctx, args, stdout, stderr)
}

func (r Runner) RunStats(ctx context.Context, repo config.Repo, stdout, stderr io.Writer) error {
	result, err := r.RunStatsJSON(ctx, repo)
	if err != nil {
		return err
	}
	if stdout != nil {
		_, _ = fmt.Fprintln(stdout, "repository stats")
		_, _ = fmt.Fprintf(stdout, "- total size: %d bytes\n", result.TotalSize)
		_, _ = fmt.Fprintf(stdout, "- total files: %d\n", result.TotalFileCount)
		_, _ = fmt.Fprintf(stdout, "- snapshots: %d\n", result.SnapshotsCount)
	}
	return nil
}

func (r Runner) RunRestore(ctx context.Context, repo config.Repo, snapshot, target string, stdout, stderr io.Writer) error {
	return r.RunRestoreWithOptions(ctx, repo, RestoreRunOptions{Snapshot: snapshot, Target: target}, stdout, stderr)
}

func (r Runner) RunRestoreWithOptions(ctx context.Context, repo config.Repo, options RestoreRunOptions, stdout, stderr io.Writer) error {
	return r.RunRestoreWithProgress(ctx, repo, options, nil, stdout, stderr)
}

func (r Runner) RunRestoreWithProgress(ctx context.Context, repo config.Repo, options RestoreRunOptions, onStatus func(RestoreStatus), stdout, stderr io.Writer) error {
	statusSeen := false
	statusHandler := onStatus
	if onStatus != nil {
		statusHandler = func(s RestoreStatus) {
			statusSeen = true
			onStatus(s)
		}
	}

	result, err := r.RunRestoreJSONStream(ctx, repo, options, statusHandler)
	if err != nil {
		return err
	}
	if onStatus != nil && statusSeen && stdout != nil {
		onStatus(RestoreStatus{PercentDone: 1})
		_, _ = fmt.Fprintln(stdout)
	}
	if stdout != nil && result.Summary != nil {
		_, _ = fmt.Fprintln(stdout, "restore summary")
		_, _ = fmt.Fprintf(stdout, "- files restored: %d\n", result.Summary.FilesRestored)
		_, _ = fmt.Fprintf(stdout, "- files skipped: %d\n", result.Summary.FilesSkipped)
	}
	return nil
}

func BuildInitArgs(repo config.Repo, passwordCommand string) []string {
	return []string{
		"-r", repo.Path,
		"--password-command", passwordCommand,
		"init",
	}
}

func BuildBackupArgs(repo config.Repo, passwordCommand string) ([]string, error) {
	return BuildBackupArgsWithOptions(repo, passwordCommand, BackupRunOptions{})
}

func BuildBackupArgsWithOptions(repo config.Repo, passwordCommand string, options BackupRunOptions) ([]string, error) {
	if len(repo.Include) == 0 {
		return nil, errors.New("repo include list is empty")
	}

	args := []string{
		"-r", repo.Path,
		"--password-command", passwordCommand,
	}

	if repo.Options.Verbose {
		args = append(args, "-v")
	}

	args = append(args, "backup")

	if repo.Options.OneFileSystem {
		args = append(args, "--one-file-system")
	}
	if options.DryRun {
		args = append(args, "--dry-run")
	}

	for _, path := range repo.Exclude {
		args = append(args, "--exclude", path)
	}

	args = append(args, repo.Include...)
	return args, nil
}

func BuildSnapshotsArgs(repo config.Repo, passwordCommand string) []string {
	return []string{
		"-r", repo.Path,
		"--password-command", passwordCommand,
		"snapshots",
	}
}

func BuildCheckArgs(repo config.Repo, passwordCommand string) []string {
	return []string{
		"-r", repo.Path,
		"--password-command", passwordCommand,
		"check",
	}
}

func BuildPruneArgs(repo config.Repo, passwordCommand string) []string {
	return []string{
		"-r", repo.Path,
		"--password-command", passwordCommand,
		"prune",
	}
}

type ForgetPolicy struct {
	KeepLast    int
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
	KeepYearly  int
	Prune       bool
	DryRun      bool
}

func BuildForgetArgs(repo config.Repo, passwordCommand string, policy ForgetPolicy) ([]string, error) {
	if policy.KeepLast <= 0 && policy.KeepDaily <= 0 && policy.KeepWeekly <= 0 && policy.KeepMonthly <= 0 && policy.KeepYearly <= 0 {
		return nil, errors.New("at least one keep policy must be greater than zero")
	}

	args := []string{
		"-r", repo.Path,
		"--password-command", passwordCommand,
		"forget",
	}

	if policy.KeepLast > 0 {
		args = append(args, "--keep-last", fmt.Sprintf("%d", policy.KeepLast))
	}
	if policy.KeepDaily > 0 {
		args = append(args, "--keep-daily", fmt.Sprintf("%d", policy.KeepDaily))
	}
	if policy.KeepWeekly > 0 {
		args = append(args, "--keep-weekly", fmt.Sprintf("%d", policy.KeepWeekly))
	}
	if policy.KeepMonthly > 0 {
		args = append(args, "--keep-monthly", fmt.Sprintf("%d", policy.KeepMonthly))
	}
	if policy.KeepYearly > 0 {
		args = append(args, "--keep-yearly", fmt.Sprintf("%d", policy.KeepYearly))
	}
	if policy.Prune {
		args = append(args, "--prune")
	}
	if policy.DryRun {
		args = append(args, "--dry-run")
	}

	return args, nil
}

func BuildStatsArgs(repo config.Repo, passwordCommand string) []string {
	return []string{
		"-r", repo.Path,
		"--password-command", passwordCommand,
		"stats",
	}
}

func BuildRestoreArgs(repo config.Repo, passwordCommand, snapshot, target string) ([]string, error) {
	return BuildRestoreArgsWithOptions(repo, passwordCommand, RestoreRunOptions{Snapshot: snapshot, Target: target})
}

func BuildRestoreArgsWithOptions(repo config.Repo, passwordCommand string, options RestoreRunOptions) ([]string, error) {
	snapshot := options.Snapshot
	target := options.Target
	if strings.TrimSpace(snapshot) == "" {
		return nil, errors.New("snapshot is required")
	}
	if strings.TrimSpace(target) == "" {
		return nil, errors.New("target is required")
	}

	args := []string{
		"-r", repo.Path,
		"--password-command", passwordCommand,
		"restore", snapshot,
		"--target", target,
	}
	if options.DryRun {
		args = append(args, "--dry-run")
	}
	return args, nil
}

func (r Runner) run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, r.ResticPath, args...)
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	cmd.Stdout = stdout

	var stderrBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(stderr, &stderrBuf)

	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return CommandError{Err: errors.New("restic executable not found in PATH"), Code: 127, Kind: ErrorKindExecutableMissing}
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			kind := classifyResticExit(exitErr.ExitCode(), stderrBuf.String())
			return CommandError{Err: err, Code: exitErr.ExitCode(), Kind: kind}
		}

		return CommandError{Err: err, Code: 1, Kind: ErrorKindUnknown}
	}
	return nil
}

func (r Runner) passwordCommand(repoID string) string {
	return fmt.Sprintf("\"%s\" pw %s", escapeQuotes(r.CLIPath), repoID)
}

func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, "\"", "\\\"")
}

func classifyResticError(stderr string) ErrorKind {
	msg := strings.ToLower(stderr)

	switch {
	case strings.Contains(msg, "wrong password") || strings.Contains(msg, "password is incorrect") || strings.Contains(msg, "decrypt master key"):
		return ErrorKindWrongPassword
	case strings.Contains(msg, "ciphertext verification failed") || strings.Contains(msg, "config or key") && strings.Contains(msg, "damaged"):
		return ErrorKindRepositoryDamaged
	case strings.Contains(msg, "repository is already locked") || strings.Contains(msg, "unable to create lock"):
		return ErrorKindRepositoryLocked
	case strings.Contains(msg, "repository does not exist") || strings.Contains(msg, "unable to open config file"):
		return ErrorKindRepositoryMissing
	default:
		return ErrorKindUnknown
	}
}

func classifyResticExit(code int, stderr string) ErrorKind {
	switch code {
	case 3:
		return ErrorKindPartialRead
	case 10:
		return ErrorKindRepositoryMissing
	case 11:
		return ErrorKindRepositoryLocked
	case 12:
		return ErrorKindWrongPassword
	case 130:
		return ErrorKindInterrupted
	case 2:
		return ErrorKindRuntime
	case 1:
		return classifyResticError(stderr)
	default:
		return ErrorKindUnknown
	}
}
