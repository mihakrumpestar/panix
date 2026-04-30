package attributes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
)

func TestPassAttributesIntoMergesTags(t *testing.T) {
	t.Parallel()

	parent := &Attributes{
		Tags: []string{"parent-tag", "shared-tag"},
	}

	child := &Attributes{
		Tags: []string{"child-tag", "shared-tag"},
	}

	err := child.passAttributesInto("my-name", parent)
	require.NoError(t, err)

	assertion := assert.New(t)

	// mergo WithAppendSlice should append parent tags to child
	assertion.Contains(child.Tags, "child-tag")
	assertion.Contains(child.Tags, "shared-tag")
	assertion.Contains(child.Tags, "parent-tag")
	assertion.Contains(child.Tags, "my-name", "name should be added as a tag")
}

func TestPassAttributesIntoSetsNameAndXpath(t *testing.T) {
	t.Parallel()

	parent := &Attributes{
		Xpath: xpath.New("fleet").NewXpathWithAppend("my-flake"),
	}

	child := &Attributes{}

	err := child.passAttributesInto("my-config", parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal("my-config", child.Name)
	assertion.Equal("fleet/my-flake/my-config", child.Xpath.String())
}

func TestPassAttributesIntoMergesSSH(t *testing.T) {
	t.Parallel()

	parent := &Attributes{
		SSH: ssh.SSHClient{
			Hostname:     "parent-host",
			Port:         22,
			Username:     "parent-user",
			IdentityFile: "/home/user/.ssh/id_ed25519",
		},
	}

	child := &Attributes{
		SSH: ssh.SSHClient{
			Port: 2222,
		},
	}

	err := child.passAttributesInto("child-name", parent)
	require.NoError(t, err)

	assertion := assert.New(t)

	// Child's explicit port should be preserved, parent's hostname should merge in
	assertion.Equal(uint16(2222), child.SSH.Port)
	assertion.Equal("parent-host", child.SSH.Hostname,
		"child should inherit parent hostname when not set")
	assertion.Equal("parent-user", child.SSH.Username,
		"child should inherit parent username when not set")
}

func TestPassAttributesIntoMergesSecrets(t *testing.T) {
	t.Parallel()

	parent := &Attributes{
		Secrets: []PlainFileOrDirToTransfer{
			{LocalPath: "/etc/parent.key", RemotePath: "/etc/parent.key"},
		},
	}

	child := &Attributes{
		Secrets: []PlainFileOrDirToTransfer{
			{LocalPath: "/etc/child.key", RemotePath: "/etc/child.key"},
		},
	}

	err := child.passAttributesInto("child-name", parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Len(child.Secrets, 2, "child should have both parent and child secrets")
}

func TestPassAttributesIntoMergesSudoProgram(t *testing.T) {
	t.Parallel()

	parent := &Attributes{
		SudoProgram: SudoProgram("doas"),
	}

	child := &Attributes{}

	err := child.passAttributesInto("child-name", parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal(SudoProgram("doas"), child.SudoProgram,
		"child should inherit parent's sudo program")
}

func TestPassAttributesIntoChildOverridesParent(t *testing.T) {
	t.Parallel()

	parent := &Attributes{
		SudoProgram:    SudoProgram("doas"),
		ActivationMode: ActivationModeD(ActivationModeBoot),
	}

	child := &Attributes{
		SudoProgram: SudoProgram("run0"),
	}

	err := child.passAttributesInto("child-name", parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal(SudoProgram("run0"), child.SudoProgram,
		"child's explicit sudo program should override parent")
	assertion.Equal(ActivationModeD(ActivationModeBoot), child.ActivationMode,
		"child should inherit parent activation mode when not set")
}

func TestPassAttributesIntoEmptyNameNoNameTag(t *testing.T) {
	t.Parallel()

	parent := &Attributes{
		Tags:  []string{"parent-tag"},
		Xpath: xpath.New("fleet"),
	}

	child := &Attributes{}

	err := child.passAttributesInto("", parent)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Empty(child.Name, "name should be empty when not provided")
	assertion.NotContains(child.Tags, "", "empty string should not be added as tag")
	assertion.Equal("fleet", child.Xpath.String(),
		"xpath should inherit from parent when name is empty")
}

func TestNewAttributes(t *testing.T) {
	t.Parallel()

	attr := New()

	assertion := assert.New(t)
	assertion.NotNil(attr)
	assertion.Empty(attr.Tags)
	assertion.Empty(attr.Name)
}
