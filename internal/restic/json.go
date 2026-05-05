package restic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/dmundt/rist/internal/config"
)

type InitResult struct {
	MessageType string `json:"message_type,omitempty"`
	ID          string `json:"id"`
	Repository  string `json:"repository"`
}

type Snapshot struct {
	ID       string           `json:"id"`
	ShortID  string           `json:"short_id,omitempty"`
	Time     string           `json:"time,omitempty"`
	Paths    []string         `json:"paths,omitempty"`
	Hostname string           `json:"hostname,omitempty"`
	Username string           `json:"username,omitempty"`
	Tags     []string         `json:"tags,omitempty"`
	Summary  *SnapshotSummary `json:"summary,omitempty"`
}

type SnapshotSummary struct {
	BackupStart         string `json:"backup_start,omitempty"`
	BackupEnd           string `json:"backup_end,omitempty"`
	FilesNew            uint64 `json:"files_new,omitempty"`
	FilesChanged        uint64 `json:"files_changed,omitempty"`
	FilesUnmodified     uint64 `json:"files_unmodified,omitempty"`
	DirsNew             uint64 `json:"dirs_new,omitempty"`
	DirsChanged         uint64 `json:"dirs_changed,omitempty"`
	DirsUnmodified      uint64 `json:"dirs_unmodified,omitempty"`
	DataAdded           uint64 `json:"data_added,omitempty"`
	DataAddedPacked     uint64 `json:"data_added_packed,omitempty"`
	TotalFilesProcessed uint64 `json:"total_files_processed,omitempty"`
	TotalBytesProcessed uint64 `json:"total_bytes_processed,omitempty"`
	SnapshotID          string `json:"snapshot_id,omitempty"`
}

type StatsResult struct {
	TotalSize              uint64  `json:"total_size,omitempty"`
	TotalFileCount         uint64  `json:"total_file_count,omitempty"`
	TotalBlobCount         uint64  `json:"total_blob_count,omitempty"`
	SnapshotsCount         uint64  `json:"snapshots_count,omitempty"`
	TotalUncompressedSize  uint64  `json:"total_uncompressed_size,omitempty"`
	CompressionRatio       float64 `json:"compression_ratio,omitempty"`
	CompressionProgress    float64 `json:"compression_progress,omitempty"`
	CompressionSpaceSaving float64 `json:"compression_space_saving,omitempty"`
}

type CheckResult struct {
	Summary *CheckSummary `json:"summary,omitempty"`
	Errors  []CheckError  `json:"errors,omitempty"`
}

type CheckSummary struct {
	MessageType        string   `json:"message_type,omitempty"`
	NumErrors          int64    `json:"num_errors,omitempty"`
	BrokenPacks        []string `json:"broken_packs,omitempty"`
	SuggestRepairIndex bool     `json:"suggest_repair_index,omitempty"`
	SuggestPrune       bool     `json:"suggest_prune,omitempty"`
}

type CheckError struct {
	MessageType string `json:"message_type,omitempty"`
	Message     string `json:"message,omitempty"`
}

type BackupResult struct {
	Status  []BackupStatus        `json:"status,omitempty"`
	Verbose []BackupVerboseStatus `json:"verbose_status,omitempty"`
	Summary *BackupSummary        `json:"summary,omitempty"`
	Errors  []OperationError      `json:"errors,omitempty"`
}

type BackupStatus struct {
	MessageType      string   `json:"message_type,omitempty"`
	SecondsElapsed   uint64   `json:"seconds_elapsed,omitempty"`
	SecondsRemaining uint64   `json:"seconds_remaining,omitempty"`
	PercentDone      float64  `json:"percent_done,omitempty"`
	TotalFiles       uint64   `json:"total_files,omitempty"`
	FilesDone        uint64   `json:"files_done,omitempty"`
	TotalBytes       uint64   `json:"total_bytes,omitempty"`
	BytesDone        uint64   `json:"bytes_done,omitempty"`
	ErrorCount       uint64   `json:"error_count,omitempty"`
	CurrentFiles     []string `json:"current_files,omitempty"`
}

type BackupVerboseStatus struct {
	MessageType        string  `json:"message_type,omitempty"`
	Action             string  `json:"action,omitempty"`
	Item               string  `json:"item,omitempty"`
	Duration           float64 `json:"duration,omitempty"`
	DataSize           uint64  `json:"data_size,omitempty"`
	DataSizeInRepo     uint64  `json:"data_size_in_repo,omitempty"`
	MetadataSize       uint64  `json:"metadata_size,omitempty"`
	MetadataSizeInRepo uint64  `json:"metadata_size_in_repo,omitempty"`
	TotalFiles         uint64  `json:"total_files,omitempty"`
}

type BackupSummary struct {
	MessageType         string  `json:"message_type,omitempty"`
	DryRun              bool    `json:"dry_run,omitempty"`
	FilesNew            uint64  `json:"files_new,omitempty"`
	FilesChanged        uint64  `json:"files_changed,omitempty"`
	FilesUnmodified     uint64  `json:"files_unmodified,omitempty"`
	DirsNew             uint64  `json:"dirs_new,omitempty"`
	DirsChanged         uint64  `json:"dirs_changed,omitempty"`
	DirsUnmodified      uint64  `json:"dirs_unmodified,omitempty"`
	DataAdded           uint64  `json:"data_added,omitempty"`
	DataAddedPacked     uint64  `json:"data_added_packed,omitempty"`
	TotalFilesProcessed uint64  `json:"total_files_processed,omitempty"`
	TotalBytesProcessed uint64  `json:"total_bytes_processed,omitempty"`
	BackupStart         string  `json:"backup_start,omitempty"`
	BackupEnd           string  `json:"backup_end,omitempty"`
	TotalDuration       float64 `json:"total_duration,omitempty"`
	SnapshotID          string  `json:"snapshot_id,omitempty"`
}

type RestoreResult struct {
	Status  []RestoreStatus        `json:"status,omitempty"`
	Verbose []RestoreVerboseStatus `json:"verbose_status,omitempty"`
	Summary *RestoreSummary        `json:"summary,omitempty"`
	Errors  []OperationError       `json:"errors,omitempty"`
}

type RestoreStatus struct {
	MessageType    string  `json:"message_type,omitempty"`
	SecondsElapsed uint64  `json:"seconds_elapsed,omitempty"`
	PercentDone    float64 `json:"percent_done,omitempty"`
	TotalFiles     uint64  `json:"total_files,omitempty"`
	FilesRestored  uint64  `json:"files_restored,omitempty"`
	FilesSkipped   uint64  `json:"files_skipped,omitempty"`
	FilesDeleted   uint64  `json:"files_deleted,omitempty"`
	TotalBytes     uint64  `json:"total_bytes,omitempty"`
	BytesRestored  uint64  `json:"bytes_restored,omitempty"`
	BytesSkipped   uint64  `json:"bytes_skipped,omitempty"`
}

type RestoreVerboseStatus struct {
	MessageType string `json:"message_type,omitempty"`
	Action      string `json:"action,omitempty"`
	Item        string `json:"item,omitempty"`
	Size        uint64 `json:"size,omitempty"`
}

type RestoreSummary struct {
	MessageType    string `json:"message_type,omitempty"`
	SecondsElapsed uint64 `json:"seconds_elapsed,omitempty"`
	TotalFiles     uint64 `json:"total_files,omitempty"`
	FilesRestored  uint64 `json:"files_restored,omitempty"`
	FilesSkipped   uint64 `json:"files_skipped,omitempty"`
	FilesDeleted   uint64 `json:"files_deleted,omitempty"`
	TotalBytes     uint64 `json:"total_bytes,omitempty"`
	BytesRestored  uint64 `json:"bytes_restored,omitempty"`
	BytesSkipped   uint64 `json:"bytes_skipped,omitempty"`
}

type OperationError struct {
	MessageType string          `json:"message_type,omitempty"`
	Error       OperationDetail `json:"error,omitempty"`
	During      string          `json:"during,omitempty"`
	Item        string          `json:"item,omitempty"`
}

type OperationDetail struct {
	Message string `json:"message,omitempty"`
}

type exitErrorPayload struct {
	MessageType string `json:"message_type,omitempty"`
	Code        int    `json:"code,omitempty"`
	Message     string `json:"message,omitempty"`
}

type BackupRunOptions struct {
	DryRun bool
}

type RestoreRunOptions struct {
	Snapshot string
	Target   string
	DryRun   bool
}

func (r Runner) RunInitJSON(ctx context.Context, repo config.Repo) (InitResult, error) {
	var result InitResult
	stdout, stderr, err := r.runCapture(ctx, BuildInitJSONArgs(repo, r.passwordCommand(repo.ID)))
	if err != nil {
		return InitResult{}, err
	}
	if err := decodeSingleJSONLine(stdout, &result); err != nil {
		return InitResult{}, annotateJSONParseError("init", stdout, stderr, err)
	}
	return result, nil
}

func (r Runner) RunSnapshotsJSON(ctx context.Context, repo config.Repo) ([]Snapshot, error) {
	var result []Snapshot
	stdout, stderr, err := r.runCapture(ctx, BuildSnapshotsJSONArgs(repo, r.passwordCommand(repo.ID)))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, annotateJSONParseError("snapshots", stdout, stderr, err)
	}
	return result, nil
}

func (r Runner) RunStatsJSON(ctx context.Context, repo config.Repo) (StatsResult, error) {
	var result StatsResult
	stdout, stderr, err := r.runCapture(ctx, BuildStatsJSONArgs(repo, r.passwordCommand(repo.ID)))
	if err != nil {
		return StatsResult{}, err
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		return StatsResult{}, annotateJSONParseError("stats", stdout, stderr, err)
	}
	return result, nil
}

func (r Runner) RunCheckJSON(ctx context.Context, repo config.Repo) (CheckResult, error) {
	stdout, stderr, err := r.runCapture(ctx, BuildCheckJSONArgs(repo, r.passwordCommand(repo.ID)))
	if err != nil {
		return CheckResult{}, err
	}
	return parseCheckResult(stdout, stderr)
}

func (r Runner) RunBackupJSON(ctx context.Context, repo config.Repo) (BackupResult, error) {
	return r.RunBackupJSONWithOptions(ctx, repo, BackupRunOptions{})
}

func (r Runner) RunBackupJSONWithOptions(ctx context.Context, repo config.Repo, options BackupRunOptions) (BackupResult, error) {
	args, err := BuildBackupJSONArgsWithOptions(repo, r.passwordCommand(repo.ID), options)
	if err != nil {
		return BackupResult{}, err
	}
	stdout, stderr, err := r.runCapture(ctx, args)
	if err != nil {
		return BackupResult{}, err
	}
	return parseBackupResult(stdout, stderr)
}

func (r Runner) RunBackupJSONStream(ctx context.Context, repo config.Repo, options BackupRunOptions, onStatus func(BackupStatus)) (BackupResult, error) {
	args, err := BuildBackupJSONArgsWithOptions(repo, r.passwordCommand(repo.ID), options)
	if err != nil {
		return BackupResult{}, err
	}

	cmd := exec.CommandContext(ctx, r.ResticPath, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return BackupResult{}, err
	}

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return BackupResult{}, buildCommandError(err, stderrBuf.String())
	}

	result := BackupResult{}
	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		status, parseErr := parseBackupJSONLine(line, &result)
		if parseErr != nil {
			_ = cmd.Wait()
			stderr := stderrBuf.Bytes()
			return BackupResult{}, annotateJSONParseError("backup", line, stderr, parseErr)
		}

		if status != nil && onStatus != nil {
			onStatus(*status)
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		_ = cmd.Wait()
		return BackupResult{}, err
	}

	if err := cmd.Wait(); err != nil {
		result.Errors = parseOperationErrors(stderrBuf.Bytes())
		return result, buildCommandError(err, stderrBuf.String())
	}

	result.Errors = parseOperationErrors(stderrBuf.Bytes())
	return result, nil
}

func (r Runner) RunRestoreJSON(ctx context.Context, repo config.Repo, snapshot, target string) (RestoreResult, error) {
	return r.RunRestoreJSONWithOptions(ctx, repo, RestoreRunOptions{Snapshot: snapshot, Target: target})
}

func (r Runner) RunRestoreJSONWithOptions(ctx context.Context, repo config.Repo, options RestoreRunOptions) (RestoreResult, error) {
	args, err := BuildRestoreJSONArgsWithOptions(repo, r.passwordCommand(repo.ID), options)
	if err != nil {
		return RestoreResult{}, err
	}
	stdout, stderr, err := r.runCapture(ctx, args)
	if err != nil {
		return RestoreResult{}, err
	}
	return parseRestoreResult(stdout, stderr)
}

func (r Runner) RunRestoreJSONStream(ctx context.Context, repo config.Repo, options RestoreRunOptions, onStatus func(RestoreStatus)) (RestoreResult, error) {
	args, err := BuildRestoreJSONArgsWithOptions(repo, r.passwordCommand(repo.ID), options)
	if err != nil {
		return RestoreResult{}, err
	}

	cmd := exec.CommandContext(ctx, r.ResticPath, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return RestoreResult{}, err
	}

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return RestoreResult{}, buildCommandError(err, stderrBuf.String())
	}

	result := RestoreResult{}
	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		status, parseErr := parseRestoreJSONLine(line, &result)
		if parseErr != nil {
			_ = cmd.Wait()
			stderr := stderrBuf.Bytes()
			return RestoreResult{}, annotateJSONParseError("restore", line, stderr, parseErr)
		}

		if status != nil && onStatus != nil {
			onStatus(*status)
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		_ = cmd.Wait()
		return RestoreResult{}, err
	}

	if err := cmd.Wait(); err != nil {
		result.Errors = parseOperationErrors(stderrBuf.Bytes())
		return result, buildCommandError(err, stderrBuf.String())
	}

	result.Errors = parseOperationErrors(stderrBuf.Bytes())
	return result, nil
}

func BuildInitJSONArgs(repo config.Repo, passwordCommand string) []string {
	return []string{"-r", repo.Path, "--password-command", passwordCommand, "init", "--json"}
}

func BuildSnapshotsJSONArgs(repo config.Repo, passwordCommand string) []string {
	return []string{"-r", repo.Path, "--password-command", passwordCommand, "snapshots", "--json"}
}

func BuildStatsJSONArgs(repo config.Repo, passwordCommand string) []string {
	return []string{"-r", repo.Path, "--password-command", passwordCommand, "stats", "--json"}
}

func BuildCheckJSONArgs(repo config.Repo, passwordCommand string) []string {
	return []string{"-r", repo.Path, "--password-command", passwordCommand, "check", "--json"}
}

func BuildBackupJSONArgs(repo config.Repo, passwordCommand string) ([]string, error) {
	return BuildBackupJSONArgsWithOptions(repo, passwordCommand, BackupRunOptions{})
}

func BuildBackupJSONArgsWithOptions(repo config.Repo, passwordCommand string, options BackupRunOptions) ([]string, error) {
	if len(repo.Include) == 0 {
		return nil, errors.New("repo include list is empty")
	}

	args := []string{"-r", repo.Path, "--password-command", passwordCommand}
	if repo.Options.Verbose {
		args = append(args, "-v")
	}
	args = append(args, "backup", "--json")
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

func BuildRestoreJSONArgs(repo config.Repo, passwordCommand, snapshot, target string) ([]string, error) {
	return BuildRestoreJSONArgsWithOptions(repo, passwordCommand, RestoreRunOptions{Snapshot: snapshot, Target: target})
}

func BuildRestoreJSONArgsWithOptions(repo config.Repo, passwordCommand string, options RestoreRunOptions) ([]string, error) {
	snapshot := options.Snapshot
	target := options.Target
	if strings.TrimSpace(snapshot) == "" {
		return nil, errors.New("snapshot is required")
	}
	if strings.TrimSpace(target) == "" {
		return nil, errors.New("target is required")
	}
	args := []string{"-r", repo.Path, "--password-command", passwordCommand, "restore", snapshot, "--target", target, "--json"}
	if options.DryRun {
		args = append(args, "--dry-run")
	}
	return args, nil
}

func (r Runner) runCapture(ctx context.Context, args []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, r.ResticPath, args...)
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), buildCommandError(err, stderrBuf.String())
	}
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), nil
}

func buildCommandError(err error, stderr string) error {
	if errors.Is(err, exec.ErrNotFound) {
		return CommandError{Err: errors.New("restic executable not found in PATH"), Code: 127, Kind: ErrorKindExecutableMissing}
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		payload, ok := parseExitError(stderr)
		if ok {
			kind := classifyResticExit(payload.Code, payload.Message+"\n"+stderr)
			return CommandError{Err: errors.New(payload.Message), Code: payload.Code, Kind: kind}
		}
		kind := classifyResticExit(exitErr.ExitCode(), stderr)
		return CommandError{Err: err, Code: exitErr.ExitCode(), Kind: kind}
	}

	return CommandError{Err: err, Code: 1, Kind: ErrorKindUnknown}
}

func parseExitError(stderr string) (exitErrorPayload, bool) {
	for _, line := range splitJSONLines([]byte(stderr)) {
		var payload exitErrorPayload
		if err := json.Unmarshal(line, &payload); err != nil {
			continue
		}
		if payload.MessageType == "exit_error" {
			return payload, true
		}
	}
	return exitErrorPayload{}, false
}

func parseCheckResult(stdout, stderr []byte) (CheckResult, error) {
	result := CheckResult{}
	for _, line := range splitJSONLines(stdout) {
		var env struct {
			MessageType string `json:"message_type"`
		}
		if err := json.Unmarshal(line, &env); err != nil {
			return CheckResult{}, annotateJSONParseError("check", stdout, stderr, err)
		}
		switch env.MessageType {
		case "summary":
			var summary CheckSummary
			if err := json.Unmarshal(line, &summary); err != nil {
				return CheckResult{}, annotateJSONParseError("check", stdout, stderr, err)
			}
			result.Summary = &summary
		}
	}
	for _, line := range splitJSONLines(stderr) {
		var env struct {
			MessageType string `json:"message_type"`
		}
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}
		if env.MessageType == "error" {
			var item CheckError
			if err := json.Unmarshal(line, &item); err == nil {
				result.Errors = append(result.Errors, item)
			}
		}
	}
	return result, nil
}

func parseBackupResult(stdout, stderr []byte) (BackupResult, error) {
	result := BackupResult{}
	for _, line := range splitJSONLines(stdout) {
		if _, err := parseBackupJSONLine(line, &result); err != nil {
			return BackupResult{}, annotateJSONParseError("backup", stdout, stderr, err)
		}
	}
	result.Errors = parseOperationErrors(stderr)
	return result, nil
}

func parseBackupJSONLine(line []byte, result *BackupResult) (*BackupStatus, error) {
	var env struct {
		MessageType string `json:"message_type"`
	}
	if err := json.Unmarshal(line, &env); err != nil {
		return nil, err
	}

	switch env.MessageType {
	case "status":
		var item BackupStatus
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, err
		}
		result.Status = append(result.Status, item)
		return &item, nil
	case "verbose_status":
		var item BackupVerboseStatus
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, err
		}
		result.Verbose = append(result.Verbose, item)
	case "summary":
		var item BackupSummary
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, err
		}
		result.Summary = &item
	}

	return nil, nil
}

func parseRestoreResult(stdout, stderr []byte) (RestoreResult, error) {
	result := RestoreResult{}
	for _, line := range splitJSONLines(stdout) {
		if _, err := parseRestoreJSONLine(line, &result); err != nil {
			return RestoreResult{}, annotateJSONParseError("restore", stdout, stderr, err)
		}
	}
	result.Errors = parseOperationErrors(stderr)
	return result, nil
}

func parseRestoreJSONLine(line []byte, result *RestoreResult) (*RestoreStatus, error) {
	var env struct {
		MessageType string `json:"message_type"`
	}
	if err := json.Unmarshal(line, &env); err != nil {
		return nil, err
	}

	switch env.MessageType {
	case "status":
		var item RestoreStatus
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, err
		}
		result.Status = append(result.Status, item)
		return &item, nil
	case "verbose_status":
		var item RestoreVerboseStatus
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, err
		}
		result.Verbose = append(result.Verbose, item)
	case "summary":
		var item RestoreSummary
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, err
		}
		result.Summary = &item
	}

	return nil, nil
}

func parseOperationErrors(stderr []byte) []OperationError {
	var result []OperationError
	for _, line := range splitJSONLines(stderr) {
		var env struct {
			MessageType string `json:"message_type"`
		}
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}
		if env.MessageType != "error" {
			continue
		}
		var item OperationError
		if err := json.Unmarshal(line, &item); err == nil {
			result = append(result, item)
		}
	}
	return result
}

func decodeSingleJSONLine(data []byte, dest any) error {
	lines := splitJSONLines(data)
	if len(lines) != 1 {
		return fmt.Errorf("expected exactly one JSON line, got %d", len(lines))
	}
	return json.Unmarshal(lines[0], dest)
}

func splitJSONLines(data []byte) [][]byte {
	var lines [][]byte
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func annotateJSONParseError(command string, stdout, stderr []byte, err error) error {
	return fmt.Errorf("parse restic %s json: %w", command, err)
}
