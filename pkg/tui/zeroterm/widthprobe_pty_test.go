//go:build linux || darwin || freebsd

package zeroterm

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"github.com/mihakrumpestar/panix/pkg/pty"
)

// TestProbeGlyphWidths_RealPTY pins the end-to-end path over a real PTY: the
// fake terminal reports font gap glyph U+1FA9C as one cell while the static table
// says two, so the probe must return the measured width. It also pins the batched
// protocol: one write wrapped in a balanced synchronized output pair.
func TestProbeGlyphWidths_RealPTY(t *testing.T) {
	t.Parallel()

	pseudo, err := pty.New()
	require.NoError(t, err)

	defer func() { require.NoError(t, pseudo.Close()) }()

	slave, err := os.OpenFile(pseudo.Name(), os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)

	defer func() { require.NoError(t, slave.Close()) }()

	_, err = term.MakeRaw(int(slave.Fd()))
	require.NoError(t, err)

	// Terminal side: U+1FA9C renders one cell wide (font gap), so the CPR
	// reports column two after it is written.
	widths := map[string]int{"a": 1, "\u6f22": 2, "\U0001FA9C": 1}

	go serveSlaveWidthProbe(slave, widths)

	// Program side: a dup of the master fd, so closing it does not race p.Close.
	masterFd, err := unix.Dup(int(pseudo.Fd()))
	require.NoError(t, err)

	master := os.NewFile(uintptr(masterFd), "pty-master")
	require.NotNil(t, master)

	defer func() { require.NoError(t, master.Close()) }()

	writer := &probeChunkWriter{dst: master}

	terminal := &Terminal{in: master, out: master, probeOut: writer}

	measured, _, err := terminal.ProbeGlyphWidths([]string{"\U0001FA9C"})
	require.NoError(t, err)
	assert.Equal(t, 1, measured["\U0001FA9C"],
		"font gap glyph must measure the terminal width, not the static two")

	assert.Equal(t, 1, writer.count(), "the whole probe must be issued in one write")

	emitted := writer.all()
	assertBalancedSync(t, emitted)
	assert.Contains(t, string(emitted), "\r\U0001FA9C"+probeCPRRequest)
}

// serveSlaveWidthProbe reads probe requests from the PTY slave and answers them
// with the fake terminal's own width table.
func serveSlaveWidthProbe(slave *os.File, widths map[string]int) {
	var acc []byte

	buf := make([]byte, 1024) //nolint:mnd // test read buffer

	for {
		n, err := slave.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			acc = respondToSlaveProbe(acc, widths, slave)
		}

		if err != nil {
			return
		}
	}
}

// respondToSlaveProbe answers every complete cursor position request in acc.
func respondToSlaveProbe(acc []byte, widths map[string]int, slave *os.File) []byte {
	for {
		idx := bytes.Index(acc, []byte(probeCPRRequest))
		if idx < 0 {
			return acc
		}

		start := bytes.LastIndexByte(acc[:idx], '\r')
		if start < 0 {
			start = 0
		} else {
			start++
		}

		width, ok := widths[string(acc[start:idx])]
		if !ok {
			width = 1
		}

		_, _ = slave.Write(fmt.Appendf(nil, "\x1b[1;%dR", width+1))

		acc = acc[idx+len(probeCPRRequest):]
	}
}
