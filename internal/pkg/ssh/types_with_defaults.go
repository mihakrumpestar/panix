package ssh

import "strconv"

// SSHPort

const (
	SSHPortDefault = 22
)

type SSHPort uint16 //nolint:recvcheck

func (s SSHPort) Get() uint16 {
	if s == 0 {
		return SSHPortDefault
	}

	return uint16(s)
}

func (s *SSHPort) Set(port uint16) {
	*s = SSHPort(port)
}

func (s SSHPort) String() string {
	return strconv.Itoa(int(s.Get()))
}

// SSHUsername

type SSHUsername string //nolint:recvcheck

func (s SSHUsername) Get() string {
	if s == "" {
		return "root"
	}

	return string(s)
}

func (s *SSHUsername) Set(username string) {
	*s = SSHUsername(username)
}
