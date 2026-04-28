package filepermissions

import "os"

const (
	DefaultFilePermissions os.FileMode = 0640
	DefaultDirPermissions  os.FileMode = 0750
)
