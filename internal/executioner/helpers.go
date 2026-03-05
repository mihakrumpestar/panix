package executioner

import (
	"fmt"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
)

const (
	// 5 min disconnect timeout
	WaitForDisconnectTimeout  = 300
	WaitForDisconnectInterval = time.Second

	// 10 min reconnect timeout
	WaitForReconnectTimeout      = 300
	WaitForReconnectCheckTimeout = 2 * time.Second
)

func (ex *Executioner) ExecuteHooks(hooks []config_attributes.PostBootstrapHookCommand, hookType string) error {
	for i, hook := range hooks {
		switch hook {
		case config_attributes.PostBootstrapHookWaitForOnline:
			activeSSH := ex.machine.MetaInspect.GetActiveSSH()
			err := WaitForReconnect(ex, activeSSH, fmt.Sprintf("waiting for %s to be online", hookType), fmt.Sprintf("%s did not come online", hookType))
			if err != nil {
				return err
			}
		case config_attributes.PostBootstrapHookWaitForOffline:
			activeSSH := ex.machine.MetaInspect.GetActiveSSH()
			err := WaitForDisconnect(ex, activeSSH, fmt.Sprintf("waiting for %s to go offline", hookType))
			if err != nil {
				return err
			}
		default:
			err := ex.Exec(
				fmt.Sprintf("%s %d", hookType, i+1),
				fmt.Sprintf("running %s: %s", hookType, hook),
				fmt.Sprintf("%s failed", hookType),
				[]string{string(hook)},
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func WaitForDisconnect(exc *Executioner, sshClient *ssh.SshClient, statusMsg string) error {
	return exc.ExecFn(
		"wait for disconnect",
		statusMsg,
		"failed to wait for disconnect",
		func() error {
			for i := 0; i < WaitForDisconnectTimeout; i++ {
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

func WaitForReconnect(exc *Executioner, sshClient *ssh.SshClient, statusMsg, failMsg string) error {
	return exc.ExecFn(
		"wait for reconnect",
		statusMsg,
		failMsg,
		func() error {
			for i := 0; i < WaitForReconnectTimeout; i++ {
				select {
				case <-exc.ctx.Done():
					return exc.ctx.Err()
				default:
					if sshClient.ReachabilityCheck(WaitForReconnectCheckTimeout) {
						return nil
					}
				}
			}
			return fmt.Errorf("host %s:%d did not reconnect within 10 minutes", sshClient.Hostname, sshClient.Port)
		},
	)
}
