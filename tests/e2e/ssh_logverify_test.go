package main

import (
	"strings"
	"testing"
)

const (
	isoXPathLiteral   = "test/nixosConfigurations/test-vm/nixos-iso-vm"
	kexecXPathLiteral = "test/nixosConfigurations/test-vm-kexec/kexec-vm"

	// notJSONLine mixes into the fixture to prove non-JSON bootstrap output is
	// ignored rather than failing the assertion.
	notJSONLine = `bootstrap output that is not JSON and must be ignored`

	// kexecSkipLine is the unrelated info entry logged before the kexec boot
	// when nixos-generate-config is still unavailable.
	kexecSkipLine = `{"level":"info","xpath":"test/nixosConfigurations/test-vm-kexec/kexec-vm","hardware_config_path":"tests/e2e/testflakes/hardware-configuration.nix","time":"2026-09-18T23:15:29+02:00","message":"skipping hardware config generation before kexec (nixos-generate-config is unavailable on the current OS)"}` //nolint:lll // raw log fixture mirrors the real bootstrap line

	isoGenerateStartLine = `{"level":"info","xpath":"test/nixosConfigurations/test-vm/nixos-iso-vm","phase":"inspect","description":"generate config","command":"ssh -o LogLevel=ERROR -l root -p 10022 -i /tmp/ssh.key 127.0.0.1 'nixos-generate-config' '--show-hardware-config' '--no-filesystems'","event":"command_start","status_running":"generating hardware config","time":"2026-09-18T23:15:29+02:00","message":"command started"}` //nolint:lll // raw log fixture mirrors the real bootstrap line

	isoGenerateSuccessLine = `{"level":"info","xpath":"test/nixosConfigurations/test-vm/nixos-iso-vm","phase":"inspect","description":"generate config","command":"ssh -o LogLevel=ERROR -l root -p 10022 -i /tmp/ssh.key 127.0.0.1 'nixos-generate-config' '--show-hardware-config' '--no-filesystems'","event":"command_end","status":"success","time":"2026-09-18T23:15:29+02:00","message":"command finished"}` //nolint:lll // raw log fixture mirrors the real bootstrap line

	kexecGenerateStartLine = `{"level":"info","xpath":"test/nixosConfigurations/test-vm-kexec/kexec-vm","phase":"bootstrap","description":"generate config","command":"ssh -o LogLevel=ERROR -l root -p 10023 -i /tmp/ssh.key 127.0.0.1 'nixos-generate-config' '--show-hardware-config' '--no-filesystems'","event":"command_start","status_running":"generating hardware config","time":"2026-09-18T23:16:05+02:00","message":"command started"}` //nolint:lll // raw log fixture mirrors the real bootstrap line

	kexecGenerateSuccessLine = `{"level":"info","xpath":"test/nixosConfigurations/test-vm-kexec/kexec-vm","phase":"bootstrap","description":"generate config","command":"ssh -o LogLevel=ERROR -l root -p 10023 -i /tmp/ssh.key 127.0.0.1 'nixos-generate-config' '--show-hardware-config' '--no-filesystems'","event":"command_end","status":"success","time":"2026-09-18T23:16:05+02:00","message":"command finished"}` //nolint:lll // raw log fixture mirrors the real bootstrap line
)

// hardwareConfigSuccessLog is the positive fixture: both xpaths succeeded with
// shell-quoted argv, alongside started lines without status, an unrelated info
// entry, and non-JSON output.
func hardwareConfigSuccessLog() string {
	return strings.Join([]string{
		notJSONLine,
		kexecSkipLine,
		isoGenerateStartLine,
		isoGenerateSuccessLine,
		kexecGenerateStartLine,
		kexecGenerateSuccessLine,
	}, "\n")
}

func hardwareConfigKexecFailedStatusLog() string {
	return strings.Join([]string{
		isoGenerateSuccessLine,
		strings.Replace(kexecGenerateSuccessLine, `"status":"success"`, `"status":"failed"`, 1),
	}, "\n")
}

func hardwareConfigKexecMissingArgLog() string {
	return strings.Join([]string{
		isoGenerateSuccessLine,
		strings.Replace(kexecGenerateSuccessLine, " '--no-filesystems'", "", 1),
	}, "\n")
}

type hardwareConfigLogTestCase struct {
	name        string
	log         string
	wantErr     bool
	errContains []string
}

func hardwareConfigLogTestCases() []hardwareConfigLogTestCase {
	return []hardwareConfigLogTestCase{
		{
			name:        "both xpaths successful with quoted commands",
			log:         hardwareConfigSuccessLog(),
			wantErr:     false,
			errContains: nil,
		},
		{
			name: "kexec xpath missing",
			log: strings.Join([]string{
				notJSONLine,
				isoGenerateStartLine,
				isoGenerateSuccessLine,
			}, "\n"),
			wantErr:     true,
			errContains: []string{kexecXPathLiteral},
		},
		{
			name:        "kexec entry present but status failed",
			log:         hardwareConfigKexecFailedStatusLog(),
			wantErr:     true,
			errContains: []string{kexecXPathLiteral},
		},
		{
			name:        "kexec command missing --no-filesystems",
			log:         hardwareConfigKexecMissingArgLog(),
			wantErr:     true,
			errContains: []string{kexecXPathLiteral, nixosNoFilesystemsArg},
		},
	}
}

func TestVerifyHardwareConfigGenerationLogContent(t *testing.T) {
	t.Parallel()

	for _, testCase := range hardwareConfigLogTestCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := verifyHardwareConfigGenerationLogContent(testCase.log)

			if !testCase.wantErr {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				return
			}

			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			for _, want := range testCase.errContains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err.Error(), want)
				}
			}
		})
	}
}
