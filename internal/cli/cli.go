package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/dmundt/bargo"
	"github.com/dmundt/rist/internal/config"
	"github.com/dmundt/rist/internal/credentials"
	"github.com/dmundt/rist/internal/restic"
	"github.com/dmundt/rist/internal/ui"
	"golang.org/x/term"
)

func Run(args []string, stdout, stderr io.Writer) (err error) {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	switch args[0] {
	case "config":
		return runConfig(args[1:], stdout)
	case "repo":
		return runRepo(args[1:], stdout, stderr)
	case "backup":
		return runBackupRun(args[1:], stdout, stderr)
	case "snapshots":
		return runSnapshots(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "prune":
		return runPrune(args[1:], stdout, stderr)
	case "stats":
		return runStats(args[1:], stdout, stderr)
	case "forget":
		return runForget(args[1:], stdout, stderr)
	case "restore":
		return runRestoreRun(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout)
	case "pw":
		return runPassword(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "rist cli")
	fmt.Fprintln(out, "usage:")
	fmt.Fprintln(out, "  rist config init")
	fmt.Fprintln(out, "  rist config validate")
	fmt.Fprintln(out, "  rist config migrate")
	fmt.Fprintln(out, "  rist config show")
	fmt.Fprintln(out, "  rist repo add --id <id> --path <path> [--password-target <target>]")
	fmt.Fprintln(out, "  rist repo remove <id>")
	fmt.Fprintln(out, "  rist repo list")
	fmt.Fprintln(out, "  rist repo init <id>")
	fmt.Fprintln(out, "  rist repo include add <id> <path>")
	fmt.Fprintln(out, "  rist repo include remove <id> <path>")
	fmt.Fprintln(out, "  rist repo include list <id>")
	fmt.Fprintln(out, "  rist repo exclude add <id> <path>")
	fmt.Fprintln(out, "  rist repo exclude remove <id> <path>")
	fmt.Fprintln(out, "  rist repo exclude list <id>")
	fmt.Fprintln(out, "  rist repo options show <id>")
	fmt.Fprintln(out, "  rist repo options set <id> [--one-file-system=true|false] [--verbose=true|false]")
	fmt.Fprintln(out, "  rist backup <id> [--dry-run]")
	fmt.Fprintln(out, "  rist snapshots <id>")
	fmt.Fprintln(out, "  rist check <id>")
	fmt.Fprintln(out, "  rist prune <id>")
	fmt.Fprintln(out, "  rist stats <id>")
	fmt.Fprintln(out, "  rist forget <id> [--keep-last n] [--keep-daily n] [--keep-weekly n] [--keep-monthly n] [--keep-yearly n] [--prune] [--dry-run]")
	fmt.Fprintln(out, "  rist restore <id> --snapshot <snapshot> --target <path> [--dry-run]")
	fmt.Fprintln(out, "  rist serve [--addr 127.0.0.1:8787]")
	fmt.Fprintln(out, "  rist pw <id>")
	fmt.Fprintln(out, "  rist pw set <id>  (password from stdin)")
	fmt.Fprintln(out, "  rist pw clear <id>")
}

func runConfig(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing config subcommand")
	}

	switch args[0] {
	case "init":
		cfg := config.DefaultConfig()
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Fprintln(out, "config initialized")
		return nil
	case "validate":
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := config.Validate(cfg); err != nil {
			return err
		}
		fmt.Fprintln(out, "config is valid")
		return nil
	case "migrate":
		cfg, err := config.LoadRaw()
		if err != nil {
			return err
		}
		migrated, changed, err := config.MigrateToCurrent(cfg)
		if err != nil {
			return err
		}
		if err := config.Save(migrated); err != nil {
			return err
		}
		if changed {
			fmt.Fprintf(out, "config migrated to version %d\n", config.CurrentVersion())
		} else {
			fmt.Fprintf(out, "config already at version %d\n", config.CurrentVersion())
		}
		return nil
	case "show":
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Fprintln(out, config.MustMarshalYAML(cfg))
		return nil
	default:
		return fmt.Errorf("unknown config subcommand: %s", args[0])
	}
}

func runRepo(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing repo subcommand")
	}

	switch args[0] {
	case "add":
		return runRepoAdd(args[1:], out)
	case "remove":
		return runRepoRemove(args[1:], out)
	case "list":
		return runRepoList(out)
	case "init":
		return runRepoInit(args[1:], out, errOut)
	case "include":
		return runRepoPathListCommand(args[1:], out, "include")
	case "exclude":
		return runRepoPathListCommand(args[1:], out, "exclude")
	case "options":
		return runRepoOptions(args[1:], out)
	default:
		return fmt.Errorf("unknown repo subcommand: %s", args[0])
	}
}

func runRepoPathListCommand(args []string, out io.Writer, listType string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing repo %s subcommand", listType)
	}

	switch args[0] {
	case "add":
		if len(args) != 3 {
			return fmt.Errorf("usage: rist repo %s add <id> <path>", listType)
		}
		return addRepoPath(args[1], args[2], listType, out)
	case "remove":
		if len(args) != 3 {
			return fmt.Errorf("usage: rist repo %s remove <id> <path>", listType)
		}
		return removeRepoPath(args[1], args[2], listType, out)
	case "list":
		if len(args) != 2 {
			return fmt.Errorf("usage: rist repo %s list <id>", listType)
		}
		return listRepoPaths(args[1], listType, out)
	default:
		return fmt.Errorf("unknown repo %s subcommand: %s", listType, args[0])
	}
}

func runSnapshots(args []string, out, errOut io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: rist snapshots <id>")
	}

	repo, err := loadRepoByID(args[0])
	if err != nil {
		return err
	}

	runner, err := restic.NewDefaultRunner()
	if err != nil {
		return err
	}
	return runner.RunSnapshots(context.Background(), repo, out, errOut)
}

func runServe(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	addr := fs.String("addr", "127.0.0.1:8787", "listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Fprintf(out, "serving on http://%s\n", *addr)
	return ui.Server{Addr: *addr}.Serve()
}

func runRepoAdd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("repo add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	id := fs.String("id", "", "repository id")
	repoPath := fs.String("path", "", "repository path")
	passwordTarget := fs.String("password-target", "", "credential manager target")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("--id is required")
	}
	if *repoPath == "" {
		return errors.New("--path is required")
	}

	cfg, err := config.LoadOrDefault()
	if err != nil {
		return err
	}

	if config.RepoExists(cfg, *id) {
		return fmt.Errorf("repo already exists: %s", *id)
	}

	target := *passwordTarget
	if target == "" {
		target = config.DefaultPasswordTarget(*id)
	}

	cfg.Repos = append(cfg.Repos, config.Repo{
		ID:             *id,
		Path:           *repoPath,
		PasswordTarget: target,
		Include:        []string{},
		Exclude:        []string{},
		Options: config.Options{
			OneFileSystem: false,
			Verbose:       false,
		},
	})

	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Fprintf(out, "repo added: %s\n", *id)
	return nil
}

func runRepoRemove(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: rist repo remove <id>")
	}

	repoID := args[0]
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	updated := make([]config.Repo, 0, len(cfg.Repos))
	removed := false
	for _, repo := range cfg.Repos {
		if repo.ID == repoID {
			removed = true
			continue
		}
		updated = append(updated, repo)
	}

	if !removed {
		return fmt.Errorf("repo not found: %s", repoID)
	}

	cfg.Repos = updated
	if err := config.Save(cfg); err != nil {
		return err
	}

	fmt.Fprintf(out, "repo removed: %s\n", repoID)
	return nil
}

func runRepoList(out io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	repos := make([]config.Repo, len(cfg.Repos))
	copy(repos, cfg.Repos)
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].ID < repos[j].ID
	})

	if len(repos) == 0 {
		fmt.Fprintln(out, "no repos configured")
		return nil
	}

	for _, repo := range repos {
		fmt.Fprintf(out, "%s\t%s\t%s\n", repo.ID, repo.Path, repo.PasswordTarget)
	}
	return nil
}

func runRepoInit(args []string, out, errOut io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: rist repo init <id>")
	}

	repo, err := loadRepoByID(args[0])
	if err != nil {
		return err
	}

	runner, err := restic.NewDefaultRunner()
	if err != nil {
		return err
	}

	if err := runner.RunInit(context.Background(), repo, out, errOut); err != nil {
		return err
	}

	fmt.Fprintf(out, "repo initialized: %s\n", args[0])
	return nil
}

func runRepoOptions(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing repo options subcommand")
	}

	switch args[0] {
	case "show":
		if len(args) != 2 {
			return errors.New("usage: rist repo options show <id>")
		}
		repo, err := loadRepoByID(args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "oneFileSystem=%t\n", repo.Options.OneFileSystem)
		fmt.Fprintf(out, "verbose=%t\n", repo.Options.Verbose)
		return nil
	case "set":
		return runRepoOptionsSet(args[1:], out)
	default:
		return fmt.Errorf("unknown repo options subcommand: %s", args[0])
	}
}

func runRepoOptionsSet(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: rist repo options set <id> [--one-file-system=true|false] [--verbose=true|false]")
	}
	repoID := args[0]

	fs := flag.NewFlagSet("repo options set", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	oneFileSystem := fs.String("one-file-system", "", "true or false")
	verbose := fs.String("verbose", "", "true or false")

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: rist repo options set <id> [--one-file-system=true|false] [--verbose=true|false]")
	}
	if *oneFileSystem == "" && *verbose == "" {
		return errors.New("at least one option flag must be provided")
	}

	return updateRepo(repoID, func(repo *config.Repo) error {
		if *oneFileSystem != "" {
			v, err := strconv.ParseBool(*oneFileSystem)
			if err != nil {
				return errors.New("--one-file-system must be true or false")
			}
			repo.Options.OneFileSystem = v
		}
		if *verbose != "" {
			v, err := strconv.ParseBool(*verbose)
			if err != nil {
				return errors.New("--verbose must be true or false")
			}
			repo.Options.Verbose = v
		}
		return nil
	}, func() {
		fmt.Fprintf(out, "repo options updated: %s\n", repoID)
	})
}

func addRepoPath(repoID, path, listType string, out io.Writer) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	for i := range cfg.Repos {
		if cfg.Repos[i].ID != repoID {
			continue
		}

		items := getRepoPathList(cfg.Repos[i], listType)
		if containsString(items, path) {
			fmt.Fprintf(out, "%s already exists for repo: %s\n", listType, repoID)
			return nil
		}

		items = append(items, path)
		setRepoPathList(&cfg.Repos[i], listType, items)

		if err := config.Save(cfg); err != nil {
			return err
		}

		fmt.Fprintf(out, "%s added for repo: %s\n", listType, repoID)
		return nil
	}

	return fmt.Errorf("repo not found: %s", repoID)
}

func removeRepoPath(repoID, path, listType string, out io.Writer) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	for i := range cfg.Repos {
		if cfg.Repos[i].ID != repoID {
			continue
		}

		items := getRepoPathList(cfg.Repos[i], listType)
		updated, removed := withoutString(items, path)
		if !removed {
			fmt.Fprintf(out, "%s not present for repo: %s\n", listType, repoID)
			return nil
		}

		setRepoPathList(&cfg.Repos[i], listType, updated)

		if err := config.Save(cfg); err != nil {
			return err
		}

		fmt.Fprintf(out, "%s removed for repo: %s\n", listType, repoID)
		return nil
	}

	return fmt.Errorf("repo not found: %s", repoID)
}

func listRepoPaths(repoID, listType string, out io.Writer) error {
	repo, err := loadRepoByID(repoID)
	if err != nil {
		return err
	}

	items := getRepoPathList(repo, listType)
	if len(items) == 0 {
		fmt.Fprintf(out, "no %s paths for repo: %s\n", listType, repoID)
		return nil
	}

	for _, item := range items {
		fmt.Fprintln(out, item)
	}

	return nil
}

func runBackupRun(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: rist backup <id> [--dry-run]")
	}
	repoID := args[0]

	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dryRun := fs.Bool("dry-run", false, "show what would be backed up without writing a snapshot")

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: rist backup <id> [--dry-run]")
	}

	repo, err := loadRepoByID(repoID)
	if err != nil {
		return err
	}

	runner, err := restic.NewDefaultRunner()
	if err != nil {
		return err
	}
	options := restic.BackupRunOptions{DryRun: *dryRun}

	if shouldRenderProgress(out) && !*dryRun {
		progress := newBackupProgressBar(out)
		if err := runner.RunBackupWithProgress(context.Background(), repo, options, progress.Update, out, errOut); err != nil {
			return err
		}
	} else {
		if err := runner.RunBackupWithOptions(context.Background(), repo, options, out, errOut); err != nil {
			return err
		}
	}

	return nil
}

func runCheck(args []string, out, errOut io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: rist check <id>")
	}
	repo, err := loadRepoByID(args[0])
	if err != nil {
		return err
	}
	runner, err := restic.NewDefaultRunner()
	if err != nil {
		return err
	}
	return runner.RunCheck(context.Background(), repo, out, errOut)
}

func runPrune(args []string, out, errOut io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: rist prune <id>")
	}
	repo, err := loadRepoByID(args[0])
	if err != nil {
		return err
	}
	runner, err := restic.NewDefaultRunner()
	if err != nil {
		return err
	}
	return runner.RunPrune(context.Background(), repo, out, errOut)
}

func runStats(args []string, out, errOut io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: rist stats <id>")
	}
	repo, err := loadRepoByID(args[0])
	if err != nil {
		return err
	}
	runner, err := restic.NewDefaultRunner()
	if err != nil {
		return err
	}
	return runner.RunStats(context.Background(), repo, out, errOut)
}

func runForget(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: rist forget <id> [--keep-last N] [--keep-daily N] [--keep-weekly N] [--keep-monthly N] [--keep-yearly N] [--prune] [--dry-run]")
	}
	repoID := args[0]

	fs := flag.NewFlagSet("forget", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	keepLast := fs.Int("keep-last", 0, "keep last snapshots")
	keepDaily := fs.Int("keep-daily", 0, "keep daily snapshots")
	keepWeekly := fs.Int("keep-weekly", 0, "keep weekly snapshots")
	keepMonthly := fs.Int("keep-monthly", 0, "keep monthly snapshots")
	keepYearly := fs.Int("keep-yearly", 0, "keep yearly snapshots")
	prune := fs.Bool("prune", false, "run prune after forget")
	dryRun := fs.Bool("dry-run", false, "show what would be forgotten without deleting snapshots")

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: rist forget <id> [--keep-last N] [--keep-daily N] [--keep-weekly N] [--keep-monthly N] [--keep-yearly N] [--prune] [--dry-run]")
	}

	repo, err := loadRepoByID(repoID)
	if err != nil {
		return err
	}

	runner, err := restic.NewDefaultRunner()
	if err != nil {
		return err
	}

	policy := restic.ForgetPolicy{
		KeepLast:    *keepLast,
		KeepDaily:   *keepDaily,
		KeepWeekly:  *keepWeekly,
		KeepMonthly: *keepMonthly,
		KeepYearly:  *keepYearly,
		Prune:       *prune,
		DryRun:      *dryRun,
	}

	return runner.RunForget(context.Background(), repo, policy, out, errOut)
}

func runRestoreRun(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: rist restore <id> --snapshot <snapshot> --target <path> [--dry-run]")
	}
	repoID := args[0]

	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	snapshot := fs.String("snapshot", "", "snapshot id, tag, or latest")
	target := fs.String("target", "", "restore destination path")
	dryRun := fs.Bool("dry-run", false, "show what would be restored without writing files")

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: rist restore <id> --snapshot <snapshot> --target <path> [--dry-run]")
	}

	repo, err := loadRepoByID(repoID)
	if err != nil {
		return err
	}

	runner, err := restic.NewDefaultRunner()
	if err != nil {
		return err
	}
	options := restic.RestoreRunOptions{Snapshot: *snapshot, Target: *target, DryRun: *dryRun}

	if shouldRenderProgress(out) && !*dryRun {
		progress := newRestoreProgressBar(out)
		err = runner.RunRestoreWithProgress(context.Background(), repo, options, progress.Update, out, errOut)
		if err != nil {
			return err
		}
	} else {
		err = runner.RunRestoreWithOptions(context.Background(), repo, options, out, errOut)
		if err != nil {
			return err
		}
	}

	if *dryRun {
		fmt.Fprintf(out, "restore dry-run completed: %s\n", repoID)
		return nil
	}

	fmt.Fprintf(out, "restore completed: %s\n", repoID)
	return nil
}

func runPassword(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing repo id or pw subcommand")
	}

	switch args[0] {
	case "set":
		return runPasswordSet(args[1:], out)
	case "clear":
		return runPasswordClear(args[1:], out)
	default:
		return runPasswordGet(args[0], out)
	}
}

func runPasswordGet(repoID string, out io.Writer) error {
	target, err := resolvePasswordTarget(repoID)
	if err != nil {
		return err
	}

	password, err := credentials.GetSecret(target)
	if err != nil {
		if errors.Is(err, credentials.ErrNotFound) {
			return fmt.Errorf("password not found for repo: %s", repoID)
		}
		if errors.Is(err, credentials.ErrStoreUnavailable) {
			return fmt.Errorf("credential backend unavailable: %w", err)
		}
		return err
	}

	_, err = io.WriteString(out, password)
	return err
}

func runPasswordSet(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: rist pw set <id>")
	}

	repoID := args[0]
	target, err := resolvePasswordTarget(repoID)
	if err != nil {
		return err
	}

	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read password from stdin: %w", err)
	}
	password := strings.TrimRight(string(b), "\r\n")
	if strings.TrimSpace(password) == "" {
		return errors.New("password from stdin is empty")
	}

	if err := credentials.SetSecret(target, password); err != nil {
		if errors.Is(err, credentials.ErrStoreUnavailable) {
			return fmt.Errorf("credential backend unavailable: %w", err)
		}
		return err
	}

	fmt.Fprintf(out, "password stored for repo: %s\n", repoID)
	return nil
}

func runPasswordClear(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: rist pw clear <id>")
	}

	repoID := args[0]
	target, err := resolvePasswordTarget(repoID)
	if err != nil {
		return err
	}

	err = credentials.DeleteSecret(target)
	if errors.Is(err, credentials.ErrStoreUnavailable) {
		return fmt.Errorf("credential backend unavailable: %w", err)
	}
	if err != nil && !errors.Is(err, credentials.ErrNotFound) {
		return err
	}

	fmt.Fprintf(out, "password cleared for repo: %s\n", repoID)
	return nil
}

func resolvePasswordTarget(repoID string) (string, error) {
	repo, err := loadRepoByID(repoID)
	if err != nil {
		return "", err
	}

	return repo.PasswordTarget, nil
}

func loadRepoByID(repoID string) (config.Repo, error) {
	if strings.TrimSpace(repoID) == "" {
		return config.Repo{}, errors.New("repo id is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return config.Repo{}, err
	}

	repo, ok := config.FindRepoByID(cfg, repoID)
	if !ok {
		return config.Repo{}, fmt.Errorf("repo not found: %s", repoID)
	}

	return repo, nil
}

func getRepoPathList(repo config.Repo, listType string) []string {
	if listType == "exclude" {
		return append([]string(nil), repo.Exclude...)
	}
	return append([]string(nil), repo.Include...)
}

func setRepoPathList(repo *config.Repo, listType string, items []string) {
	if listType == "exclude" {
		repo.Exclude = items
		return
	}
	repo.Include = items
}

func containsString(items []string, v string) bool {
	for _, item := range items {
		if item == v {
			return true
		}
	}
	return false
}

func withoutString(items []string, v string) ([]string, bool) {
	result := make([]string, 0, len(items))
	removed := false
	for _, item := range items {
		if item == v {
			removed = true
			continue
		}
		result = append(result, item)
	}
	return result, removed
}

func updateRepo(repoID string, mutate func(*config.Repo) error, onSuccess func()) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	for i := range cfg.Repos {
		if cfg.Repos[i].ID != repoID {
			continue
		}

		if err := mutate(&cfg.Repos[i]); err != nil {
			return err
		}

		if err := config.Save(cfg); err != nil {
			return err
		}

		if onSuccess != nil {
			onSuccess()
		}
		return nil
	}

	return fmt.Errorf("repo not found: %s", repoID)
}

type backupProgressBar struct {
	out        io.Writer
	bar        *bargo.Bar
	filesDone  uint64
	totalFiles uint64
}

func newBackupProgressBar(out io.Writer) *backupProgressBar {
	if out == nil {
		out = io.Discard
	}

	return &backupProgressBar{out: out}
}

func (p *backupProgressBar) Update(status restic.BackupStatus) {
	if p == nil {
		return
	}
	if p.bar == nil {
		p.bar = bargo.New(
			bargo.WithCarriageReturn(true),
			bargo.WithClamp(true),
			bargo.WithHeadRune('>'),
		)
	}

	if status.TotalFiles > 0 {
		p.totalFiles = status.TotalFiles
	}
	if status.FilesDone > 0 || status.PercentDone >= 1 {
		p.filesDone = status.FilesDone
	}
	if status.PercentDone >= 1 && p.totalFiles > 0 {
		p.filesDone = p.totalFiles
	}

	suffix := fmt.Sprintf("%d/%d files", p.filesDone, p.totalFiles)
	_, _ = p.bar.WriteToWithText(p.out, status.PercentDone*100, 24, suffix)
}

func (p *backupProgressBar) Finish() {
	if p == nil || p.bar == nil {
		return
	}
	_, _ = io.WriteString(p.out, "\n")
}

type restoreProgressBar struct {
	out           io.Writer
	bar           *bargo.Bar
	filesRestored uint64
	totalFiles    uint64
}

func newRestoreProgressBar(out io.Writer) *restoreProgressBar {
	if out == nil {
		out = io.Discard
	}

	return &restoreProgressBar{out: out}
}

func (p *restoreProgressBar) Update(status restic.RestoreStatus) {
	if p == nil {
		return
	}
	if p.bar == nil {
		p.bar = bargo.New(
			bargo.WithCarriageReturn(true),
			bargo.WithClamp(true),
			bargo.WithHeadRune('>'),
		)
	}

	if status.TotalFiles > 0 {
		p.totalFiles = status.TotalFiles
	}
	if status.FilesRestored > 0 || status.PercentDone >= 1 {
		p.filesRestored = status.FilesRestored
	}
	if status.PercentDone >= 1 && p.totalFiles > 0 {
		p.filesRestored = p.totalFiles
	}

	suffix := fmt.Sprintf("%d/%d files", p.filesRestored, p.totalFiles)
	_, _ = p.bar.WriteToWithText(p.out, status.PercentDone*100, 24, suffix)
}

func (p *restoreProgressBar) Finish() {
	if p == nil || p.bar == nil {
		return
	}
	_, _ = io.WriteString(p.out, "\n")
}

func shouldRenderProgress(out io.Writer) bool {
	if out == nil {
		return false
	}

	type fdWriter interface {
		Fd() uintptr
	}

	fdOut, ok := out.(fdWriter)
	if !ok {
		return false
	}

	return term.IsTerminal(int(fdOut.Fd()))
}
