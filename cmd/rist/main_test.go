package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MAIN_HELPER") != "1" {
		return
	}

	helperArgs := os.Args
	for index, arg := range os.Args {
		if arg == "--" {
			helperArgs = os.Args[index+1:]
			break
		}
	}

	os.Args = append([]string{"rist"}, helperArgs...)
	main()
	os.Exit(0)
}

func TestMainSuccessExitCodeZero(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	toolDir := t.TempDir()
	writeBatch(t, filepath.Join(toolDir, "icacls.cmd"), "@echo off\r\nexit /b 0\r\n")

	exitCode, _, _ := runMainSubprocess(t, []string{"config", "init"}, map[string]string{
		"APPDATA": appData,
		"PATH":    toolDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if exitCode != 0 {
		t.Fatalf("main exit code = %d, want 0", exitCode)
	}
}

func TestMainUnknownCommandExitCodeOne(t *testing.T) {
	exitCode, _, _ := runMainSubprocess(t, []string{"does-not-exist"}, nil)
	if exitCode != 1 {
		t.Fatalf("main exit code = %d, want 1", exitCode)
	}
}

func TestMainExitCodePropagation(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	if err := os.MkdirAll(filepath.Join(appData, "Rist"), 0o700); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	configYAML := "version: 1\nrepos:\n  - id: main\n    path: C:\\Backups\\restic\n    passwordTarget: rist-repo-main\n    include:\n      - C:\\Users\\me\\Documents\n"
	if err := os.WriteFile(filepath.Join(appData, "Rist", "config.yaml"), []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	toolDir := t.TempDir()
	writeBatch(t, filepath.Join(toolDir, "restic.cmd"), "@echo off\r\necho {\"message_type\":\"exit_error\",\"code\":12,\"message\":\"wrong password\"} 1>&2\r\nexit /b 12\r\n")

	exitCode, _, _ := runMainSubprocess(t, []string{"repo", "init", "main"}, map[string]string{
		"APPDATA": appData,
		"PATH":    toolDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if exitCode != 12 {
		t.Fatalf("main exit code = %d, want 12", exitCode)
	}
}

func runMainSubprocess(t *testing.T, args []string, env map[string]string) (int, string, string) {
	t.Helper()
	cmdArgs := append([]string{"-test.run=TestMainHelperProcess", "--"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_MAIN_HELPER=1")
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	stdout, err := cmd.Output()
	if err == nil {
		return 0, string(stdout), ""
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), string(stdout), string(exitErr.Stderr)
	}
	t.Fatalf("subprocess failed: %v", err)
	return -1, "", ""
}

func writeBatch(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write batch %s: %v", path, err)
	}
}

func TestMainHelperProcessArgsParsing(t *testing.T) {
	args := []string{"a", "--", "b", "c"}
	idx := -1
	for i, arg := range args {
		if arg == "--" {
			idx = i
			break
		}
	}
	if idx != 1 || !strings.EqualFold(strings.Join(args[idx+1:], " "), "b c") {
		t.Fatalf("unexpected args parse result: idx=%d args=%v", idx, args)
	}
}
