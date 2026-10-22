package filepermissions

import "os"

const (
	DefaultFilePermissions os.FileMode = 0640
	PrivateFilePermissions os.FileMode = 0600
	DefaultDirPermissions  os.FileMode = 0750
)
