package ssh

import (
	"fmt"
	"net"
	"time"
)

func (sC *SshClient) ReachabilityCheck(timeout time.Duration) bool {
	address := net.JoinHostPort(sC.Hostname, fmt.Sprintf("%d", sC.Port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()

	return true
}
