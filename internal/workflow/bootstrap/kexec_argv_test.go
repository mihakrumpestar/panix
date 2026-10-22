package bootstrap

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/stringbyte"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newKexecMachine builds a LOCAL machine with a controlled runtime IsRoot,
// so recorded argv shows privilege prefixes directly.
func newKexecMachine(t *testing.T, isRoot bool) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	client := &ssh.SSHClient{Hostname: "local-test", Username: "kexec-user"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: isRoot})

	return mach
}

// The staging dir must be owned by the ACTIVE connection's user (bootstrap
// during kexec staging), not the regular SSH user, which may differ.
func TestCreateKexecDirectory_BootstrapSSHUser(t *testing.T) {
	t.Parallel()

	mach := newKexecMachine(t, false)

	bootstrapClient := &ssh.SSHClient{Hostname: "10.0.0.9", Port: 22, Username: "bootstrap-admin"}
	require.NoError(t, bootstrapClient.Init("bootstrap-host", "", nixver.Info{}))
	mach.Bootstrap.SSH = *bootstrapClient
	mach.State.Store(&machine.State{ActiveSSH: machine.SSHTypeBootstrap})

	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	stagingDir, err := createKexecDirectory(exc, mach)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(stagingDir, "/tmp/panix-kexec-"), stagingDir)

	// Assert usernames: the active connection's user owns the staging dir.
	line := testutil.LastCommandLine(t, phaseLog)
	assert.Contains(t, line, "install")
	assert.Contains(t, line, "-d")
	assert.Contains(t, line, "-m")
	assert.Contains(t, line, "700")
	assert.Contains(t, line, "bootstrap-admin")
	assert.Contains(t, line, stagingDir)
	assert.NotContains(t, line, "kexec-user")
	assert.NotContains(t, line, "/tmp/kexec")
}

// Each run creates a fresh elevated dir owned by the SSH user (mode 700);
// only the kexec run itself elevates beyond this.
func TestCreateKexecDirectory_Elevation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		isRoot bool
		want   string
	}{
		{
			name:   "non-root SSH user: elevated install, SSH-user-owned, mode 700",
			isRoot: false,
			want:   "sudo install -d -m 700 -o kexec-user ",
		},
		{
			name:   "root SSH user: bare install still owned by the SSH user",
			isRoot: true,
			want:   "install -d -m 700 -o kexec-user ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachine(t, tt.isRoot)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

			stagingDir, err := createKexecDirectory(exc, mach)
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(stagingDir, "/tmp/panix-kexec-"), stagingDir)

			lines := testutil.CommandLines(t, phaseLog)
			require.Len(t, lines, 1)
			assert.Equal(t, tt.want+stagingDir, lines[0])
			assert.NotContains(t, lines[0], "/tmp/kexec")
		})
	}
}

// The tarball lands inside the per-run staging dir.
func TestDownloadOrTransferKexec_UsesStagingDir(t *testing.T) {
	t.Parallel()

	mach := newKexecMachine(t, false)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	const stagingDir = "/tmp/panix-kexec-test0123456789"

	require.NoError(t, downloadOrTransferKexec(exc, mach, "https://example.com/file.tar.gz", stagingDir))

	assert.Equal(t, "curl --fail -# -L --proto =https --proto-redir =https https://example.com/file.tar.gz -o "+stagingDir+"/kexec.tar",
		testutil.LastCommandLine(t, phaseLog))
}

// No elevation: the staging dir is SSH-user-owned, so tar needs no prefix.
func TestExtractKexecTarball_NoElevation(t *testing.T) {
	t.Parallel()

	mach := newKexecMachine(t, false)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	const stagingDir = "/tmp/panix-kexec-test0123456789"

	require.NoError(t, extractKexecTarball(exc, "https://example.com/file.tar.gz", stagingDir))

	assert.Equal(t,
		"tar -xzf "+stagingDir+"/kexec.tar -C "+stagingDir,
		testutil.LastCommandLine(t, phaseLog))
}

// The staged run script is the one elevated step: kexec needs root.
func TestRunKexecCommand_Elevation(t *testing.T) {
	t.Parallel()

	const stagingDir = "/tmp/panix-kexec-test0123456789"

	tests := []struct {
		name   string
		isRoot bool
		want   string
	}{
		{"non-root SSH user: sudo prefix", false, "sudo " + stagingDir + "/kexec/run"},
		{"root SSH user: bare", true, stagingDir + "/kexec/run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachine(t, tt.isRoot)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

			require.NoError(t, runKexecCommand(exc, mach, stagingDir))

			assert.Equal(t, tt.want, testutil.LastCommandLine(t, phaseLog))
		})
	}
}

// newKexecMachineWithArch builds a local machine with the given probed
// architecture, as resolveKexecURL sees it after inspection.
func newKexecMachineWithArch(t *testing.T, arch string) *machine.Machine {
	t.Helper()

	mach := newKexecMachine(t, false)
	mach.MetaInspect.Store(&machine.MetaInspect{Architecture: stringbyte.StringByte(arch)})

	return mach
}

// The kexec image is a plain argv value, so only PANIX_* variables are
// expanded; the dry-run pseudo-architecture maps to x86_64.
func TestResolveKexecURL_Expansion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		arch  string
		image attributes.KexecImage
		want  string
	}{
		{
			name:  "dollar form expanded from detected architecture",
			arch:  "aarch64",
			image: attributes.KexecImage("./kexec-$PANIX_ARCH.tar.gz"),
			want:  "./kexec-aarch64.tar.gz",
		},
		{
			name:  "brace form expanded",
			arch:  "x86_64",
			image: attributes.KexecImage("https://example.com/${PANIX_ARCH}/kexec.tar.gz"),
			want:  "https://example.com/x86_64/kexec.tar.gz",
		},
		{
			name:  "dry run architecture maps to x86_64",
			arch:  "DRY_RUN",
			image: attributes.KexecImage("./kexec-$PANIX_ARCH.tar.gz"),
			want:  "./kexec-x86_64.tar.gz",
		},
		{
			name:  "default image is a $PANIX_ARCH template",
			arch:  "aarch64",
			image: attributes.KexecImage(""),
			want:  strings.ReplaceAll(attributes.KexecImage("").String(), "$PANIX_ARCH", "aarch64"),
		},
		{
			name:  "foreign references stay untouched",
			arch:  "x86_64",
			image: attributes.KexecImage("https://example.com/$USER/kexec-$PANIX_ARCH.tar.gz"),
			want:  "https://example.com/$USER/kexec-x86_64.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachineWithArch(t, tt.arch)
			mach.Bootstrap.Kexec.Image = tt.image

			got, err := resolveKexecURL(mach)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestResolveKexecURL_Errors pins the hard failures: runtime variable mistakes
// surface with the kexec image context.
func TestResolveKexecURL_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		image   attributes.KexecImage
		wantErr []string
	}{
		{
			name:    "unknown panix variable",
			image:   attributes.KexecImage("./kexec-$PANIX_NOPE.tar.gz"),
			wantErr: []string{"invalid kexec image", "unknown panix runtime variable PANIX_NOPE"},
		},
		{
			name:    "known variable unavailable in kexec context",
			image:   attributes.KexecImage("./kexec-$PANIX_SECRET_LOCAL_PATH.tar.gz"),
			wantErr: []string{"invalid kexec image", "panix runtime variable PANIX_SECRET_LOCAL_PATH is not available in this context"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachineWithArch(t, "x86_64")
			mach.Bootstrap.Kexec.Image = tt.image

			got, err := resolveKexecURL(mach)
			require.Error(t, err)
			assert.Empty(t, got)

			for _, want := range tt.wantErr {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}
