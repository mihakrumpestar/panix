package ssh

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaybeSSHCommandArguments_DisableStrictKeyChecking(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:                 "example.com",
		Port:                     22,
		Username:                 "root",
		DisableStrictKeyChecking: true,
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-o")
	assertion.Contains(args, "UserKnownHostsFile=/dev/null")
	assertion.Contains(args, "StrictHostKeyChecking=no")
}

func TestMaybeSSHCommandArguments_Defaults(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname: "example.com",
		Port:     22,
		Username: "root",
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-o")
	assertion.Contains(args, "StrictHostKeyChecking=accept-new")
	assertion.NotContains(args, "UserKnownHostsFile")
}

func TestMaybeSSHCommandArguments_KnownHostsFileWithAcceptNew(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:       "example.com",
		Port:           22,
		Username:       "root",
		KnownHostsFile: KnownHostsFile("/tmp/panix-knownhosts-test"),
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-o")
	assertion.Contains(args, "UserKnownHostsFile=/tmp/panix-knownhosts-test")
	assertion.Contains(args, "StrictHostKeyChecking=accept-new")
}

func TestMaybeSSHCommandArguments_KnownHostsFileStrictCheck(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:              "example.com",
		Port:                  22,
		Username:              "root",
		DisableAutoAddHostKey: true,
		KnownHostsFile:        KnownHostsFile("/tmp/panix-knownhosts-test"),
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-o")
	assertion.Contains(args, "UserKnownHostsFile=/tmp/panix-knownhosts-test")
	assertion.NotContains(args, "StrictHostKeyChecking")
}

func TestMaybeSSHCommandArguments_DisableStrictOverridesKnownHostsFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:                 "example.com",
		Port:                     22,
		Username:                 "root",
		DisableStrictKeyChecking: true,
		KnownHostsFile:           KnownHostsFile("/tmp/panix-knownhosts-test"),
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-o")
	assertion.Contains(args, "UserKnownHostsFile=/dev/null")
	assertion.Contains(args, "StrictHostKeyChecking=no")
	assertion.NotContains(args, "/tmp/panix-knownhosts-test")
}

func TestMaybeSSHCommandArguments_DisableAutoAddHostKeyWithoutKnownHostsFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:              "example.com",
		Port:                  22,
		Username:              "root",
		DisableAutoAddHostKey: true,
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.NotContains(args, "StrictHostKeyChecking")
	assertion.NotContains(args, "UserKnownHostsFile")
}

func TestKnownHostsFile_IsAuto(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	assertion.False(KnownHostsFile("").IsAuto())
	assertion.True(KnownHostsFile(os.TempDir() + "/panix-knownhosts-1234").IsAuto())
	assertion.False(KnownHostsFile("/home/user/.ssh/known_hosts").IsAuto())
}
