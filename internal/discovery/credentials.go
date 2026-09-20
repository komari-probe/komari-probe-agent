package discovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type autoDiscoveryCredentials struct {
	UUID  string `json:"uuid"`
	Token string `json:"token"`
}

type credentialStore interface {
	Load() (*autoDiscoveryCredentials, error)
	Save(*autoDiscoveryCredentials) error
}

type fileCredentialStore struct {
	path       string
	legacyPath string
}

func defaultCredentialStore() credentialStore {
	legacyPath := legacyCredentialPath()
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fileCredentialStore{path: legacyPath}
	}
	return fileCredentialStore{
		path:       filepath.Join(configDir, "komari-agent", "auto-discovery.json"),
		legacyPath: legacyPath,
	}
}

func legacyCredentialPath() string {
	execPath, err := os.Executable()
	if err != nil {
		return "auto-discovery.json"
	}
	return filepath.Join(filepath.Dir(execPath), "auto-discovery.json")
}

func (s fileCredentialStore) Load() (*autoDiscoveryCredentials, error) {
	credentials, err := loadCredentials(s.path)
	if err == nil || !errors.Is(err, os.ErrNotExist) || s.legacyPath == "" || s.legacyPath == s.path {
		return credentials, err
	}
	credentials, err = loadCredentials(s.legacyPath)
	if err != nil || credentials == nil {
		return credentials, err
	}
	if err := s.Save(credentials); err != nil {
		return nil, fmt.Errorf("migrate legacy credentials: %w", err)
	}
	return credentials, nil
}

func (s fileCredentialStore) Save(credentials *autoDiscoveryCredentials) error {
	if credentials == nil {
		return errors.New("credentials are required")
	}
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create credentials directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".auto-discovery-*")
	if err != nil {
		return fmt.Errorf("create temporary credentials file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect temporary credentials file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close credentials: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace credentials: %w", err)
	}
	return nil
}

func loadCredentials(path string) (*autoDiscoveryCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var credentials autoDiscoveryCredentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if credentials.Token == "" {
		return nil, errors.New("credentials contain an empty token")
	}
	return &credentials, nil
}
