package credentials

import (
	"errors"
	"fmt"
	"os"
	"testing"
)

type mockStore struct {
	setTarget    string
	setSecret    string
	getTarget    string
	deleteTarget string
	getValue     string
	err          error
}

func (m *mockStore) SetSecret(target, secret string) error {
	m.setTarget = target
	m.setSecret = secret
	return m.err
}

func (m *mockStore) GetSecret(target string) (string, error) {
	m.getTarget = target
	return m.getValue, m.err
}

func (m *mockStore) DeleteSecret(target string) error {
	m.deleteTarget = target
	return m.err
}

func TestDefaultStoreWrappers(t *testing.T) {
	original := defaultStore
	t.Cleanup(func() { defaultStore = original })

	mock := &mockStore{getValue: "secret-value"}
	defaultStore = mock

	if DefaultStore() != mock {
		t.Fatal("DefaultStore did not return current default store")
	}
	if err := SetSecret("target-a", "secret-a"); err != nil {
		t.Fatalf("SetSecret returned error: %v", err)
	}
	if mock.setTarget != "target-a" || mock.setSecret != "secret-a" {
		t.Fatalf("unexpected set call: %+v", mock)
	}

	value, err := GetSecret("target-b")
	if err != nil {
		t.Fatalf("GetSecret returned error: %v", err)
	}
	if value != "secret-value" || mock.getTarget != "target-b" {
		t.Fatalf("unexpected get result: value=%q store=%+v", value, mock)
	}

	if err := DeleteSecret("target-c"); err != nil {
		t.Fatalf("DeleteSecret returned error: %v", err)
	}
	if mock.deleteTarget != "target-c" {
		t.Fatalf("unexpected delete call: %+v", mock)
	}
}

func TestDefaultStoreWrappersPropagateErrors(t *testing.T) {
	original := defaultStore
	t.Cleanup(func() { defaultStore = original })

	want := errors.New("boom")
	defaultStore = &mockStore{err: want}

	if err := SetSecret("target", "secret"); !errors.Is(err, want) {
		t.Fatalf("SetSecret error = %v, want %v", err, want)
	}
	if _, err := GetSecret("target"); !errors.Is(err, want) {
		t.Fatalf("GetSecret error = %v, want %v", err, want)
	}
	if err := DeleteSecret("target"); !errors.Is(err, want) {
		t.Fatalf("DeleteSecret error = %v, want %v", err, want)
	}
}

func TestWindowsStoreRoundTrip(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("skip native Windows credential manager round-trip in CI")
	}

	store := windowsStore{}
	target := fmt.Sprintf("rist-test-%d", os.Getpid())
	t.Cleanup(func() {
		_ = store.DeleteSecret(target)
	})

	if err := store.SetSecret(target, "secret-value"); err != nil {
		t.Fatalf("SetSecret returned error: %v", err)
	}
	value, err := store.GetSecret(target)
	if err != nil {
		t.Fatalf("GetSecret returned error: %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("GetSecret = %q, want %q", value, "secret-value")
	}
	if err := store.DeleteSecret(target); err != nil {
		t.Fatalf("DeleteSecret returned error: %v", err)
	}
	if _, err := store.GetSecret(target); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSecret after delete error = %v, want %v", err, ErrNotFound)
	}
	if err := store.DeleteSecret(target); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteSecret missing target error = %v, want %v", err, ErrNotFound)
	}
}
