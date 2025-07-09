package config

import (
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	/*
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test.yaml")

			configContent := `global:
		  flake: "../nixos-config"
		  requireAllSuccess: false
		  autoBootstrap: false
		machines:
		  - name: "test-machine"
		    host: "localhost"
		    user: "root"
		    port: 22
		    tags: ["test"]
		    flakeOutput: "nixosConfigurations.test-machine"
		`

			err := os.WriteFile(configPath, []byte(configContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			// Test loading the config
			cfg, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}
			/*
			   // Validate the loaded config

			   	if cfg.Global.Flake != "../nixos-config" {
			   		t.Errorf("Expected flake '../nixos-config', got '%s'", cfg.Global.Flake)
			   	}

			   	if len(cfg.Machines) != 1 {
			   		t.Errorf("Expected 1 machine, got %d", len(cfg.Machines))
			   	}

			   machine := cfg.Machines[0]

			   	if machine.Name != "test-machine" {
			   		t.Errorf("Expected machine name 'test-machine', got '%s'", machine.Name)
			   	}

			   	if machine.Host != "localhost" {
			   		t.Errorf("Expected host 'localhost', got '%s'", machine.Host)
			   	}

			   	if machine.Port != 22 {
			   		t.Errorf("Expected port 22, got %d", machine.Port)
			   	}
	*/
}

func TestGetMachinesByTags(t *testing.T) {
	/*
		cfg := &Config{
			Machines: []MachineConfig{
				{Name: "prod1", Tags: []string{"prod", "web"}},
				{Name: "prod2", Tags: []string{"prod", "db"}},
				{Name: "test1", Tags: []string{"test", "web"}},
				{Name: "dev1", Tags: []string{"dev"}},
			},
		}

		// Test filtering by single tag
		machines := cfg.GetMachinesByTags([]string{"prod"})
		if len(machines) != 2 {
			t.Errorf("Expected 2 machines with 'prod' tag, got %d", len(machines))
		}

		// Test filtering by multiple tags
		machines = cfg.GetMachinesByTags([]string{"web"})
		if len(machines) != 2 {
			t.Errorf("Expected 2 machines with 'web' tag, got %d", len(machines))
		}

		// Test filtering with inclusion/exclusion
		machines = cfg.GetMachinesByTags([]string{"+prod", "-db"})
		if len(machines) != 1 {
			t.Errorf("Expected 1 machine with '+prod,-db' filter, got %d", len(machines))
		}
		if len(machines) > 0 && machines[0].Name != "prod1" {
			t.Errorf("Expected machine 'prod1', got '%s'", machines[0].Name)
		}

		// Test no filters (should return all)
		machines = cfg.GetMachinesByTags([]string{})
		if len(machines) != 4 {
			t.Errorf("Expected 4 machines with no filter, got %d", len(machines))
		}
	*/
}
