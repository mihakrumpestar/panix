package config

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func TestMachineOrder(t *testing.T) {
	f := flags.Flags{
		GlobalFlags: flags.GlobalFlags{
			Config: "../../examples/panix.deploy.yml",
		},
	}
	conf, err := LoadConfig(f, []phases.Phase{phases.Inspect})
	if err != nil {
		t.Fatal(err)
	}

	expectedConfigs := []string{"personal-workstation", "personal-laptop", "server-01", "server-03", "vps-02", "kiosk"}

	var actualConfigs []string
	for _, flakePair := range conf.Fleet.Flakes.Pairs() {
		for _, configPair := range flakePair.Value.Configurations.Pairs() {
			actualConfigs = append(actualConfigs, configPair.Key)
		}
	}

	if len(actualConfigs) != len(expectedConfigs) {
		t.Fatalf("expected %d configs, got %d", len(expectedConfigs), len(actualConfigs))
	}

	for i, expected := range expectedConfigs {
		if actualConfigs[i] != expected {
			t.Errorf("config[%d]: expected %q, got %q", i, expected, actualConfigs[i])
		}
	}
}
