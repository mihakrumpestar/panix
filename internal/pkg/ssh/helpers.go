package ssh

import (
	"fmt"
	"net"
	"time"
)

type Executor interface {
	ExecLocal(description, statusIfRunning, statusIfFailed string, commandWithArgs []string) error
}

func (sC *SshClient) ReachabilityCheck(timeout time.Duration) bool {
	address := net.JoinHostPort(sC.Hostname, fmt.Sprintf("%d", sC.Port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (sC *SshClient) String() string {
	return fmt.Sprintf("%s@%s:%d", sC.Username, sC.Hostname, sC.Port)
}

// WaitForDisconnect waits up to 5min for the host to become unreachable
func (sC *SshClient) WaitForDisconnect(exc Executor, statusMsg string) error {
	return exc.ExecLocal(
		"wait for disconnect",
		statusMsg,
		"failed to wait for disconnect",
		[]string{"sh", "-c", fmt.Sprintf(
			`for i in $(seq 1 300); do if ! nc -zvw1 %s %d 2>/dev/null; then exit 0; fi; sleep 1; done; exit 1`,
			sC.Hostname, sC.Port,
		)},
	)
}

// WaitForReconnect waits up to 10min for the host to become reachable again
func (sC *SshClient) WaitForReconnect(exc Executor, statusMsg, failMsg string) error {
	return exc.ExecLocal(
		"wait for reconnect",
		statusMsg,
		failMsg,
		[]string{"sh", "-c", fmt.Sprintf(
			`for i in $(seq 1 300); do if nc -zvw1 %s %d 2>/dev/null; then exit 0; fi; sleep 2; done; exit 1`,
			sC.Hostname, sC.Port,
		)},
	)
}
