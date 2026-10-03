package executioner

import (
	"context"
	"io"
	"time"
)

// stdinPumpDeadline is the reader capability the pump uses to unblock a
// pending Read when the exec ends: os.Pipe files and net conns implement it,
// while in-memory readers never block and do not need it.
type stdinPumpDeadline interface {
	SetReadDeadline(time.Time) error
}

// startStdinPump spawns the stdin pump for one exec: it copies src into dst
// (the PTY master) until src ends, the context is canceled or a write fails.
// The returned channel closes once the pump goroutine has exited, which gives
// tests a deterministic handle on the pump lifecycle.
//
// A companion goroutine unblocks a pending src.Read when the context ends, so
// an exec that finishes while the pump waits on its reader leaves no
// goroutine behind for deadline-capable readers.
func startStdinPump(ctx context.Context, dst io.Writer, src io.Reader) <-chan struct{} {
	done := make(chan struct{})

	go pumpStdin(ctx, dst, src, done)
	go unblockStdinPumpOnCancel(ctx, src, done)

	return done
}

// pumpStdin is the pump loop. Every read chunk is written to dst in order,
// which is the existing Pty.Write path onto the PTY master, so the bytes
// reach the ssh child's stdin and are forwarded to the remote exec channel.
//
// The pump is best-effort transport: a write failure only ends the pump and
// is swallowed, it never reaches the exec outcome or the command log. Any
// read error, io.EOF included, ends the pump too; the reader belongs to the
// caller, which observes its own errors.
func pumpStdin(ctx context.Context, dst io.Writer, src io.Reader, done chan<- struct{}) {
	defer close(done)

	buf := make([]byte, ptyBufferSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		bytesRead, readErr := src.Read(buf)
		if bytesRead > 0 {
			_, writeErr := dst.Write(buf[:bytesRead])
			if writeErr != nil {
				return
			}
		}

		if readErr != nil {
			return
		}
	}
}

// unblockStdinPumpOnCancel releases a pump blocked in src.Read when the exec
// ends: for deadline-capable readers an already expired deadline unblocks
// the pending Read and every future one, which the pump treats as end of
// stream. Readers without deadline support are waited out instead: they must
// terminate on their own (in-memory readers never block, io.Pipe must be
// closed by its owner).
func unblockStdinPumpOnCancel(ctx context.Context, src io.Reader, done <-chan struct{}) {
	deadlineSetter, canSetDeadline := src.(stdinPumpDeadline)
	if !canSetDeadline {
		<-done

		return
	}

	select {
	case <-ctx.Done():
		_ = deadlineSetter.SetReadDeadline(time.Now())
	case <-done:
	}
}
