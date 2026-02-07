package config_attributes

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type SecretConfig struct {
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

func (sc *SecretConfig) Validate() error {
	if sc.Local.Path == nil && sc.Local.CommandOutput == nil {
		return errors.New("both local input secrets options are empty")
	}

	if sc.Local.Path != nil && sc.Local.CommandOutput != nil {
		return errors.New("can't use both local input secrets options")
	}

	if sc.Remote.Path == "" {
		return errors.New("remote secrets path is empty")
	}

	if !strings.HasPrefix(sc.Remote.Path, "/") {
		return fmt.Errorf("remote secrets path must be absolute for %s", strconv.Quote(sc.Remote.Path))
	}

	return nil
}
