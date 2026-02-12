package config_attributes

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Secret struct {
	Local  Local  `yaml:"local"`
	Remote Remote `yaml:"remote"`
}

type Local struct {
	Path          *string `yaml:"path"`
	CommandOutput *string `yaml:"command_output"`
}

type Remote struct {
	Path string `yaml:"path"`
	UID  *uint  `yaml:"uid,omitempty"`
	GID  *uint  `yaml:"gid,omitempty"`
}

func (sc *Secret) Validate() error {
	switch {
	case sc.Local.Path == nil && sc.Local.CommandOutput == nil:
		return errors.New("both local input secrets options are empty")
	case sc.Local.Path != nil && sc.Local.CommandOutput != nil:
		return errors.New("can't use both local input secrets options")
	case sc.Remote.Path == "":
		return errors.New("remote secrets path is empty")
	case !strings.HasPrefix(sc.Remote.Path, "/"):
		return fmt.Errorf("remote secrets path must be absolute for %s", strconv.Quote(sc.Remote.Path))
	}
	return nil
}
