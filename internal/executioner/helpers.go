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

const (
	WaitForDisconnectTimeoutTimes = 300 // 5 min disconnect timeout
	WaitForDisconnectInterval     = time.Second

	WaitForReconnectTimeoutTimes  = 600 // 10 min reconnect timeout
	WaitForReconnectCheckInterval = time.Second
)

func (ex *Executioner) ExecuteHooks(hooks []attributes.PostBootstrapHookCommand, hookType string) error {
	for idx, hook := range hooks {
		switch hook {
		case attributes.PostBootstrapHookWaitForOnline:
			activeSSH := ex.machine.GetActiveSSH()

			err := WaitForReconnect(ex, activeSSH, fmt.Sprintf("waiting for %s to be online", hookType), hookType+" did not come online")
			if err != nil {
				return err
			}
		case attributes.PostBootstrapHookWaitForOffline:
			activeSSH := ex.machine.GetActiveSSH()

			err := WaitForDisconnect(ex, activeSSH, fmt.Sprintf("waiting for %s to go offline", hookType))
			if err != nil {
				return err
			}
		default:
			err := ex.Exec(
				fmt.Sprintf("%s %d", hookType, idx+1),
				fmt.Sprintf("running %s: %s", hookType, hook),
				hookType+" failed",
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
			for range WaitForDisconnectTimeoutTimes {
				select {
				case <-exc.ctx.Done():
					return exc.ctx.Err()
				default:
					if !sshClient.ReachabilityCheck(time.Second) {
						return nil
					}

					time.Sleep(WaitForDisconnectInterval)
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
			for range WaitForReconnectTimeoutTimes {
				select {
				case <-exc.ctx.Done():
					return exc.ctx.Err()
				default:
					if sshClient.ReachabilityCheck(time.Second) {
						return nil
					}

					time.Sleep(WaitForDisconnectInterval)
				}
			}

			return errors.Wrap(ErrHostReconnectTimeout, sshClient.HostPortString())
		},
	)
}
