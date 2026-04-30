package ssh

import (
	"context"
	"net"
	"time"
)

func (sC *SSHClient) ReachabilityCheck(timeout time.Duration) bool {
	address := net.JoinHostPort(sC.Hostname, sC.PortString())

	deadline := time.Now().Add(timeout)

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	conn, err := dialer.DialContext(context.Background(), "tcp", address)
	if err != nil {
		return false
	}

	remaining := time.Until(deadline)
	if remaining <= 0 {
		_ = conn.Close()

		return false
	}

	// Read 1 byte to confirm an SSH daemon is listening.
	// QEMU SLIRP accepts TCP connections even when the guest SSH daemon is down,
	// so a successful TCP dial alone doesn't confirm SSH is running.
	// A real SSH daemon sends its banner ("SSH-2.0-...") immediately upon connection.
	_ = conn.SetReadDeadline(time.Now().Add(remaining))

	buf := make([]byte, 1)
	_, err = conn.Read(buf)

	_ = conn.Close()

	return err == nil
}
