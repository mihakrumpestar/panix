package filepermissions

import "os"

const (
	DefaultFilePermissions os.FileMode = 0644
	DefaultDirPermissions  os.FileMode = 0755
)
