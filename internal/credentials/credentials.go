// Package credentials stores the API key the CLI signed in with.
//
// The key lives in a file only its owner can read (0600, in a 0700
// directory). EINVOICING_API_KEY overrides it, for CI and agents, where
// nothing should be written to disk at all.
package credentials

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Credentials struct {
	APIURL string `json:"api_url,omitempty"`
	Key    string `json:"key,omitempty"`
	Email  string `json:"email,omitempty"`
}

// Dir is where the credentials file lives. EINVOICING_CONFIG_DIR overrides
// the platform default (~/.config/einvoicing on Linux).
func Dir() (string, error) {
	if dir := os.Getenv("EINVOICING_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "einvoicing"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// Load returns empty credentials, not an error, when none are saved.
func Load() (Credentials, error) {
	path, err := Path()
	if err != nil {
		return Credentials{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Credentials{}, nil
	}
	if err != nil {
		return Credentials{}, err
	}
	var creds Credentials
	if err := json.Unmarshal(raw, &creds); err != nil {
		return Credentials{}, err
	}
	return creds, nil
}

// Save writes atomically: a temporary file in the same directory, then a
// rename, so a crash never leaves half a key on disk.
func Save(creds Credentials) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".credentials-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return path, os.Rename(tmp.Name(), path)
}

func Delete() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
