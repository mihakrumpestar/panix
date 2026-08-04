package executioner

import (
	"fmt"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	logs_command "github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/pkg/errors"
)

var (
	ErrHostDisconnectTimeout = errors.New("host did not disconnect within timeout")
	ErrHostReconnectTimeout  = errors.New("host did not reconnect within timeout")
)

// waitPollInterval is the polling interval used by WaitForDisconnect and
// WaitForReconnect when checking host reachability.
const waitPollInterval = time.Second

// waitPollReachabilityTimeout is the TCP dial timeout for each reachability
// probe inside the disconnect/reconnect wait loops. This is intentionally
// shorter than the overall wait timeout, it bounds a single poll attempt.
const waitPollReachabilityTimeout = time.Second

func (ex *Executioner) ExecuteHooks(hooks []attributes.PostBootstrapHookCommand, hookName string) error {
	machine := ex.conf.Machine

	for idx, hook := range hooks {
		switch hook {
		case attributes.PostBootstrapHookWaitForOnline:
			activeSSH := machine.GetActiveSSH()

			err := WaitForReconnect(ex, activeSSH, fmt.Sprintf("waiting for %s to be online", hookName), hookName+" did not come online")
			if err != nil {
				return err
			}
		case attributes.PostBootstrapHookWaitForOffline:
			activeSSH := machine.GetActiveSSH()

			err := WaitForDisconnect(ex, activeSSH, fmt.Sprintf("waiting for %s to go offline", hookName))
			if err != nil {
				return err
			}
		default:
			err := ex.Exec(
				fmt.Sprintf("%s %d", hookName, idx+1),
				fmt.Sprintf("running %s: %s", hookName, hook),
				hookName+" failed",
				[]string{string(hook)},
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func WaitForDisconnect(exc *Executioner, sshClient ssh.SSHClient, statusMsg string) error {
	return exc.ExecFn(
		"wait for disconnect",
		statusMsg,
		"failed to wait for disconnect",
		func(_ *logs_command.CommandLog) error {
			deadline := time.Now().Add(exc.conf.Timeouts.Disconnect)
			for time.Now().Before(deadline) {
				select {
				case <-exc.conf.Ctx.Done():
					return exc.conf.Ctx.Err()
				default:
					if !sshClient.ReachabilityCheck(waitPollReachabilityTimeout) {
						return nil
					}

					time.Sleep(waitPollInterval)
				}
			}

			return errors.Wrap(ErrHostDisconnectTimeout, sshClient.HostPortString())
		},
	)
}

func WaitForReconnect(exc *Executioner, sshClient ssh.SSHClient, statusMsg, failMsg string) error {
	return exc.ExecFn(
		"wait for reconnect",
		statusMsg,
		failMsg,
		func(_ *logs_command.CommandLog) error {
			deadline := time.Now().Add(exc.conf.Timeouts.Reconnect)
			for time.Now().Before(deadline) {
				select {
				case <-exc.conf.Ctx.Done():
					return exc.conf.Ctx.Err()
				default:
					if sshClient.ReachabilityCheck(waitPollReachabilityTimeout) {
						return nil
					}

					time.Sleep(waitPollInterval)
				}
			}

			return errors.Wrap(ErrHostReconnectTimeout, sshClient.HostPortString())
		},
	)
}
