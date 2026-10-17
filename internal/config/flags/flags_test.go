package flags

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
)

func TestDefaultIfNoTTY(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		output     OutputMode
		wantOutput OutputMode
		wantExit   bool
	}{
		{"sets console when no tty and empty output", "", OutputModeConsole, true},
		{"preserves explicit tui output", OutputModeTui, OutputModeTui, false},
		{"preserves explicit console output", OutputModeConsole, OutputModeConsole, true},
		{"preserves explicit json output", OutputModeJSON, OutputModeJSON, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			flags := &Flags{}
			flags.Output = test.output

			flags.DefaultIfNoTTY()

			assertion := assert.New(t)
			assertion.Equal(test.wantOutput, flags.Output)
			assertion.Equal(test.wantExit, flags.ExitOnComplete)
		})
	}
}

// TestApplySourcesPrecedence pins the source precedence: the explicit mask beats
// the config file even at Go zero values, config values beat non-explicit parsed
// values, and parsed values fill what the config leaves empty.
//
//nolint:funlen // single precedence table
func TestApplySourcesPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   Flags
		parsed   Flags
		explicit map[string]bool
		want     Flags
	}{
		{
			name:     "explicit string beats config",
			config:   Flags{WorkflowFlags: WorkflowFlags{Tui: Tui{Theme: "config-theme"}}},
			parsed:   Flags{WorkflowFlags: WorkflowFlags{Tui: Tui{Theme: "cli-theme"}}},
			explicit: map[string]bool{"tui.theme": true},
			want:     Flags{WorkflowFlags: WorkflowFlags{Tui: Tui{Theme: "cli-theme"}}},
		},
		{
			name:     "explicit empty string beats config",
			config:   Flags{WorkflowFlags: WorkflowFlags{Tui: Tui{Theme: "config-theme"}}},
			parsed:   Flags{},
			explicit: map[string]bool{"tui.theme": true},
			want:     Flags{},
		},
		{
			name:     "explicit themes file beats config",
			config:   Flags{WorkflowFlags: WorkflowFlags{Tui: Tui{ThemesFile: "config-themes.yml"}}},
			parsed:   Flags{WorkflowFlags: WorkflowFlags{Tui: Tui{ThemesFile: "cli-themes.yml"}}},
			explicit: map[string]bool{"tui.themes-file": true},
			want:     Flags{WorkflowFlags: WorkflowFlags{Tui: Tui{ThemesFile: "cli-themes.yml"}}},
		},
		{
			name:     "explicit false bool beats config true",
			config:   Flags{WorkflowFlags: WorkflowFlags{DryRun: true}},
			parsed:   Flags{},
			explicit: map[string]bool{"dry-run": true},
			want:     Flags{},
		},
		{
			name:     "explicit zero int beats config",
			config:   Flags{WorkflowFlags: WorkflowFlags{Tui: Tui{CommandOutputMaxHeight: 5}}},
			parsed:   Flags{},
			explicit: map[string]bool{"tui.command-output-max-height": true},
			want:     Flags{},
		},
		{
			name:     "explicit zero uint beats config",
			config:   Flags{WorkflowFlags: WorkflowFlags{Runtime: Runtime{MaxConcurrency: 4}}},
			parsed:   Flags{},
			explicit: map[string]bool{"runtime.max-concurrency": true},
			want:     Flags{},
		},
		{
			name:     "explicit duration beats config",
			config:   Flags{WorkflowFlags: WorkflowFlags{EvalFlags: EvalFlags{Timeout: 2 * time.Hour}}},
			parsed:   Flags{WorkflowFlags: WorkflowFlags{EvalFlags: EvalFlags{Timeout: 5 * time.Minute}}},
			explicit: map[string]bool{"timeout": true},
			want:     Flags{WorkflowFlags: WorkflowFlags{EvalFlags: EvalFlags{Timeout: 5 * time.Minute}}},
		},
		{
			name:     "explicit zero duration beats config",
			config:   Flags{WorkflowFlags: WorkflowFlags{EvalFlags: EvalFlags{Timeout: 2 * time.Hour}}},
			parsed:   Flags{},
			explicit: map[string]bool{"timeout": true},
			want:     Flags{},
		},
		{
			name:     "explicit slice beats config",
			config:   Flags{WorkflowFlags: WorkflowFlags{EvalFlags: EvalFlags{Tags: []string{"production"}}}},
			parsed:   Flags{WorkflowFlags: WorkflowFlags{EvalFlags: EvalFlags{Tags: []string{"staging"}}}},
			explicit: map[string]bool{"tags": true},
			want:     Flags{WorkflowFlags: WorkflowFlags{EvalFlags: EvalFlags{Tags: []string{"staging"}}}},
		},
		{
			name: "explicit activation mode beats config",
			config: Flags{WorkflowFlags: WorkflowFlags{
				EvalFlags: EvalFlags{ActivationMode: ActivationMode{AllTypes: "switch"}},
			}},
			parsed: Flags{WorkflowFlags: WorkflowFlags{
				EvalFlags: EvalFlags{ActivationMode: ActivationMode{AllTypes: "test"}},
			}},
			explicit: map[string]bool{"activation-mode": true},
			want: Flags{WorkflowFlags: WorkflowFlags{
				EvalFlags: EvalFlags{ActivationMode: ActivationMode{AllTypes: "test"}},
			}},
		},
		{
			name: "config beats non-explicit parsed values",
			config: Flags{WorkflowFlags: WorkflowFlags{
				Tui:       Tui{Theme: "config-theme"},
				EvalFlags: EvalFlags{Timeout: 30 * time.Minute},
				DryRun:    true,
			}},
			parsed: Flags{WorkflowFlags: WorkflowFlags{
				Tui:               Tui{Theme: "parsed-theme"},
				EvalFlags:         EvalFlags{Timeout: 2 * time.Hour},
				OutLinksDir:       ".panix",
				RequireAllSuccess: true,
			}},
			explicit: map[string]bool{},
			want: Flags{WorkflowFlags: WorkflowFlags{
				Tui:               Tui{Theme: "config-theme"},
				EvalFlags:         EvalFlags{Timeout: 30 * time.Minute},
				DryRun:            true,
				OutLinksDir:       ".panix",
				RequireAllSuccess: true,
			}},
		},
		{
			name:   "non-explicit parsed values fill empty config",
			config: Flags{},
			parsed: Flags{WorkflowFlags: WorkflowFlags{
				Tui:         Tui{Theme: "parsed-theme"},
				EvalFlags:   EvalFlags{Timeout: 2 * time.Hour},
				OutLinksDir: ".panix",
			}},
			explicit: map[string]bool{},
			want: Flags{WorkflowFlags: WorkflowFlags{
				Tui:         Tui{Theme: "parsed-theme"},
				EvalFlags:   EvalFlags{Timeout: 2 * time.Hour},
				OutLinksDir: ".panix",
			}},
		},
		{
			name:     "both empty stays empty",
			config:   Flags{},
			parsed:   Flags{},
			explicit: map[string]bool{},
			want:     Flags{},
		},
		{
			name: "explicit names without a Flags leaf are ignored",
			config: Flags{WorkflowFlags: WorkflowFlags{
				Output: OutputModeConsole,
				Tui:    Tui{Theme: "config-theme"},
			}},
			parsed:   Flags{},
			explicit: map[string]bool{"help": true, "unknown-flag": true, "output": true},
			want: Flags{WorkflowFlags: WorkflowFlags{
				Output: OutputModeConsole,
				Tui:    Tui{Theme: "config-theme"},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.config

			err := got.ApplySources(tt.parsed, tt.explicit)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExplicitMask pins which sources mark a flag as explicitly set: the command
// line and PANIX_* env vars do, kong defaults do not.
// Not parallel: it reads and mutates the process environment.
//
//nolint:funlen,paralleltest // the process environment is global state
func TestExplicitMask(t *testing.T) {
	t.Cleanup(unsetEnvars())

	tests := []struct {
		name string
		env  map[string]string
		args []string
		want map[string]bool
	}{
		{
			name: "nothing set leaves the mask empty",
			want: map[string]bool{},
		},
		{
			name: "argv value is explicit",
			args: []string{"--tui.theme=rose-pine"},
			want: map[string]bool{"tui.theme": true},
		},
		{
			name: "env value is explicit",
			env:  map[string]string{"PANIX_TUI_THEME": "rose-pine"},
			want: map[string]bool{"tui.theme": true},
		},
		{
			name: "themes file argv value is explicit",
			args: []string{"--tui.themes-file=custom-themes.yml"},
			want: map[string]bool{"tui.themes-file": true},
		},
		{
			name: "themes file env value is explicit",
			env:  map[string]string{"PANIX_TUI_THEMES_FILE": "custom-themes.yml"},
			want: map[string]bool{"tui.themes-file": true},
		},
		{
			name: "empty env value is still explicit",
			env:  map[string]string{"PANIX_TUI_THEME": ""},
			want: map[string]bool{"tui.theme": true},
		},
		{
			name: "value equal to the kong default is explicit",
			args: []string{"--timeout=2h"},
			want: map[string]bool{"timeout": true},
		},
		{
			name: "explicit false bool is explicit",
			args: []string{"--dry-run=false"},
			want: map[string]bool{"dry-run": true},
		},
		{
			name: "explicit zero value is explicit",
			args: []string{"--runtime.max-concurrency=0"},
			want: map[string]bool{"runtime.max-concurrency": true},
		},
		{
			name: "short flag is explicit",
			args: []string{"-d"},
			want: map[string]bool{"debug": true},
		},
		{
			name: "argv and env combine",
			env:  map[string]string{"PANIX_TUI_THEME": "rose-pine"},
			args: []string{"--timeout=5m", "--tui.theme=rose-pine-dawn"},
			want: map[string]bool{"tui.theme": true, "timeout": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			assert.Equal(t, tt.want, ExplicitMask(tt.args, Flags{}))
		})
	}
}

// Command scope fixtures mirror the real command layouts: the eval command
// declares only ConfigFlags, EvalFlags, a prefixed Runtime and its local output
// file flag (not a Flags leaf); workflow commands declare WorkflowFlags and
// rollback adds RollbackFlags.
type evalScopeCmd struct {
	ConfigFlags
	EvalFlags
	Runtime `embed:"" prefix:"runtime."`

	OutputFile string `name:"output" short:"o" help:"Output file path"`
}

type workflowScopeCmd struct {
	WorkflowFlags
}

type rollbackScopeCmd struct {
	RollbackFlags
	WorkflowFlags
}

// TestExplicitMaskCommandScope pins that env provenance is scoped to the flags
// the running command declares. Not parallel: it mutates the process environment.
//
//nolint:paralleltest // the process environment is global state
func TestExplicitMaskCommandScope(t *testing.T) {
	t.Cleanup(unsetEnvars())

	t.Setenv("PANIX_CONFIG", "x.yml")
	t.Setenv("PANIX_TIMEOUT", "5m")
	t.Setenv("PANIX_RUNTIME_MAX_CONCURRENCY", "3")
	t.Setenv("PANIX_OUTPUT_MODE", "json")
	t.Setenv("PANIX_TUI_THEME", "rose-pine-dawn")
	t.Setenv("PANIX_DRY_RUN", "true")
	t.Setenv("PANIX_GEN", "3")

	tests := []struct {
		name     string
		cmd      any
		contains []string
		excludes []string
	}{
		{
			name:     "eval command owns only its declared leaves",
			cmd:      evalScopeCmd{},
			contains: []string{"config", "timeout", "runtime.max-concurrency"},
			excludes: []string{"tui.theme", "dry-run", "gen", "output-mode"},
		},
		{
			name:     "workflow command owns the workflow leaves",
			cmd:      workflowScopeCmd{},
			contains: []string{"config", "timeout", "runtime.max-concurrency", "output-mode", "tui.theme", "dry-run"},
			excludes: []string{"gen"},
		},
		{
			name:     "rollback command adds the gen leaf",
			cmd:      rollbackScopeCmd{},
			contains: []string{"tui.theme", "dry-run", "gen"},
		},
		{
			name:     "full Flags owns every leaf",
			cmd:      Flags{},
			contains: []string{"tui.theme", "dry-run", "gen"},
		},
		{
			name:     "nil command keeps the full Flags leaf set",
			cmd:      nil,
			contains: []string{"tui.theme", "dry-run", "gen"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mask := ExplicitMask(nil, tt.cmd)

			for _, name := range tt.contains {
				assert.Truef(t, mask[name], "mask must contain %q: %v", name, mask)
			}

			for _, name := range tt.excludes {
				assert.Falsef(t, mask[name], "mask must not contain %q: %v", name, mask)
			}
		})
	}
}

// kongTraceExplicitMask is the kong-internals implementation kept only as a
// parity oracle in tests, so the command-scoped scanner cannot silently diverge
// from kong's own trace.
func kongTraceExplicitMask(ctx *kong.Context) map[string]bool {
	explicit := map[string]bool{}

	for _, element := range ctx.Path {
		if element.Flag != nil && !element.Resolved {
			explicit[element.Flag.Name] = true
		}
	}

	for _, flag := range ctx.Flags() {
		for _, env := range flag.Envs {
			if _, ok := os.LookupEnv(env); ok {
				explicit[flag.Name] = true

				break
			}
		}
	}

	return explicit
}

// scopedFlagsLeaves returns the Flags leaf names a command declares, derived
// independently of scopeLeaves so the parity harness is not circular.
func scopedFlagsLeaves(cmd any) map[string]bool {
	typ := reflect.TypeOf(cmd)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	cmdNames := leafFieldNames(typ)
	names := map[string]bool{}

	for name := range leafFieldNames(reflect.TypeFor[Flags]()) {
		if _, ok := cmdNames[name]; ok {
			names[name] = true
		}
	}

	return names
}

// assertMaskParity parses args with kong for cmd and asserts ExplicitMask equals
// the kong-trace mask filtered to cmd's Flags leaves.
func assertMaskParity(t *testing.T, cmd any, scoped map[string]bool, args []string) {
	t.Helper()

	target := reflect.New(reflect.TypeOf(cmd))

	parser := kong.Must(target.Interface(), kong.NoDefaultHelp(), kong.DefaultEnvars(EnvPrefix), kong.FlagNamer(KongFlagName))

	kctx, err := parser.Parse(args)
	require.NoErrorf(t, err, "args=%v", args)

	old := map[string]bool{}

	for name := range kongTraceExplicitMask(kctx) {
		if scoped[name] {
			old[name] = true
		}
	}

	assert.Equalf(t, old, ExplicitMask(args, cmd), "args=%v", args)
}

// TestExplicitMaskMatchesKongTrace is the permanent parity harness: for full
// Flags and every real command scope it parses valid argv with kong and asserts
// the panix-owned ExplicitMask equals the kong-trace mask filtered to the
// command's Flags leaves, including env vars whose leaf a command does not
// declare. Not parallel: it mutates the process environment.
//
//nolint:funlen,paralleltest // the process environment is global state
func TestExplicitMaskMatchesKongTrace(t *testing.T) {
	t.Cleanup(unsetEnvars())

	t.Setenv("PANIX_TIMEOUT", "5m")
	t.Setenv("PANIX_TUI_THEME", "rose-pine-dawn")
	t.Setenv("PANIX_DRY_RUN", "true")
	t.Setenv("PANIX_GEN", "3")
	t.Setenv("PANIX_OUTPUT_MODE", "json")
	t.Setenv("PANIX_OUTPUT", "out.yml")

	fixtures := []struct {
		name string
		cmd  any
		args [][]string
	}{
		{
			name: "full Flags",
			cmd:  Flags{},
			args: [][]string{
				nil,
				{"--config", "x.yml"},
				{"--timeout=1h"},
				{"--runtime.max-concurrency=0"},
				{"-c", "x.yml"},
				{"-cx.yml"},
				{"--tags", "a"},
				{"-t", "a"},
				{"--dry-run"},
				{"--dry-run=false"},
				{"--tui.theme=rose-pine"},
				{"-d"},
				{"-dl"},
				{"--output-mode=json"},
				{"--gen=3"},
				{"--snapshot.dir=/tmp/x"},
				{"--profile.cpu=/tmp/cpu"},
			},
		},
		{
			name: "workflow scope",
			cmd:  workflowScopeCmd{},
			args: [][]string{
				nil,
				{"--config", "x.yml"},
				{"--timeout=1h"},
				{"--runtime.max-concurrency=0"},
				{"-c", "x.yml"},
				{"-cx.yml"},
				{"--tags", "a"},
				{"-t", "a"},
				{"--dry-run"},
				{"--dry-run=false"},
				{"--tui.theme=rose-pine"},
				{"-d"},
				{"-dl"},
				{"--output-mode=json"},
				{"--snapshot.dir=/tmp/x"},
				{"--profile.cpu=/tmp/cpu"},
			},
		},
		{
			name: "rollback scope",
			cmd:  rollbackScopeCmd{},
			args: [][]string{
				nil,
				{"--gen=3"},
				{"--gen", "3"},
				{"--dry-run"},
				{"--tui.theme=rose-pine"},
				{"-c", "x.yml"},
			},
		},
		{
			name: "eval scope",
			cmd:  evalScopeCmd{},
			args: [][]string{
				nil,
				{"--config", "x.yml"},
				{"--timeout=1h"},
				{"--runtime.max-concurrency=0"},
				{"-c", "x.yml"},
				{"-cx.yml"},
				{"--tags", "a"},
				{"-t", "a"},
				{"--skip-phases", "inspect"},
				{"--validate.flakes"},
				{"--activation-mode", "test"},
				{"--output", "out.yml"},
				{"-o", "out.yml"},
			},
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			scoped := scopedFlagsLeaves(fixture.cmd)

			for _, args := range fixture.args {
				assertMaskParity(t, fixture.cmd, scoped, args)
			}
		})
	}
}

// TestScanArgv pins the argv scanner grammar: long and short forms, bool versus
// value leaves, the "--" terminator, unknown names, flag-looking values, repeats,
// nested names and interleaved positionals.
//
//nolint:funlen // single scanner grammar table
func TestScanArgv(t *testing.T) {
	t.Parallel()

	leaves := leafFields(reflect.TypeFor[Flags]())

	tests := []struct {
		name string
		args []string
		want map[string]bool
	}{
		{"empty", nil, map[string]bool{}},
		{"long equals value leaf", []string{"--tui.theme=rose-pine"}, map[string]bool{"tui.theme": true}},
		{"long space value leaf", []string{"--tui.theme", "rose-pine"}, map[string]bool{"tui.theme": true}},
		{"bare bool leaf", []string{"--dry-run"}, map[string]bool{"dry-run": true}},
		{"explicit false bool leaf", []string{"--dry-run=false"}, map[string]bool{"dry-run": true}},
		{"empty value leaf", []string{"--tui.theme="}, map[string]bool{"tui.theme": true}},
		{"short bool leaf", []string{"-d"}, map[string]bool{"debug": true}},
		{"short attached value", []string{"-cpanix.yml"}, map[string]bool{"config": true}},
		{"short separate value", []string{"-c", "panix.yml"}, map[string]bool{"config": true}},
		{"short equals value", []string{"-c=panix.yml"}, map[string]bool{"config": true}},
		{"short value at end of args", []string{"-c"}, map[string]bool{"config": true}},
		{"combined bool shorts", []string{"-dl"}, map[string]bool{"debug": true, "log": true}},
		{"combined shorts ending in value leaf", []string{"-dc", "panix.yml"}, map[string]bool{"debug": true, "config": true}},
		{"combined shorts with attached value", []string{"-dcpanix.yml"}, map[string]bool{"debug": true, "config": true}},
		{"short slice value", []string{"-t", "prod"}, map[string]bool{"tags": true}},
		{"short slice attached value", []string{"-tprod"}, map[string]bool{"tags": true}},
		{"unknown long ignored", []string{"--nope", "value"}, map[string]bool{}},
		{"unknown short ignored", []string{"-z"}, map[string]bool{}},
		{"single dash is a positional", []string{"-"}, map[string]bool{}},
		{"terminator stops scanning", []string{"--", "--dry-run", "-d"}, map[string]bool{}},
		{"value that looks like a flag is consumed", []string{"--config", "--dry-run"}, map[string]bool{"config": true}},
		{"nested name", []string{"--runtime.max-concurrency=0"}, map[string]bool{"runtime.max-concurrency": true}},
		{"repeated flags collapse", []string{"--timeout=1m", "--timeout=2m"}, map[string]bool{"timeout": true}},
		{"interleaved positionals", []string{"deploy", "--dry-run", "positional", "-d"}, map[string]bool{"dry-run": true, "debug": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := map[string]bool{}

			scanArgv(tt.args, leaves, got)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNoBoolMapperLeaf guards the argv scanner's bool detection: leafFields
// derives boolness from reflect.Kind, valid only while no Flags leaf implements
// kong's BoolMapperValue interface. If one is added, scanArgv must ask the mapper.
func TestNoBoolMapperLeaf(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeFor[Flags]()

	for name, field := range leafFields(typ) {
		fieldType := typ.FieldByIndex(field.index).Type

		_, isBoolMapper := reflect.TypeAssert[kong.BoolMapperValue](reflect.New(fieldType))

		assert.False(t, isBoolMapper,
			"leaf %q implements kong.BoolMapperValue; scanArgv must ask the mapper for boolness", name)
	}
}

// TestNoEnvOrNegatableTags guards the leaf walker's assumption that every leaf
// derives its envar from EnvPrefix: a kong env: tag would add a source the walker
// never scans, and a negatable tag would add a flag variant ExplicitMask does not
// model.
func TestNoEnvOrNegatableTags(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeFor[Flags]()

	for name, field := range leafFields(typ) {
		structField := typ.FieldByIndex(field.index)

		assert.Falsef(t, hasTagKey(structField, "env"),
			"leaf %q declares a kong env tag; the walker relies on derived envars only", name)
		assert.Falsef(t, hasTagKey(structField, "negatable"),
			"leaf %q declares a kong negatable tag; the walker does not model negated flags", name)
	}
}

// hasTagKey reports whether field declares key either as a standalone struct tag
// or as an option inside the kong:"" tag.
func hasTagKey(field reflect.StructField, key string) bool {
	if _, ok := field.Tag.Lookup(key); ok {
		return true
	}

	for option := range strings.SplitSeq(field.Tag.Get("kong"), ",") {
		name, _, _ := strings.Cut(option, "=")
		if strings.TrimSpace(name) == key {
			return true
		}
	}

	return false
}

// TestEnvarNameMatchesKong is the envar naming drift guard: for every Flags leaf,
// envarName must equal the PANIX_* env var kong generates with
// kong.DefaultEnvars. Not parallel: kong resolves PANIX_* envars at parse time.
//
//nolint:paralleltest // kong parses resolve PANIX_* envars at parse time
func TestEnvarNameMatchesKong(t *testing.T) {
	t.Cleanup(unsetEnvars())

	_, kongEnvs := kongFlagNames(t, nil)
	leaves := leafFieldNames(reflect.TypeFor[Flags]())

	require.Len(t, kongEnvs, len(leaves), "kong must generate one envar per Flags leaf")

	for name := range leaves {
		env, ok := kongEnvs[name]
		require.True(t, ok, "kong must generate an envar for leaf %q", name)

		assert.Equal(t, env, envarName(name), "envar name drift for leaf %q", name)
	}
}

// TestKongFlagNameMatchesKong is the flag naming drift guard: the names kong
// derives for the real Flags struct, with and without FlagNamer(KongFlagName),
// must match leafFieldNames 1:1 and each other, and every flag must keep exactly
// one PANIX_* envar. Not parallel: kong resolves PANIX_* envars at parse time.
//
//nolint:paralleltest // kong parses resolve PANIX_* envars at parse time
func TestKongFlagNameMatchesKong(t *testing.T) {
	t.Cleanup(unsetEnvars())

	walkerNames := map[string]bool{}

	for name := range leafFieldNames(reflect.TypeFor[Flags]()) {
		walkerNames[name] = true
	}

	defaultNames, defaultEnvs := kongFlagNames(t, nil)
	namerNames, namerEnvs := kongFlagNames(t, []kong.Option{kong.FlagNamer(KongFlagName)})

	assertion := assert.New(t)

	assertion.Equal(walkerNames, defaultNames,
		"kong's default namer and leafFieldNames must agree 1:1")
	assertion.Equal(walkerNames, namerNames,
		"FlagNamer(KongFlagName) must not change any flag name")
	assertion.Equal(defaultEnvs, namerEnvs,
		"FlagNamer registration must not change any PANIX_* envar")
}

// kongFlagNames parses the real Flags struct with kong and returns the flag
// names with their single PANIX_* envar. Default help is disabled so the flag
// set is exactly the Flags leaves.
func kongFlagNames(t *testing.T, extra []kong.Option) (map[string]bool, map[string]string) {
	t.Helper()

	var cli struct {
		Flags
	}

	options := append([]kong.Option{kong.NoDefaultHelp(), kong.DefaultEnvars(EnvPrefix)}, extra...)
	parser := kong.Must(&cli, options...)

	kctx, err := parser.Parse(nil)
	require.NoError(t, err)

	names := map[string]bool{}
	envs := map[string]string{}

	for _, flag := range kctx.Flags() {
		names[flag.Name] = true

		require.Len(t, flag.Envs, 1, "flag %s must have exactly one envar", flag.Name)
		require.True(t, strings.HasPrefix(flag.Envs[0], EnvPrefix+"_"),
			"flag %s envar must use the %s_ prefix", flag.Name, EnvPrefix)

		envs[flag.Name] = flag.Envs[0]
	}

	return names, envs
}

func TestOutputModeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode OutputMode
		want string
	}{
		{"tui", OutputModeTui, "tui"},
		{"console", OutputModeConsole, "console"},
		{"json", OutputModeJSON, "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, string(tt.mode))
		})
	}
}

func TestWorkflowFlagsActivationMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode string
		want string
	}{
		{"switch mode", "switch", "switch"},
		{"boot mode", "boot", "boot"},
		{"test mode", "test", "test"},
		{"check mode", "check", "check"},
		{"dry-activate mode", "dry-activate", "dry-activate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.mode)
		})
	}
}

func TestSkipPhasesInWorkflowFlags(t *testing.T) {
	t.Parallel()

	flags := &Flags{
		WorkflowFlags: WorkflowFlags{
			EvalFlags: EvalFlags{SkipPhases: []phase.Phase{phase.Inspect, phase.Build}},
		},
	}

	assertion := assert.New(t)
	assertion.Len(flags.SkipPhases, 2)
	assertion.Equal(phase.Inspect, flags.SkipPhases[0])
	assertion.Equal(phase.Build, flags.SkipPhases[1])
}

// Guards the kong default tag OutLinksDir relies on.
func TestOutLinksDirDefaultTag(t *testing.T) {
	t.Parallel()

	field, ok := reflect.TypeFor[WorkflowFlags]().FieldByName("OutLinksDir")
	require.True(t, ok, "WorkflowFlags.OutLinksDir field must exist")

	assert.Equal(t, ".panix", field.Tag.Get("default"))
}

// Guards the kong name:"output-mode" tag on Output: the CLI flag name and the
// PANIX_OUTPUT_MODE envar must not collide with the eval command's --output/-o
// file flag, while the YAML key flags.output stays "output".
// TestKongFlagNameMatchesKong covers the kong side of the same contract.
func TestOutputModeFlagNameTag(t *testing.T) {
	t.Parallel()

	field, ok := reflect.TypeFor[WorkflowFlags]().FieldByName("Output")
	require.True(t, ok, "WorkflowFlags.Output field must exist")

	assertion := assert.New(t)

	assertion.Equal("output-mode", field.Tag.Get("name"), "CLI flag name")
	assertion.Equal("output", field.Tag.Get("yaml"), "config YAML key must not change")

	names := leafFieldNames(reflect.TypeFor[Flags]())

	assertion.Contains(names, "output-mode", "walker must honor the name tag")
	assertion.NotContains(names, "output", "the old flag name must not map to any leaf")
}

// TestKongOutputEmptyWithoutFlag guards against reintroducing a kong default
// tag on Output: DefaultIfNoTTY relies on an empty Output to auto-select
// console mode when no TTY is present.
func TestKongOutputEmptyWithoutFlag(t *testing.T) {
	t.Parallel()

	var cli struct {
		WorkflowFlags
	}

	parser := kong.Must(&cli)

	_, err := parser.Parse(nil)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Empty(cli.Output, "kong must not default Output when --output-mode is not passed")
}

// TestTuiThemeFlagSources covers every source of Tui.Theme: CLI flag, PANIX_*
// env var and their precedence. Not parallel: t.Setenv is not parallel-safe.
//
//nolint:paralleltest // t.Setenv is not parallel-safe
func TestTuiThemeFlagSources(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want string
	}{
		{"kong default when nothing is set", "", nil, "panix"},
		{"cli flag sets name", "", []string{"--tui.theme=rose-pine"}, "rose-pine"},
		{"env var sets name", "rose-pine-dawn", nil, "rose-pine-dawn"},
		{"cli flag beats env var", "rose-pine-dawn", []string{"--tui.theme=panix"}, "panix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A set-but-empty envar is an explicit value for kong and beats the
			// flag default, so only set it for cases that provide one.
			if tt.env != "" {
				t.Setenv("PANIX_TUI_THEME", tt.env)
			}

			var cli struct {
				WorkflowFlags
			}

			parser := kong.Must(&cli, kong.DefaultEnvars(EnvPrefix))

			_, err := parser.Parse(tt.args)
			require.NoError(t, err)

			assert.Equal(t, tt.want, cli.Tui.Theme)
		})
	}
}

// TestTuiThemesFileFlagSources covers every source of Tui.ThemesFile: CLI flag,
// PANIX_* env var and their precedence. There is no kong default, so nothing set
// yields the empty implicit-discovery value. Not parallel: t.Setenv is not
// parallel-safe.
//
//nolint:paralleltest // t.Setenv is not parallel-safe
func TestTuiThemesFileFlagSources(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want string
	}{
		{"kong default absent when nothing is set", "", nil, ""},
		{"cli flag sets path", "", []string{"--tui.themes-file=custom-themes.yml"}, "custom-themes.yml"},
		{"env var sets path", "custom-themes.yml", nil, "custom-themes.yml"},
		{"cli flag beats env var", "env-themes.yml", []string{"--tui.themes-file=cli-themes.yml"}, "cli-themes.yml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A set-but-empty envar is an explicit value for kong and beats the
			// flag default, so only set it for cases that provide one.
			if tt.env != "" {
				t.Setenv("PANIX_TUI_THEMES_FILE", tt.env)
			}

			var cli struct {
				WorkflowFlags
			}

			parser := kong.Must(&cli, kong.DefaultEnvars(EnvPrefix))

			_, err := parser.Parse(tt.args)
			require.NoError(t, err)

			assert.Equal(t, tt.want, cli.Tui.ThemesFile)
		})
	}
}

// TestTuiThemesFileYAMLDecode pins the flags.tui.themes_file YAML key.
func TestTuiThemesFileYAMLDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"themes file set in yaml", "tui:\n  themes_file: custom-themes.yml\n", "custom-themes.yml"},
		{"themes file absent stays empty", "tui:\n  theme: panix\n", ""},
		{"empty themes file means implicit discovery", "tui:\n  themes_file: \"\"\n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got Flags

			err := yamlx.Decode([]byte(tt.yaml), &got)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Tui.ThemesFile)
		})
	}
}

// TestTuiThemeYAMLDecode pins the flags.tui.theme YAML key.
func TestTuiThemeYAMLDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"theme set in yaml", "tui:\n  theme: rose-pine\n", "rose-pine"},
		{"theme absent stays empty", "tui:\n  show_active_only: true\n", ""},
		{"empty theme means default", "tui:\n  theme: \"\"\n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got Flags

			err := yamlx.Decode([]byte(tt.yaml), &got)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.Tui.Theme)
		})
	}
}
