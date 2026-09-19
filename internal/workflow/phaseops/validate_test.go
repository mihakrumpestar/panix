package phaseops

import (
	"os/user"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newValidatePair builds the minimal pair ValidateTargetUser needs: user, preset
// level, SSH username, MetaInspect IsRoot.
func newValidatePair(user string, systemLevel bool, sshUser string, isRoot bool) (*installable.Installable, *machine.Machine) {
	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	mach.SSH = ssh.SSHClient{Hostname: "10.0.0.1", Port: 22, Username: sshUser}
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: isRoot})

	systemLevelPtr := systemLevel
	inst := &installable.Installable{
		User: user,
	}
	inst.Preset = installable.Preset{IsSystemLevel: &systemLevelPtr}
	inst.Xpath = xpath.New("fleet", "flakes", "inst")

	return inst, mach
}

func TestValidateTargetUser(t *testing.T) {
	t.Parallel()

	for _, tt := range validateTargetUserCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inst, mach := newValidatePair(tt.user, tt.systemLevel, tt.sshUser, tt.isRoot)

			err := ValidateTargetUser(inst, mach)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.user)
				assert.Contains(t, err.Error(), tt.sshUser)
				// Discriminate the two branches: only the equality case
				// carries the unset-user guidance.
				if tt.wantUnsetUser {
					assert.Contains(t, err.Error(), "unset user")
				} else {
					assert.NotContains(t, err.Error(), "unset user")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

var validateTargetUserCases = []struct {
	name          string
	user          string
	systemLevel   bool
	sshUser       string
	isRoot        bool
	wantErr       bool
	wantUnsetUser bool
}{
	{
		name:    "no target user: always passes",
		wantErr: false,
	},
	{
		name:        "target user, root SSH user: passes",
		user:        "alice",
		systemLevel: false,
		sshUser:     "root",
		isRoot:      true,
		wantErr:     false,
	},
	{
		// su -l prompts even for the same non-root user (util-linux su has no
		// same-user bypass; only uid 0 skips auth), so this case must fail and
		// guide to unset user or SSH as root.
		name:          "target user equals SSH user: fails with unset-user guidance",
		user:          "alice",
		sshUser:       "alice",
		wantErr:       true,
		wantUnsetUser: true,
	},
	{
		name:    "target user, non-root SSH user, mismatch: fails",
		user:    "alice",
		sshUser: "deploy",
		wantErr: true,
	},
	{
		name:        "system-level target user, non-root SSH user, mismatch: fails",
		user:        "bob",
		systemLevel: true,
		sshUser:     "deploy",
		wantErr:     true,
	},
	{
		name:        "system-level user root, non-root SSH user: normalized, passes (sudo path)",
		user:        "root",
		systemLevel: true,
		sshUser:     "deploy",
		isRoot:      false,
		wantErr:     false,
	},
}

// TestValidateTargetUser_LocalUsesRunningUser pins the local rule: the SSH
// username is a config default (root) that says nothing about who runs panix, so
// validation compares against the real local user instead.
func TestValidateTargetUser_LocalUsesRunningUser(t *testing.T) {
	t.Parallel()

	current, err := user.Current()
	require.NoError(t, err)

	// Local machine with the default root SSH username.
	client := &ssh.SSHClient{Hostname: "local-test"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	mach.SSH = *client
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: false})

	systemLevel := false
	inst := &installable.Installable{User: "alice"}
	inst.Preset = installable.Preset{IsSystemLevel: &systemLevel}
	inst.Xpath = xpath.New("fleet", "flakes", "inst")

	// Local user is neither target nor root: must fail naming the RUNNING
	// user, not the config default "root".
	err = ValidateTargetUser(inst, mach)
	require.Error(t, err)
	assert.Contains(t, err.Error(), current.Username)

	// Target user is the running local user: still fails (su -l would prompt),
	// with the unset-user guidance.
	inst.User = current.Username
	err = ValidateTargetUser(inst, mach)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unset user")
}
