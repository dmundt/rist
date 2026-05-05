package credentials

type Store interface {
	SetSecret(target, secret string) error
	GetSecret(target string) (string, error)
	DeleteSecret(target string) error
}

var defaultStore Store = newDefaultStore()

func DefaultStore() Store {
	return defaultStore
}

func SetSecret(target, secret string) error {
	return defaultStore.SetSecret(target, secret)
}

func GetSecret(target string) (string, error) {
	return defaultStore.GetSecret(target)
}

func DeleteSecret(target string) error {
	return defaultStore.DeleteSecret(target)
}
