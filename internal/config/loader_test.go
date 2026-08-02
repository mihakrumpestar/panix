package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

func testdataPath(t *testing.T, name string) string {
	t.Helper()

	return filepath.Join("testdata", name) //nolint:gojoinpath // test fixture path
}

func TestDecodeConfigFileMinimalValid(t *testing.T) {
	t.Parallel()

	conf, err := decodeConfigFile(testdataPath(t, "minimal_valid.yml"))
	require.NoError(t, err)

	assertion := assert.New(t)

	assertion.NotNil(conf, "config should not be nil")
	assertion.NotNil(conf.Fleet, "fleet should not be nil")

	assertion.Equal(1, conf.Fleet.Flakes.Len(), "expected 1 flake")

	flakePair, ok := conf.Fleet.Flakes.Get("my-flake")
	assertion.True(ok, "expected flake 'my-flake' to exist")
	assertion.Equal("path:./my-flake", flakePair.URL)

	attrMap, ok := flakePair.Installables.Get("nixosConfigurations")
	assertion.True(ok, "expected nixosConfigurations to exist")

	configPair, ok := attrMap.Get("my-config")
	assertion.True(ok, "expected configuration 'my-config' to exist")

	_, ok = configPair.Machines.Get("my-machine")
	assertion.True(ok, "expected machine 'my-machine' to exist")
}

func TestDecodeConfigFileWithSSH(t *testing.T) {
	t.Parallel()

	conf, err := decodeConfigFile(testdataPath(t, "with_ssh.yml"))
	require.NoError(t, err)

	assertion := assert.New(t)

	flakePair, ok := conf.Fleet.Flakes.Get("my-flake")
	require.True(t, ok, "expected flake 'my-flake' to exist")

	attrMap, ok := flakePair.Installables.Get("nixosConfigurations")
	require.True(t, ok, "expected nixosConfigurations to exist")

	configPair, ok := attrMap.Get("my-config")
	require.True(t, ok, "expected configuration 'my-config' to exist")

	machinePair, ok := configPair.Machines.Get("my-machine")
	require.True(t, ok, "expected machine 'my-machine' to exist")

	assertion.Equal("host.example.com", machinePair.SSH.Hostname)
}

func TestDecodeConfigFileWithDisabled(t *testing.T) {
	t.Parallel()

	conf, err := decodeConfigFile(testdataPath(t, "with_disabled_machine.yml"))
	require.NoError(t, err)

	assertion := assert.New(t)

	flakePair, ok := conf.Fleet.Flakes.Get("my-flake")
	require.True(t, ok)

	attrMap, ok := flakePair.Installables.Get("nixosConfigurations")
	require.True(t, ok)

	configPair, ok := attrMap.Get("my-config")
	require.True(t, ok)

	mach, ok := configPair.Machines.Get("my-machine")
	require.True(t, ok)

	assertion.True(mach.Disabled, "machine should be marked as disabled")
}

func TestDecodeConfigFileFileNotFound(t *testing.T) {
	t.Parallel()

	_, err := decodeConfigFile(testdataPath(t, "nonexistent_file.yml"))
	require.Error(t, err)

	assertion := assert.New(t)
	assertion.Contains(err.Error(), "failed reading config")
}

func TestDecodeConfigFileInvalidYAML(t *testing.T) {
	t.Parallel()

	_, err := decodeConfigFile(testdataPath(t, "invalid_yaml.yml"))
	require.Error(t, err)
}

func TestDecodeConfigFileTemplateProcessing(t *testing.T) {
	t.Parallel()

	conf, err := decodeConfigFile(testdataPath(t, "with_gotemplate.yml"))
	require.NoError(t, err)

	assertion := assert.New(t)

	flakePair, ok := conf.Fleet.Flakes.Get("my-flake")
	require.True(t, ok)

	attrMap, ok := flakePair.Installables.Get("nixosConfigurations")
	require.True(t, ok)

	configPair, ok := attrMap.Get("my-config")
	require.True(t, ok)

	machinePair, ok := configPair.Machines.Get("my-machine")
	require.True(t, ok)

	assertion.Equal("host.example.com", machinePair.SSH.Hostname,
		"template should resolve to 'host.example.com'")
}

func TestDecodeConfigFileFullConfig(t *testing.T) {
	t.Parallel()

	conf, err := decodeConfigFile(testdataPath(t, "full_config.yml"))
	require.NoError(t, err)

	assertion := assert.New(t)

	assertion.Equal(1, conf.Fleet.Flakes.Len(), "expected 1 flake")

	flakePair, ok := conf.Fleet.Flakes.Get("infrastructure")
	require.True(t, ok, "expected flake 'infrastructure'")
	assertion.Equal("path:../infrastructure", flakePair.URL,
		"flake URL should match")

	attrMap, ok := flakePair.Installables.Get("nixosConfigurations")
	require.True(t, ok, "expected nixosConfigurations to exist")

	assertion.Equal(2, attrMap.Len(), "expected 2 configurations")

	serverCfg, ok := attrMap.Get("server")
	require.True(t, ok, "expected configuration 'server'")

	serverTags := serverCfg.Tags
	assertion.Contains(serverTags, "production",
		"configuration should have 'production' tag")

	serverMach, ok := serverCfg.Machines.Get("server-01")
	require.True(t, ok)
	assertion.Equal("server-01.example.com", serverMach.SSH.Hostname)
	assertion.True(serverMach.Bootstrap.SSH.IsInitialized(),
		"bootstrap SSH should be initialized")
}

func TestFleetInitSetsFleetXpath(t *testing.T) {
	t.Parallel()

	conf, err := decodeConfigFile(testdataPath(t, "with_ssh.yml"))
	require.NoError(t, err)

	require.NoError(t, conf.Fleet.Init())

	assertion := assert.New(t)

	// Fleet is initialized with name="" so Xpath remains empty
	assertion.Empty(conf.Fleet.Xpath.String(), "fleet xpath should be empty since name is empty")
}

func TestFleetInitSetsNamesAndXpathsThroughHierarchy(t *testing.T) {
	t.Parallel()

	conf, err := decodeConfigFile(testdataPath(t, "with_ssh.yml"))
	require.NoError(t, err)

	must := require.New(t)
	must.NoError(conf.Fleet.Init())

	// Init flake manually since initFleet on Config is hard to test
	// due to SSH config dependency
	flk, ok := conf.Fleet.Flakes.Get("my-flake")
	must.True(ok)

	must.NoError(flk.Init("my-flake", &conf.Fleet.Attributes, &conf.Fleet.Nix))

	assertion := assert.New(t)

	// Fleet Xpath is empty (name=""), so flake xpath is just "my-flake"
	expectedFlakeXpath := xpath.New("my-flake")
	assertion.Equal(expectedFlakeXpath.String(), flk.Xpath.String())
	assertion.Equal("my-flake", flk.Name.String())

	attrMap, ok := flk.Installables.Get("nixosConfigurations")
	must.True(ok)

	cfg, ok := attrMap.Get("my-config")
	must.True(ok)

	must.NoError(cfg.Init(installable.FlakeOutputType("nixosConfigurations"), "my-config", &flk.Attributes, &flk.Nix))

	expectedCfgXpath := expectedFlakeXpath.NewXpathWithAppend("nixosConfigurations/my-config")
	assertion.Equal(expectedCfgXpath.String(), cfg.Xpath.String())
	assertion.Equal("my-config", cfg.Name.String())

	mach, ok := cfg.Machines.Get("my-machine")
	must.True(ok)

	must.NoError(mach.Init("my-machine", &cfg.Attributes))

	// SSH init is separate — test it explicitly
	mach.SSH.Hostname = "host.example.com"
	must.NoError(mach.InitSSH("testhost", nixver.Info{}))

	expectedMachineXpath := expectedCfgXpath.NewXpathWithAppend("my-machine")
	assertion.Equal(expectedMachineXpath.String(), mach.Xpath.String())
	assertion.Equal("my-machine", mach.Name.String())
}

func TestPostUnmarshalInitSetsColorScheme(t *testing.T) {
	t.Parallel()

	conf := &Config{
		Fleet: &fleet.Fleet{
			Flakes: atomicorderedmap.New[string, *flake.Flake](),
		},
	}

	require.NoError(t, conf.Fleet.Init())

	conf.PostUnmarshalInit()

	assertion := assert.New(t)
	assertion.NotNil(conf.ColorScheme, "colorscheme should be initialized")
}

func TestSudoProgramDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input attributes.SudoProgram
		want  string
	}{
		{"empty defaults to sudo", attributes.SudoProgram(""), "sudo"},
		{"custom value preserved", attributes.SudoProgram("doas"), "doas"},
		{"run0 preserved", attributes.SudoProgram("run0"), "run0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.input.Get().String())
		})
	}
}

func TestActivationModeDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input attributes.ActivationModeD
		want  string
	}{
		{"empty defaults to switch", attributes.ActivationModeD(""), "switch"},
		{"check preserved", attributes.ActivationModeD("check"), "check"},
		{"boot preserved", attributes.ActivationModeD("boot"), "boot"},
		{"test preserved", attributes.ActivationModeD("test"), "test"},
		{"dry-activate preserved", attributes.ActivationModeD("dry-activate"), "dry-activate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.input.String())
		})
	}
}

func TestFileModeDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input attributes.FileMode
		want  string
	}{
		{"zero defaults to 0700", attributes.FileMode(0), "700"},
		{"custom value preserved", attributes.FileMode(0o755), "755"},
		{"0o644 preserved", attributes.FileMode(0o644), "644"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.input.String())
		})
	}
}

func TestKexecImageDefaults(t *testing.T) {
	t.Parallel()

	defaultImage := "https://github.com/nix-community/nixos-images/releases/latest/" +
		"download/nixos-kexec-installer-noninteractive-<arch>-linux.tar.gz"

	tests := []struct {
		name  string
		input attributes.KexecImage
		want  string
	}{
		{"empty defaults to default image", attributes.KexecImage(""), defaultImage},
		{"custom url preserved", attributes.KexecImage("https://example.com/custom.tar.gz"),
			"https://example.com/custom.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.input.Get().String())
		})
	}
}

func TestKexecImageArchSupport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
		wantErr bool
	}{
		{"default supports x86_64", "x86_64", false},
		{"default supports aarch64", "aarch64", false},
		{"default rejects armv7l", "armv7l", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			kexecImage := attributes.KexecImage("")
			err := kexecImage.IfDefaultImageIsArchSupported(test.subject)

			assertion := assert.New(t)

			if test.wantErr {
				assertion.Error(err, "expected unsupported arch error")
			} else {
				assertion.NoError(err, "expected supported arch")
			}
		})
	}
}
