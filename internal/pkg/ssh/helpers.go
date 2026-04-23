package ssh

import (
	"context"
	"net"
	"time"
)

func (sC *SSHClient) ReachabilityCheck(timeout time.Duration) bool {
	address := net.JoinHostPort(sC.Hostname, sC.Port.String())

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	conn, err := dialer.DialContext(context.Background(), "tcp", address)
	if err != nil {
		return false
	}

	_ = conn.Close()

	return true
}
