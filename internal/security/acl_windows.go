//go:build windows

package security

import (
	"fmt"
	"os/exec"
	"os/user"
	"strings"
)

func RestrictDirectoryToCurrentUser(dir string) error {
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("resolve current user: %w", err)
	}

	username := strings.TrimSpace(u.Username)
	if username == "" {
		return fmt.Errorf("current user name is empty")
	}

	cmd := exec.Command("icacls", dir, "/inheritance:r", "/grant:r", username+":(OI)(CI)F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("icacls failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}
