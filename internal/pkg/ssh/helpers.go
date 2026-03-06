package ssh

import (
	"net"
	"time"
)

func (sC *SSHClient) ReachabilityCheck(timeout time.Duration) bool {
	address := net.JoinHostPort(sC.Hostname, sC.PortString())

	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}

	_ = conn.Close()

	return true
}
