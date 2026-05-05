//go:build linux || darwin

package credentials

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	keyring "github.com/zalando/go-keyring"
)

const keyringServiceName = "rist"

type keyringStore struct{}

func newDefaultStore() Store {
	return keyringStore{}
}

func (keyringStore) SetSecret(target, secret string) error {
	if err := keyring.Set(keyringServiceName, target, secret); err != nil {
		return annotateBackendError("store credential", err)
	}
	return nil
}

func (keyringStore) GetSecret(target string) (string, error) {
	secret, err := keyring.Get(keyringServiceName, target)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", annotateBackendError("read credential", err)
	}
	return secret, nil
}

func (keyringStore) DeleteSecret(target string) error {
	err := keyring.Delete(keyringServiceName, target)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return ErrNotFound
		}
		return annotateBackendError("delete credential", err)
	}
	return nil
}

func annotateBackendError(op string, err error) error {
	msg := strings.ToLower(err.Error())
	if runtime.GOOS == "linux" {
		if strings.Contains(msg, "dbus") || strings.Contains(msg, "secret service") || strings.Contains(msg, "org.freedesktop.secrets") {
			return fmt.Errorf("%s: %w (hint: ensure a Secret Service/libsecret backend is running and the user session D-Bus is available)", op, err)
		}
	}
	if runtime.GOOS == "darwin" {
		if strings.Contains(msg, "user interaction is not allowed") || strings.Contains(msg, "interaction not allowed") || strings.Contains(msg, "keychain") {
			return fmt.Errorf("%s: %w (hint: unlock the login keychain and allow this app to access Keychain items)", op, err)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
