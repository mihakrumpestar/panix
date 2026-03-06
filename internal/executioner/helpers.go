package executioner

import (
	"fmt"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
)

const (
	// 5 min disconnect timeout
	WaitForDisconnectTimeoutTimes = 300
	WaitForDisconnectInterval     = time.Second

	// 10 min reconnect timeout
	WaitForReconnectTimeoutTimes  = 300
	WaitForReconnectCheckInterval = 2 * time.Second
)

func (ex *Executioner) ExecuteHooks(hooks []attributes.PostBootstrapHookCommand, hookType string) error {
	for idx, hook := range hooks {
		switch hook {
		case attributes.PostBootstrapHookWaitForOnline:
			activeSSH := ex.machine.MetaInspect.GetActiveSSH()
			err := WaitForReconnect(ex, activeSSH, fmt.Sprintf("waiting for %s to be online", hookType), fmt.Sprintf("%s did not come online", hookType))

			if err != nil {
				return err
			}
		case attributes.PostBootstrapHookWaitForOffline:
			activeSSH := ex.machine.MetaInspect.GetActiveSSH()
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

func WaitForDisconnect(exc *Executioner, sshClient *ssh.SSHClient, statusMsg string) error {
	return exc.ExecFn(
		"wait for disconnect",
		statusMsg,
		"failed to wait for disconnect",
		func() error {
			for i := 0; i < WaitForDisconnectTimeoutTimes; i++ {
				select {
				case <-exc.ctx.Done():
					return exc.ctx.Err()
				default:
					if !sshClient.ReachabilityCheck(WaitForDisconnectInterval) {
						return nil
					}

					time.Sleep(WaitForDisconnectInterval)
				}
			}

			return fmt.Errorf("host %s:%d did not disconnect within 5 minutes", sshClient.Hostname, sshClient.Port)
		},
	)
}

func WaitForReconnect(exc *Executioner, sshClient *ssh.SSHClient, statusMsg, failMsg string) error {
	return exc.ExecFn(
		"wait for reconnect",
		statusMsg,
		failMsg,
		func() error {
			for i := 0; i < WaitForReconnectTimeoutTimes; i++ {
				select {
				case <-exc.ctx.Done():
					return exc.ctx.Err()
				default:
					if sshClient.ReachabilityCheck(WaitForReconnectCheckInterval) {
						return nil
					}
				}
			}
			return fmt.Errorf("host %s:%d did not reconnect within 10 minutes", sshClient.Hostname, sshClient.Port)
		},
	)
}
