package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileCredentialStoreSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "auto-discovery.json")
	store := fileCredentialStore{path: path}
	want := &autoDiscoveryCredentials{UUID: "agent-1", Token: "token-1"}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if *got != *want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestFileCredentialStoreMigratesLegacyCredentials(t *testing.T) {
	directory := t.TempDir()
	legacyPath := filepath.Join(directory, "legacy.json")
	newPath := filepath.Join(directory, "config", "auto-discovery.json")
	legacy := fileCredentialStore{path: legacyPath}
	if err := legacy.Save(&autoDiscoveryCredentials{UUID: "legacy-agent", Token: "legacy-token"}); err != nil {
		t.Fatal(err)
	}

	store := fileCredentialStore{path: newPath, legacyPath: legacyPath}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Token != "legacy-token" {
		t.Fatalf("migrated token = %q, want legacy-token", got.Token)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy credentials were not preserved: %v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new credentials were not written: %v", err)
	}
}

func TestLoadCredentialsRejectsEmptyToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auto-discovery.json")
	if err := os.WriteFile(path, []byte(`{"uuid":"agent-1","token":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCredentials(path); err == nil {
		t.Fatal("loadCredentials() accepted an empty token")
	}
}

func TestFileCredentialStoreRejectsUnwritableParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := fileCredentialStore{path: filepath.Join(parent, "credentials.json")}
	if err := store.Save(&autoDiscoveryCredentials{Token: "token"}); err == nil {
		t.Fatal("Save() succeeded with a file as its parent directory")
	}
}
