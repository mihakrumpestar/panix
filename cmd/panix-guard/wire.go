package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

// Guardian-side wire transport (spec 6.1, 8): the event pipe carries record
// lines out to the relay's stdout; the command pipe carries wire frames in.
// The state machine never blocks on the wire: records dual-write (log first,
// then a non-blocking enqueue on a single writer goroutine with a bounded
// drop-oldest buffer), and drops and link failures surface as log records.

const (
	// wireBufferFrames bounds the drop-oldest wire buffer (spec 6.1).
	wireBufferFrames = 256
	// wireFlushGrace bounds the terminal drain wait: terminal records are
	// already durable in the log when close runs, so a wedged pipe only
	// delays the exit, never the outcome.
	wireFlushGrace = 2 * time.Second
	// wireLineCap bounds one command line: a hostile or desynchronized
	// stream longer than this is noise, not a frame.
	wireLineCap = 64 << 10
)

// wireCommandFrame is the shape of the frames panix writes on the command
// pipe (the deployer-side contract in the phaseops guard package).
type wireCommandFrame struct {
	C   string `json:"c"`
	Rid int64  `json:"rid"`
}

// wireWriter serializes record lines onto the event pipe from a single
// goroutine (spec 6.1): send never blocks the state machine; a full buffer
// drops the oldest frame and reports the drop; a write failure marks the
// link down once. Both reports go to the LOG only (callbacks), never back
// onto the dying channel.
type wireWriter struct {
	frames chan string
	w      io.Writer

	done     chan struct{} // closed by close() to start the drain
	closed   chan struct{} // closed by the goroutine after the drain
	downOnce sync.Once
	down     bool // goroutine-owned: set on the first write failure

	hooksMu sync.Mutex
	onDrop  func()
	onDown  func(error)
}

// setHooks installs the log-only report callbacks (LINK_DEGRADED on drops,
// LINK_DOWN on link failure). The hooks are mutex-guarded because the writer
// goroutine can observe a drop the instant it starts.
func (ww *wireWriter) setHooks(onDrop func(), onDown func(error)) {
	ww.hooksMu.Lock()
	ww.onDrop, ww.onDown = onDrop, onDown
	ww.hooksMu.Unlock()
}

// dropping reports a dropped frame to the hook, if one is installed.
func (ww *wireWriter) dropping() {
	ww.hooksMu.Lock()
	hook := ww.onDrop
	ww.hooksMu.Unlock()

	if hook != nil {
		hook()
	}
}

// failed reports the link-down transition to the hook, if one is installed.
func (ww *wireWriter) failed(err error) {
	ww.hooksMu.Lock()
	hook := ww.onDown
	ww.hooksMu.Unlock()

	if hook != nil {
		hook(err)
	}
}

// newWireWriter starts the single writer goroutine. w may be nil in tests
// (send then behaves as a bounded sink with the same drop accounting).
func newWireWriter(w io.Writer, onDrop func(), onDown func(error)) *wireWriter {
	ww := &wireWriter{
		frames: make(chan string, wireBufferFrames),
		w:      w,
		done:   make(chan struct{}),
		closed: make(chan struct{}),
	}
	ww.setHooks(onDrop, onDown)

	go ww.loop()

	return ww
}

// send enqueues one record line (newline added here). Never blocks: a full
// buffer drops the oldest frame (drop-oldest, spec 6.1); a dead link drops
// silently (LINK_DOWN already logged once).
func (ww *wireWriter) send(line string) {
	if ww.down {
		return
	}

	select {
	case ww.frames <- line + "\n":
	default:
		// Drop-oldest: make room, then retry once. Each failed re-queue
		// after the eviction counts as one dropped frame of its own.
		select {
		case <-ww.frames:
		default:
		}

		ww.dropping()

		select {
		case ww.frames <- line + "\n":
		default:
			ww.dropping()
		}
	}
}

// close stops the writer: the drain delivers every buffered frame (bounded
// by wireFlushGrace on a wedged pipe) before the goroutine exits.
func (ww *wireWriter) close() {
	close(ww.done)

	select {
	case <-ww.closed:
	case <-time.After(wireFlushGrace):
	}
}

// fail marks the link down exactly once; later drops are silent.
func (ww *wireWriter) fail(err error) {
	ww.downOnce.Do(func() {
		ww.down = true
		ww.failed(err)
	})
}

// loop is the single writer goroutine: frames in arrival order, then the
// terminal drain.
func (ww *wireWriter) loop() {
	defer close(ww.closed)

	for {
		select {
		case frame := <-ww.frames:
			ww.write(frame)
		case <-ww.done:
			for {
				select {
				case frame := <-ww.frames:
					ww.write(frame)
				default:
					return
				}
			}
		}
	}
}

// write delivers one frame; a failure marks the link down (EPIPE/EOF, spec
// 6.1) and every later frame is dropped silently.
func (ww *wireWriter) write(frame string) {
	if ww.down || ww.w == nil {
		return
	}

	if _, err := io.WriteString(ww.w, frame); err != nil {
		ww.fail(err)
	}
}

// readWireCommands drains the command pipe (spec 6.1: from before HELLO).
// Valid frames become state-machine inputs in arrival order; unknown or
// malformed lines get an ack-shaped record via onBad and the drain never
// stops early. The returned error is the terminal read failure (EOF or a
// hard error), which the caller logs as LINK_DOWN.
func readWireCommands(r io.Reader, onCommand func(command string, rid int64), onBad func(reason string)) error {
	reader := bufio.NewReader(r)

	for {
		line, err := readWireLine(reader)
		if err != nil {
			if errors.Is(err, io.EOF) && line == "" {
				return nil // clean EOF: the relay is gone
			}

			if errors.Is(err, io.EOF) {
				onBad("command frame truncated") // trailing bytes then EOF

				return nil
			}

			return err
		}

		if line == "" {
			continue
		}

		var frame wireCommandFrame

		jerr := json.Unmarshal([]byte(line), &frame)
		if jerr != nil || frame.C == "" {
			onBad("malformed command frame")

			continue
		}

		if frame.C != "confirm" && frame.C != "revert" {
			onBad("unknown command")

			continue
		}

		onCommand(frame.C, frame.Rid)
	}
}

// readWireLine reads one newline-terminated command line with a size cap.
// An overlong line is consumed and reported as oversized; a final segment
// without a newline surfaces with io.EOF so the caller can treat it as a
// torn frame.
func readWireLine(r *bufio.Reader) (string, error) {
	var buf []byte

	for {
		chunk, err := r.ReadByte()
		if err != nil {
			if len(buf) == 0 {
				return "", err // clean EOF or hard read error
			}

			return string(buf), err // torn trailing frame
		}

		if chunk == '\n' {
			return string(buf), nil
		}

		buf = append(buf, chunk)

		if len(buf) > wireLineCap {
			// Consume to the next newline (or EOF) so the stream stays in
			// frame sync, then report the oversized line as noise.
			for {
				b, berr := r.ReadByte()
				if berr != nil || b == '\n' {
					break
				}
			}

			return "", errOversizedFrame
		}
	}
}

// errOversizedFrame marks a command line past the size cap.
var errOversizedFrame = errors.New("command frame too large")

// wireDownReason renders the LINK_DOWN record reason for a terminal wire
// failure (spec 6.1: EOF and EPIPE both mean the relay side is gone).
func wireDownReason(err error) string {
	if err == nil {
		return "event pipe write failed"
	}

	return "event pipe write failed: " + err.Error()
}
