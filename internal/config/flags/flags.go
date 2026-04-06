package flags

import (
	"encoding/json"
	"time"

	"dario.cat/mergo"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

type ActivationMode string

const (
	ActivationModeCheck       ActivationMode = "check"        // run pre-switch checks and exit
	ActivationModeSwitch      ActivationMode = "switch"       // make the configuration the boot default and activate now
	ActivationModeBoot        ActivationMode = "boot"         // make the configuration the boot default
	ActivationModeTest        ActivationMode = "test"         // activate the configuration, but don't make it the boot default
	ActivationModeDryActivate ActivationMode = "dry-activate" // show what would be done if this configuration were activated
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
	default:
		return errors.New("invalid duration type")
	}
	return nil
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (d *Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

type Flags struct {
	Config               string            `json:"config" short:"c" help:"Nix config file" default:"panix.nix"`
	Env                  string            `json:"env" help:"Environment name (passed to Nix)"`
	NoValidatePaths      bool              `json:"no_validate_paths" help:"Skip path existence validation"`
	NixArgs              map[string]string `json:"-" help:"Additional args passed to nix eval"`
	Tags                 []string          `json:"tags" short:"t" help:"Filter machines by tags (flakes, configs and names are already registered as tags)"`
	Bootstrap            Bootstrap         `json:"bootstrap" embed:"" prefix:"bootstrap."`
	RequireAllSuccess    bool              `json:"require_all_success" help:"Abort if any task fails, primarily for CI/CD"`
	OverrideLocalMachine string            `json:"override_local_machine" help:"Hostname of the machine that is local (won't use ssh to connect to it)"`
	DryRun               bool              `json:"dry_run" help:"Show what would be done without executing"`
	DryRunWithInspect    bool              `json:"dry_run_with_inspect" help:"Show what would be done without executing, but with real inspect query"`
	Timeout              Duration          `json:"timeout" help:"Timeout per command (eg. '1h', '1m15s')" default:"2h"`
	SkipPhases           []phases.Phase    `json:"skip_phases" short:"s" help:"Declare phases to skip (not all phases can be skipped)"`
	ExitOnComplete       bool              `json:"exit_on_complete" help:"Exit TUI on completion; 'retry' and 'restart' are disabled in this mode"`
	ActivationMode       ActivationMode    `json:"activation_mode" help:"Activation mode: check, switch, boot, test, dry-activate" default:"switch" validate:"omitempty,oneof=check switch boot test dry-activate"` //nolint:lll

	Tui     `json:"tui" embed:"" prefix:"tui."`
	Logging `json:"logging"`
}

type Bootstrap struct {
	DisableAuto  bool `yaml:"disable_auto" help:"Disable automatic bootstrap (even if target machine does not have NixOS installed)"`
	DisableDisko bool `yaml:"disable_disko" help:"Disables building, transfer and bootstrap of disko tool"`
}

type Tui struct {
	ShowAllBuildLogs       bool `yaml:"show_all_build_logs" help:"Show all build logs in TUI (keybind h)"`
	ShowActiveOnly         bool `yaml:"show_active_only" help:"Show only running or errored logs in TUI build logs (keybind a)"`
	ShowCommandsInLabels   bool `yaml:"show_commands_in_labels" help:"Show raw commands instead of descriptions as labels in build logs (keybind c)"`
	CommandOutputMaxHeight int  `yaml:"command_output_max_height" help:"Maximum height for command labels and outputs viewports in TUI" default:"8"`
}

type Logging struct {
	Log        bool   `yaml:"log" short:"l" help:"Enable logging to file"`
	LogFile    string `yaml:"log_file" help:"Log file path" default:"panix.log"`
	Debug      bool   `yaml:"debug" short:"d" help:"Debug output (enables logging)"`
	CPUProfile string `yaml:"cpu_profile" help:"Path for cpu profiling to file, declaring it enables it"`
}

func (f *Flags) MergeConfWithCliFlags(cli Flags) error {
	err := mergo.Merge(f, cli)
	if err != nil {
		return errors.Wrap(err, "failed to merge flags")
	}

	return nil
}
