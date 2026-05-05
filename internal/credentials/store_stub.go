//go:build !windows && !linux && !darwin

package credentials

import "fmt"

type unsupportedStore struct{}

func newDefaultStore() Store {
	return unsupportedStore{}
}

func (unsupportedStore) SetSecret(_, _ string) error {
	return fmt.Errorf("%w: no secure credential backend is configured for this OS", ErrStoreUnavailable)
}

func (unsupportedStore) GetSecret(_ string) (string, error) {
	return "", fmt.Errorf("%w: no secure credential backend is configured for this OS", ErrStoreUnavailable)
}

func (unsupportedStore) DeleteSecret(_ string) error {
	return fmt.Errorf("%w: no secure credential backend is configured for this OS", ErrStoreUnavailable)
}
