package config_attributes

type Secret struct {
	Local  Local  `yaml:"local,required" desc:"Local secret source" validate:"required"`
	Remote Remote `yaml:"remote,required" desc:"Remote secret destination" validate:"required"`
}

type Local struct {
	Path          *string `yaml:"path" desc:"Path to file containing secret" validate:"required_without_all=CommandOutput,excluded_with=CommandOutput"`
	CommandOutput *string `yaml:"command_output" desc:"Command to generate secret value" validate:"required_without_all=Path,excluded_with=Path"`
}

type Remote struct {
	Path string `yaml:"path,required" desc:"Absolute path on remote machine" validate:"required,abspath"`
	UID  *uint  `yaml:"uid,omitempty" desc:"User ID for the secret file"`
	GID  *uint  `yaml:"gid,omitempty" desc:"Group ID for the secret file"`
}
