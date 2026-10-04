package main

import (
	"strings"
	"testing"
)

const (
	helloStorePath   = "/nix/store/w2y44l5pvh6m8qk0abcd1234efgh5678-hello-2.12.1/bin/hello"
	nixProbeSplitter = "\n---\n"

	// secretLandingWantContent stands in for the transferred fixture bytes:
	// the pure verifier takes the expected content as a parameter, so the
	// table pins the content-exact check with synthetic bytes.
	secretLandingWantContent = "landing-secret-content\n"
)

type nixProbeTestCase struct {
	name        string
	probe       string
	wantErr     bool
	errContains []string
}

// nixAbsentProbeTestCases covers the nix absence premise probe: both markers
// must report absence, and any sign of a preinstalled nix must fail.
func nixAbsentProbeTestCases() []nixProbeTestCase {
	return []nixProbeTestCase{
		{
			name:    "no nix binary and no /nix tree",
			probe:   nixBinAbsentMarker + "\n" + nixDirAbsentMarker,
			wantErr: false,
		},
		{
			name:        "nix binary present",
			probe:       nixBinPresentMarker + "\n" + nixDirAbsentMarker,
			wantErr:     true,
			errContains: []string{nixBinPresentMarker},
		},
		{
			name:        "nix tree present",
			probe:       nixBinAbsentMarker + "\n" + nixDirPresentMarker,
			wantErr:     true,
			errContains: []string{nixDirPresentMarker},
		},
		{
			name:        "both markers missing",
			probe:       "probe output without any marker",
			wantErr:     true,
			errContains: []string{nixBinAbsentMarker, nixDirAbsentMarker},
		},
	}
}

// nixInstalledProbeTestCases covers the nix-install bootstrap end state: the
// version section must carry the ok marker and the hello path must resolve
// into /nix/store.
func nixInstalledProbeTestCases() []nixProbeTestCase {
	return []nixProbeTestCase{
		{
			name:    "nix runs and hello resolves into the store",
			probe:   nixBinOKMarker + nixProbeSplitter + helloStorePath,
			wantErr: false,
		},
		{
			name:        "nix version probe failed",
			probe:       nixBinMissingMarker + nixProbeSplitter + helloStorePath,
			wantErr:     true,
			errContains: []string{nixBinOKMarker},
		},
		{
			name:        "hello not in the nix store",
			probe:       nixBinOKMarker + nixProbeSplitter + "/run/system-manager/sw/bin/hello",
			wantErr:     true,
			errContains: []string{nixStorePathPrefix},
		},
		{
			name:        "hello path empty",
			probe:       nixBinOKMarker + nixProbeSplitter,
			wantErr:     true,
			errContains: []string{nixStorePathPrefix},
		},
	}
}

// verifyNixInstalledProbe adapts the two-section probe layout to the single
// string the shared table runner feeds.
func verifyNixInstalledProbe(probe string) error {
	versionSection, helloSection, _ := strings.Cut(probe, nixProbeSplitter)

	return verifyNixInstalledOutput(versionSection, helloSection)
}

// secretLandingProbe builds the raw two-section probe output exactly as the
// remote command produces it: the plain section (raw cat bytes or the missing
// marker), the '---' separator line, then the marker line.
func secretLandingProbe(plainSection, mntMarker string) string {
	return plainSection + "---\n" + mntMarker + "\n"
}

// secretLandingProbeTestCases covers the nix-install secret landing end
// state: the content-exact copy must sit on the live root at the plain remote
// path and no copy may exist under the bootstrapping root.
func secretLandingProbeTestCases() []nixProbeTestCase {
	return []nixProbeTestCase{
		{
			name:    "secret on the live root only",
			probe:   secretLandingProbe(secretLandingWantContent, secretLandingMntAbsentMarker),
			wantErr: false,
		},
		{
			name:        "secret redirected under the bootstrapping root",
			probe:       secretLandingProbe(secretLandingPlainMissingMarker+"\n", secretLandingMntPresentMarker),
			wantErr:     true,
			errContains: []string{secretLandingMntPresentMarker},
		},
		{
			name:        "secret missing on the live root",
			probe:       secretLandingProbe(secretLandingPlainMissingMarker+"\n", secretLandingMntAbsentMarker),
			wantErr:     true,
			errContains: []string{secretLandingPlainMissingMarker},
		},
		{
			name:        "content mismatch on the live root",
			probe:       secretLandingProbe("wrong-content\n", secretLandingMntAbsentMarker),
			wantErr:     true,
			errContains: []string{"wrong-content"},
		},
		{
			name:        "mnt probe markers missing",
			probe:       secretLandingProbe(secretLandingWantContent, "probe output without any marker"),
			wantErr:     true,
			errContains: []string{secretLandingMntAbsentMarker},
		},
		{
			name:        "probe sections missing",
			probe:       "output without the separator line",
			wantErr:     true,
			errContains: []string{"secret landing probe output"},
		},
	}
}

// verifySecretLandingProbe adapts the full probe-output layout to the shared
// table runner, pinning the expected content the fixture provides at runtime.
func verifySecretLandingProbe(probe string) error {
	return verifySecretLandingOutput(probe, secretLandingWantContent)
}

func runNixProbeTestCases(t *testing.T, cases []nixProbeTestCase, verify func(string) error) {
	t.Helper()

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := verify(testCase.probe)

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

func TestVerifyNixAbsentOutput(t *testing.T) {
	t.Parallel()

	runNixProbeTestCases(t, nixAbsentProbeTestCases(), verifyNixAbsentOutput)
}

func TestVerifyNixInstalledOutput(t *testing.T) {
	t.Parallel()

	runNixProbeTestCases(t, nixInstalledProbeTestCases(), verifyNixInstalledProbe)
}

func TestVerifySecretLandingOutput(t *testing.T) {
	t.Parallel()

	runNixProbeTestCases(t, secretLandingProbeTestCases(), verifySecretLandingProbe)
}
