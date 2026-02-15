package config_attributes

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Secret struct {
	Local  Local  `yaml:"local" desc:"Local secret source"`
	Remote Remote `yaml:"remote" desc:"Remote secret destination"`
}

type Local struct {
	Path          *string `yaml:"path" desc:"Path to file containing secret"`
	CommandOutput *string `yaml:"command_output" desc:"Command to generate secret value"`
}

type Remote struct {
	Path string `yaml:"path" desc:"Absolute path on remote machine"`
	UID  *uint  `yaml:"uid,omitempty" desc:"User ID for the secret file"`
	GID  *uint  `yaml:"gid,omitempty" desc:"Group ID for the secret file"`
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
