package attributes

import (
	"os"
	"reflect"
	"slices"
	"strconv"

	"github.com/goccy/go-yaml/ast"
	"github.com/pkg/errors"

	"github.com/mihakrumpestar/panix/pkg/filemode"
	"github.com/mihakrumpestar/panix/pkg/ssh"
)

// structDefault returns the "default" struct tag of the named field on T.
// Scalar defaults live in that tag: the same tag drives schema generation
// (pkg/yamlschema) and these runtime fallbacks. List defaults live in the
// Default* vars of this package, documented in the field's desc text.
func structDefault[T any](field string) string {
	structField, ok := reflect.TypeFor[T]().FieldByName(field)
	if !ok {
		return ""
	}

	return structField.Tag.Get("default")
}

// orDefault returns configured when non-nil (an explicitly empty slice clears
// the default), else def.
func orDefault(configured, def []string) []string {
	if configured != nil {
		return configured
	}

	return def
}

// KexecImage

var (
	ErrDefaultImageUnsupportedArch = errors.New("architecture not supported by default kexec")
)

type KexecImage string

// Get returns the configured image or the built-in kexec tarball URL from the
// KexecConfig.Image "default" tag ($PANIX_ARCH is expanded at runtime).
func (k KexecImage) Get() KexecImage {
	if k == "" {
		return KexecImage(structDefault[KexecConfig]("Image"))
	}

	return k
}

func (k KexecImage) String() string {
	return string(k.Get())
}

func (k KexecImage) IfDefaultImageIsArchSupported(arch string) error {
	if string(k) == "" {
		defaultKexecImageSupportedPlatforms := []string{"x86_64", "aarch64"}

		if !slices.Contains(defaultKexecImageSupportedPlatforms, arch) {
			return errors.Wrapf(ErrDefaultImageUnsupportedArch, "%s (supported: %s)", strconv.Quote(arch), defaultKexecImageSupportedPlatforms)
		}
	}

	return nil
}

// KexecSSHPort

type KexecSSHPort uint16

func (p KexecSSHPort) Get() uint16 {
	if p == 0 {
		return ssh.SSHDefaultPort
	}

	return uint16(p)
}

func (p KexecSSHPort) String() string {
	return strconv.Itoa(int(p.Get()))
}

// SudoProgram

type SudoProgram string

func (s SudoProgram) Get() SudoProgram {
	if s == "" {
		return "sudo"
	}

	return s
}

func (s SudoProgram) String() string {
	return string(s.Get())
}

// FileMode

const FileModeDefault FileMode = 0o700

// FileMode is an octal file mode written as exactly four octal digits, for
// example 0700 or 1777. Other spellings are rejected so there is one style.
type FileMode os.FileMode

// IsFileMode marks FileMode for schema generation: its `default` struct tag is
// an octal mode, not a decimal integer.
func (FileMode) IsFileMode() {}

// UnmarshalYAML reads the raw YAML scalar through the AST node so the lexical
// form survives goccy's numeric conversion: unquoted 0600 would otherwise
// arrive as the integer 384 and be indistinguishable from a decimal value.
func (f *FileMode) UnmarshalYAML(unmarshal func(any) error) error {
	var node ast.Node

	err := unmarshal(&node)
	if err != nil {
		return err
	}

	if node == nil || node.GetToken() == nil {
		*f = 0

		return nil
	}

	mode, err := filemode.Parse(node.GetToken().Value)
	if err != nil {
		return errors.Wrap(err, "parse file mode")
	}

	*f = FileMode(mode)

	return nil
}

func (f FileMode) Get() FileMode {
	if f == 0 {
		return FileModeDefault
	}

	return f
}

func (f FileMode) String() string {
	return filemode.Format(uint32(f.Get()))
}

// StatString returns the unpadded octal form emitted by stat -c %a.
func (f FileMode) StatString() string {
	return strconv.FormatUint(uint64(f.Get()), 8)
}
