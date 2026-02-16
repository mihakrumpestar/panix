package config_attributes

type DiskEncryptionKey struct {
	Local  string `yaml:"local" validate:"required,filepath" desc:"Local path to the key file"`
	Remote string `yaml:"remote" validate:"required,abspath" desc:"Remote path where key should be placed"`
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
