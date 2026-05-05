//go:build windows

package credentials

import (
	"errors"
	"fmt"

	"github.com/danieljoos/wincred"
)

type windowsStore struct{}

func newDefaultStore() Store {
	return windowsStore{}
}

func (windowsStore) SetSecret(target, secret string) error {
	cred := wincred.NewGenericCredential(target)
	cred.UserName = "rist"
	cred.CredentialBlob = []byte(secret)
	cred.Persist = wincred.PersistLocalMachine

	if err := cred.Write(); err != nil {
		return fmt.Errorf("store credential: %w", err)
	}
	return nil
}

func (windowsStore) GetSecret(target string) (string, error) {
	cred, err := wincred.GetGenericCredential(target)
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("read credential: %w", err)
	}
	return string(cred.CredentialBlob), nil
}

func (windowsStore) DeleteSecret(target string) error {
	cred, err := wincred.GetGenericCredential(target)
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("read credential before delete: %w", err)
	}
	if err := cred.Delete(); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	return nil
}
