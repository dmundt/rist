//go:build windows

package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestrictDirectoryToCurrentUser(t *testing.T) {
	dir := t.TempDir()
	toolDir := t.TempDir()
	cmdPath := filepath.Join(toolDir, "icacls.cmd")
	if err := os.WriteFile(cmdPath, []byte("@echo off\r\nexit /b 0\r\n"), 0o700); err != nil {
		t.Fatalf("write fake icacls: %v", err)
	}
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := RestrictDirectoryToCurrentUser(dir); err != nil {
		t.Fatalf("RestrictDirectoryToCurrentUser returned error: %v", err)
	}
}

func TestRestrictDirectoryToCurrentUserIcaclsFailure(t *testing.T) {
	dir := t.TempDir()
	toolDir := t.TempDir()
	cmdPath := filepath.Join(toolDir, "icacls.cmd")
	if err := os.WriteFile(cmdPath, []byte("@echo off\r\necho access denied\r\nexit /b 1\r\n"), 0o700); err != nil {
		t.Fatalf("write fake icacls: %v", err)
	}
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := RestrictDirectoryToCurrentUser(dir)
	if err == nil || !strings.Contains(err.Error(), "icacls failed") {
		t.Fatalf("expected icacls failure error, got %v", err)
	}
}
