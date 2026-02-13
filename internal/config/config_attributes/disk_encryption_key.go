package config_attributes

import (
	"errors"
	"fmt"
	"strings"
)

// DiskEncryptionKey defines a single disk encryption key mapping
type DiskEncryptionKey struct {
	Local  string `yaml:"local" validate:"required,filepath"`  // Local path to the key file
	Remote string `yaml:"remote" validate:"required,filepath"` // Remote path where key should be placed
}

// Validate ensures the disk encryption key configuration is valid
func (dek *DiskEncryptionKey) Validate() error {
	if dek.Local == "" {
		return errors.New("local path is empty")
	}
	if dek.Remote == "" {
		return errors.New("remote path is empty")
	}
	if !strings.HasPrefix(dek.Remote, "/") {
		return fmt.Errorf("disk encryption key: remote path must be absolute, got: %s", dek.Remote)
	}
	return nil
}

// ToSecretConfig converts DiskEncryptionKey to SecretConfig for reuse of transferSecret
func (dek *DiskEncryptionKey) ToSecretConfig() *Secret {
	localPath := dek.Local // Create a variable to take address
	return &Secret{
		Local: Local{
			Path: &localPath,
		},
		Remote: Remote{
			Path: dek.Remote,
			// UID/GID left as nil (will use defaults)
		},
	}
}
