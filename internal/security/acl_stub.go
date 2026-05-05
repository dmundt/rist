//go:build !windows

package security

import "errors"

func RestrictDirectoryToCurrentUser(_ string) error {
	return errors.New("ACL hardening is only supported on Windows")
}
