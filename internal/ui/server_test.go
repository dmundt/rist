package ui

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dmundt/rist/internal/config"
)

func init() {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "config" && args[1] == "show" {
		_, _ = os.Stdout.WriteString("version: 1\nrepos: []\n")
		os.Exit(0)
	}
	if len(args) >= 3 && args[0] == "pw" && args[1] == "set" {
		_, _ = os.Stdout.WriteString("password stored for repo: " + args[2] + "\n")
		os.Exit(0)
	}
	if len(args) >= 2 && args[0] == "backup" {
		_, _ = os.Stdout.WriteString("Backup Completed\n- Repo: main\n")
		os.Exit(0)
	}
	_, _ = os.Stderr.WriteString("helper failure")
	os.Exit(1)
}

func TestValidateAllowedUICommand(t *testing.T) {
	allowed := [][]string{
		{"config", "show"},
		{"config", "validate"},
		{"repo", "list"},
		{"repo", "init", "main"},
		{"backup", "main"},
		{"snapshots", "main"},
		{"check", "main"},
		{"prune", "main"},
		{"stats", "main"},
	}
	for _, args := range allowed {
		if err := validateAllowedUICommand(args); err != nil {
			t.Fatalf("expected allowed command %v, got error %v", args, err)
		}
	}
}

func TestValidateAllowedUICommandRejectsOtherCommands(t *testing.T) {
	rejected := [][]string{
		{"pw", "main"},
		{"pw", "set", "main"},
		{"repo", "remove", "main"},
		{"forget", "main", "--keep-daily", "7"},
		{"restore", "run", "main", "--snapshot", "latest", "--target", "C:\\restore"},
	}
	for _, args := range rejected {
		if err := validateAllowedUICommand(args); err == nil {
			t.Fatalf("expected command %v to be rejected", args)
		}
	}
}

func TestRunRist(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	output, err := runRist([]string{"config", "show"}, "")
	if err != nil {
		t.Fatalf("runRist returned error: %v", err)
	}
	if !strings.Contains(output, "version: 1") {
		t.Fatalf("unexpected runRist output: %q", output)
	}
}

func TestServeInvalidAddress(t *testing.T) {
	err := (Server{Addr: " 127.0.0.1:-1 "}).Serve()
	if err == nil {
		t.Fatal("expected Serve to fail for invalid address")
	}
}

func TestRenderPage(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)
	rec := httptest.NewRecorder()
	renderPage(rec, pageData{Output: "done"})
	if rec.Code != 200 {
		t.Fatalf("renderPage status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "done") {
		t.Fatalf("renderPage body missing output: %q", rec.Body.String())
	}
}

func TestHandleIndex(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	Server{}.handleIndex(rec, req)
	if rec.Code != 200 {
		t.Fatalf("handleIndex status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "version:") {
		t.Fatalf("handleIndex body missing config yaml: %q", rec.Body.String())
	}
}

func TestHandleIndexConfigError(t *testing.T) {
	t.Setenv("APPDATA", "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	Server{}.handleIndex(rec, req)
	if rec.Code != 200 {
		t.Fatalf("handleIndex status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "APPDATA is not set") {
		t.Fatalf("handleIndex body missing APPDATA error: %q", rec.Body.String())
	}
}

func TestHandleConfigSave(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)
	prependFakeIcaclsToPath(t)

	form := url.Values{"yaml": {config.MustMarshalYAML(config.DefaultConfig())}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/config/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	Server{}.handleConfigSave(rec, req)
	if rec.Code != 200 {
		t.Fatalf("handleConfigSave status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "config saved") {
		t.Fatalf("handleConfigSave body missing success message: %q", rec.Body.String())
	}
}

func TestHandleConfigSaveInvalidYAML(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)

	form := url.Values{"yaml": {": not yaml"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/config/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	Server{}.handleConfigSave(rec, req)
	if !strings.Contains(rec.Body.String(), "yaml parse failed") {
		t.Fatalf("unexpected handleConfigSave parse failure response: %q", rec.Body.String())
	}
}

func TestHandleConfigSaveParseAndSaveErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/config/save", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Server{}.handleConfigSave(rec, req)
	if !strings.Contains(rec.Body.String(), "invalid URL escape") {
		t.Fatalf("expected ParseForm error response, got %q", rec.Body.String())
	}

	t.Setenv("APPDATA", "")
	form := url.Values{"yaml": {config.MustMarshalYAML(config.DefaultConfig())}}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/config/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Server{}.handleConfigSave(rec, req)
	if !strings.Contains(rec.Body.String(), "APPDATA is not set") {
		t.Fatalf("expected Save failure response, got %q", rec.Body.String())
	}
}

func TestHandleRunValidationErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/run", strings.NewReader(url.Values{"command": {""}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	Server{}.handleRun(rec, req)
	if !strings.Contains(rec.Body.String(), "command is required") {
		t.Fatalf("unexpected handleRun empty-command response: %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/run", strings.NewReader(url.Values{"command": {"pw set main"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Server{}.handleRun(rec, req)
	if !strings.Contains(rec.Body.String(), "command not allowed from UI") {
		t.Fatalf("unexpected handleRun rejection response: %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/run", strings.NewReader(url.Values{"command": {"   "}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Server{}.handleRun(rec, req)
	if !strings.Contains(rec.Body.String(), "command is required") {
		t.Fatalf("unexpected handleRun blank response: %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/run", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Server{}.handleRun(rec, req)
	if !strings.Contains(rec.Body.String(), "invalid URL escape") {
		t.Fatalf("expected ParseForm error response, got %q", rec.Body.String())
	}
}

func TestHandleRunAndPasswordSet(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("APPDATA", appData)
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/run", strings.NewReader(url.Values{"command": {"backup main"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Server{}.handleRun(rec, req)
	if !strings.Contains(rec.Body.String(), "Backup Completed") {
		t.Fatalf("unexpected handleRun success response: %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/pw/set", strings.NewReader(url.Values{"repoID": {"main"}, "password": {"secret"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Server{}.handlePasswordSet(rec, req)
	if !strings.Contains(rec.Body.String(), "password stored for repo: main") {
		t.Fatalf("unexpected handlePasswordSet response: %q", rec.Body.String())
	}
}

func TestHandleRunExecutionErrorAndPasswordSetParseError(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/run", strings.NewReader(url.Values{"command": {"check main"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Server{}.handleRun(rec, req)
	if !strings.Contains(rec.Body.String(), "helper failure") {
		t.Fatalf("expected runRist error output, got %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/pw/set", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Server{}.handlePasswordSet(rec, req)
	if !strings.Contains(rec.Body.String(), "invalid URL escape") {
		t.Fatalf("expected ParseForm error response, got %q", rec.Body.String())
	}
}

func TestHandlePasswordSetValidationError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/pw/set", strings.NewReader(url.Values{"repoID": {""}, "password": {"secret"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	Server{}.handlePasswordSet(rec, req)
	if !strings.Contains(rec.Body.String(), "repo id is required") {
		t.Fatalf("unexpected handlePasswordSet validation response: %q", rec.Body.String())
	}
}

func TestValidateAllowedUICommandErrors(t *testing.T) {
	if err := validateAllowedUICommand(nil); err == nil {
		t.Fatal("expected empty command to be rejected")
	}
	if err := validateAllowedUICommand([]string{"forget", "main"}); err == nil {
		t.Fatal("expected forget to be rejected")
	}
}

func prependFakeIcaclsToPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "icacls.cmd"), []byte("@echo off\r\nexit /b 0\r\n"), 0o700); err != nil {
		t.Fatalf("write fake icacls: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
