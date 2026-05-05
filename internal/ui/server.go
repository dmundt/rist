package ui

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/dmundt/rist/internal/config"
	"gopkg.in/yaml.v3"
)

type Server struct {
	Addr string
}

type pageData struct {
	ConfigYAML string
	Output     string
	Error      string
}

func (s Server) Serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/config/save", s.handleConfigSave)
	mux.HandleFunc("/run", s.handleRun)
	mux.HandleFunc("/pw/set", s.handlePasswordSet)

	addr := strings.TrimSpace(s.Addr)
	if addr == "" {
		addr = "127.0.0.1:8787"
	}

	return http.ListenAndServe(addr, mux)
}

func (s Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadOrDefault()
	if err != nil {
		renderPage(w, pageData{Error: err.Error()})
		return
	}

	renderPage(w, pageData{ConfigYAML: config.MustMarshalYAML(cfg)})
}

func (s Server) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPage(w, pageData{Error: err.Error()})
		return
	}

	raw := r.FormValue("yaml")
	var cfg config.Config
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		renderPage(w, pageData{ConfigYAML: raw, Error: fmt.Sprintf("yaml parse failed: %v", err)})
		return
	}
	if err := config.Save(cfg); err != nil {
		renderPage(w, pageData{ConfigYAML: raw, Error: err.Error()})
		return
	}

	renderPage(w, pageData{ConfigYAML: config.MustMarshalYAML(cfg), Output: "config saved"})
}

func (s Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPage(w, pageData{Error: err.Error()})
		return
	}

	cmdText := strings.TrimSpace(r.FormValue("command"))
	if cmdText == "" {
		renderPage(w, pageData{Error: "command is required"})
		return
	}

	args := strings.Fields(cmdText)
	if len(args) == 0 {
		renderPage(w, pageData{Error: "invalid command"})
		return
	}
	if err := validateAllowedUICommand(args); err != nil {
		renderPage(w, pageData{Error: err.Error()})
		return
	}

	output, err := runRist(args, "")
	cfg, _ := config.LoadOrDefault()
	data := pageData{ConfigYAML: config.MustMarshalYAML(cfg), Output: output}
	if err != nil {
		data.Error = err.Error()
	}
	renderPage(w, data)
}

func (s Server) handlePasswordSet(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPage(w, pageData{Error: err.Error()})
		return
	}

	repoID := strings.TrimSpace(r.FormValue("repoID"))
	password := r.FormValue("password")
	if repoID == "" {
		renderPage(w, pageData{Error: "repo id is required"})
		return
	}

	output, err := runRist([]string{"pw", "set", repoID}, password)
	cfg, _ := config.LoadOrDefault()
	data := pageData{ConfigYAML: config.MustMarshalYAML(cfg), Output: output}
	if err != nil {
		data.Error = err.Error()
	}
	renderPage(w, data)
}

func runRist(args []string, stdin string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(exe, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err = cmd.Run()
	combined := strings.TrimSpace(strings.TrimSpace(out.String()) + "\n" + strings.TrimSpace(errOut.String()))
	if err != nil {
		return combined, fmt.Errorf("command failed: %w", err)
	}
	return combined, nil
}

func renderPage(w http.ResponseWriter, data pageData) {
	if strings.TrimSpace(data.ConfigYAML) == "" {
		cfg, _ := config.LoadOrDefault()
		data.ConfigYAML = config.MustMarshalYAML(cfg)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pageTemplate.Execute(w, data)
}

//go:embed page.html
var pageFS embed.FS

var pageTemplate = template.Must(template.ParseFS(pageFS, "page.html"))

func validateAllowedUICommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("command is required")
	}

	switch args[0] {
	case "config":
		if len(args) == 2 && (args[1] == "show" || args[1] == "validate") {
			return nil
		}
	case "repo":
		if len(args) == 2 && args[1] == "list" {
			return nil
		}
		if len(args) == 3 && args[1] == "init" {
			return nil
		}
	case "backup":
		if len(args) == 3 && args[1] == "run" {
			return nil
		}
	case "snapshots":
		if len(args) == 2 {
			return nil
		}
	case "maintenance":
		if len(args) == 3 && (args[1] == "check" || args[1] == "prune" || args[1] == "stats") {
			return nil
		}
	}

	return fmt.Errorf("command not allowed from UI; use one of: config show|validate, repo list|init <id>, backup run <id>, snapshots <id>, maintenance check|prune|stats <id>")
}
