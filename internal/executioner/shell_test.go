package executioner

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessTerminalOutput_BasicWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"first output", "hello", []string{"hello"}},
		{"trailing newline", "hello\r\n", []string{"hello"}},
		{"CRLF output", "line1\r\nline2\r\nline3", []string{"line1", "line2", "line3"}},
		{"CRLF trailing", "line1\r\nline2\r\n", []string{"line1", "line2"}},
		{"bare newline", "a\nb\nc", []string{"a", "b", "c"}},
		{"bare newline trailing", "a\nb\nc\n", []string{"a", "b", "c"}},
		{"intentional empty line", "hello\n\nworld\n", []string{"hello", "", "world"}},
		{"intentional empty CRLF", "hello\r\n\r\nworld\r\n", []string{"hello", "", "world"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cmdLog := newTestCommandLog()
			processTestData([]byte(test.input), cmdLog)
			assertLines(t, cmdLog, test.want)
		})
	}
}

func TestProcessTerminalOutput_CarriageReturn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"overwrite", "old\rnew", []string{"new"}},
		{"progress bar", "10%\r50%\r100%", []string{"100%"}},
		{"progress then newline", "[===   ]\r[======]\r[=========]\r\n", []string{"[=========]"}},
		{"CR then content", "\rnew", []string{"new"}},
		{"trailing CR", "hello\r", []string{"hello"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cmdLog := newTestCommandLog()
			processTestData([]byte(test.input), cmdLog)
			assertLines(t, cmdLog, test.want)
		})
	}
}

func TestProcessTerminalOutput_MultiRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		reads []string
		want  []string
	}{
		{"two reads separate lines", []string{"hello\r\n", "world\r\n"}, []string{"hello", "world"}},
		{"three reads", []string{"line1\r\n", "line2\r\n", "line3\r\n"}, []string{"line1", "line2", "line3"}},
		{"append then new line", []string{"hello", " world\r\n"}, []string{"hello world"}},
		{"nix output then next", []string{"\x1b[0m/nix/store/abc\r\n", "Checking...\r\n"}, []string{"\x1b[0m/nix/store/abc", "Checking..."}},
		{"progress then done", []string{"[===   ]\r[======]\r[=========]\r\n", "Done!\r\n"}, []string{"[=========]", "Done!"}},
		{"multi-read intentional empty", []string{"hello\r\n", "\r\n", "world\r\n"}, []string{"hello", "", "world"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cmdLog := newTestCommandLog()
			for _, read := range test.reads {
				processTestData([]byte(read), cmdLog)
			}

			assertLines(t, cmdLog, test.want)
		})
	}
}

func TestProcessTerminalOutput_PendingNewline(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()

	processTestData([]byte("hello\r\n"), cmdLog)
	assert.True(t, cmdLog.PendingNewline, "PendingNewline should be set after trailing \\n")

	processTestData([]byte("world"), cmdLog)
	assert.False(t, cmdLog.PendingNewline, "PendingNewline should be cmdLogeared after content read")

	assertLines(t, cmdLog, []string{"hello", "world"})
}

func TestProcessTerminalOutput_PendingNewlineOverwrite(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()

	processTestData([]byte("hello\r\n"), cmdLog)
	assert.True(t, cmdLog.PendingNewline, "PendingNewline should be set after trailing \\n")

	processTestData([]byte("\roverwritten"), cmdLog)
	assertLines(t, cmdLog, []string{"hello", "overwritten"})
}

func TestProcessTerminalOutput_ANSIOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"ANSI reset only", "\x1b[0m\n", nil},
		{"ANSI color only", "\x1b[31m\n", nil},
		{"multiple ANSI only", "\x1b[1;32m\x1b[0m\n", nil},
		{"ANSI reset overwrite", "real content\r\x1b[0m\n", nil},
		{"ANSI with visible content", "\x1b[31mred\x1b[0m\n", []string{"\x1b[31mred\x1b[0m"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cmdLog := newTestCommandLog()
			processTestData([]byte(test.input), cmdLog)
			assertLines(t, cmdLog, test.want)
		})
	}
}

func TestProcessTerminalOutput_EraseLineStripped(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()
	processTestData([]byte("before\x1b[Kafter\r\n"), cmdLog)
	assertLines(t, cmdLog, []string{"beforeafter"})
}

func TestProcessTerminalOutput_EmptyInput(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()
	processTestData([]byte{}, cmdLog)
	assertLines(t, cmdLog, nil)

	processTestData([]byte("\x1b[K"), cmdLog)
	assertLines(t, cmdLog, nil)
}

func TestProcessTerminalOutput_BareNewlineWithinFirstSegment(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()
	processTestData([]byte("line1\nline2"), cmdLog)
	assertLines(t, cmdLog, []string{"line1", "line2"})
}

func TestProcessTerminalOutput_AppendToLastLine(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()
	processTestData([]byte("hello"), cmdLog)
	processTestData([]byte(" world"), cmdLog)
	assertLines(t, cmdLog, []string{"hello world"})
}

func TestProcessTerminalOutput_CarriageReturnAcrossReads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		reads  []string
		want   []string
		wantCR bool
	}{
		{
			"progress then store path",
			[]string{"[1/1/2 built] building disko\r", "/nix/store/abc-disko\n"},
			[]string{"/nix/store/abc-disko"},
			false,
		},
		{
			"progress with ANSI then store path",
			[]string{"[\x1b[34;1m1\x1b[0m/1 built] building disko\r", "/nix/store/abc-disko\n"},
			[]string{"/nix/store/abc-disko"},
			false,
		},
		{
			"trailing CR sets CarriageReturn",
			[]string{"hello\r"},
			[]string{"hello"},
			true,
		},
		{
			"trailing CR then content overrides",
			[]string{"old\r", "new\n"},
			[]string{"new"},
			false,
		},
		{
			"pending newline then CR then content",
			[]string{"hello\r\n", "\r/nix/store/abc\n"},
			[]string{"hello", "/nix/store/abc"},
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cmdLog := newTestCommandLog()
			for _, read := range test.reads {
				processTestData([]byte(read), cmdLog)
			}

			assertLines(t, cmdLog, test.want)
			assert.Equal(t, test.wantCR, cmdLog.CarriageReturn, "CarriageReturn flag")
		})
	}
}

func TestProcessTerminalOutput_CrossReadPendingNewlineCR(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()

	processTestData([]byte("line1\r\n"), cmdLog)
	assert.True(t, cmdLog.PendingNewline, "PendingNewline should be set after trailing \\n")

	processTestData([]byte("\rline2\r\n"), cmdLog)
	assertLines(t, cmdLog, []string{"line1", "line2"})
}

func TestProcessTerminalOutput_NixErrorOverwrite(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()

	processTestData([]byte("[0/1 built] building foo\x1b[0m\x1b[K\r"), cmdLog)
	assert.True(t, cmdLog.CarriageReturn, "progress line ends with \\r")

	processTestData([]byte("\x1b[K\x1b[31;1merror:\x1b[0m hash mismatch\r\n"), cmdLog)
	assertLines(t, cmdLog, []string{"\x1b[31;1merror:\x1b[0m hash mismatch"})
}

func TestProcessTerminalOutput_NixMultilineError(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()

	oneRead := "" +
		"\x1b[0m\x1b[K\r" +
		"\x1b[31;1merror:\x1b[0m hash mismatch in fixed-output derivation '...drv':\x1b[0m\r\n" +
		"         specified: sha256-deadbeef\x1b[0m\r\n" +
		"            got:    sha256-cafebabe\x1b[0m\x1b[0m"

	processTestData([]byte(oneRead), cmdLog)
	assertLines(t, cmdLog, []string{
		"\x1b[31;1merror:\x1b[0m hash mismatch in fixed-output derivation '...drv':\x1b[0m",
		"         specified: sha256-deadbeef\x1b[0m",
		"            got:    sha256-cafebabe\x1b[0m\x1b[0m",
	})
}

func TestProcessTerminalOutput_NixErrorFollowedByProgress(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()

	processTestData([]byte("            got:    sha256-cafebabe\x1b[0m\x1b[0m\r\n"), cmdLog)
	assert.True(t, cmdLog.PendingNewline)

	processTestData([]byte("\r[8/835 built (1 failed)] building rust\x1b[0m\x1b[K\r"), cmdLog)
	assertLines(t, cmdLog, []string{
		"            got:    sha256-cafebabe\x1b[0m\x1b[0m",
		"[8/835 built (1 failed)] building rust\x1b[0m",
	})
}

func TestProcessTerminalOutput_NixWarningSurvivesProgressBars(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()

	processTestData([]byte(
		"\x1b[0m\x1b[K\r"+
			"\x1b[K\x1b[35;1mwarning:\x1b[0m Git tree is dirty\x1b[0m\r\n\r"+
			"evaluating derivation\x1b[0m\x1b[K\r"+
			"querying info\x1b[0m\x1b[K\r",
	), cmdLog)

	assertLines(t, cmdLog, []string{
		"\x1b[35;1mwarning:\x1b[0m Git tree is dirty\x1b[0m",
		"querying info\x1b[0m",
	})
	assert.True(t, cmdLog.CarriageReturn)

	for range 50 {
		processTestData([]byte(
			"\x1b[0m\x1b[K\r"+
				"[\x1b[32;1m0\x1b[0m/293 built, 0/47 copied]\x1b[0m\x1b[K\r",
		), cmdLog)
	}

	lines := make([]string, cmdLog.Output.Len())
	for i := range lines {
		lines[i] = string(cmdLog.Output.Line(i))
	}

	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "warning:")
	assert.Contains(t, lines[1], "built")

	processTestData([]byte(
		"\x1b[0m\x1b[K\r"+
			"\x1b[31;1merror:\x1b[0m hash mismatch\r\n"+
			"         specified: sha256-deadbeef\x1b[0m\r\n"+
			"            got:    sha256-cafebabe\x1b[0m\x1b[0m\r\n",
	), cmdLog)

	assertLines(t, cmdLog, []string{
		"\x1b[35;1mwarning:\x1b[0m Git tree is dirty\x1b[0m",
		"\x1b[31;1merror:\x1b[0m hash mismatch",
		"         specified: sha256-deadbeef\x1b[0m",
		"            got:    sha256-cafebabe\x1b[0m\x1b[0m",
	})
}

func TestProcessTerminalOutput_NixFullWarningToError(t *testing.T) {
	t.Parallel()

	cmdLog := newTestCommandLog()
	proc := terminalProcessor{output: cmdLog.Output}

	raw := []byte(
		"\x1b[0m\x1b[K\r" +
			"\x1b[K" +
			"\x1b[35;1mwarning:\x1b[0m Git tree is dirty\x1b[0m\r\n\r" +
			"evaluating derivation\x1b[0m\x1b[K\r" +
			"querying info\x1b[0m\x1b[K\r" +
			"\x1b[0m\x1b[K\r[\x1b[32;1m0\x1b[0m/293 built]\x1b[0m\x1b[K\r" +
			"\x1b[0m\x1b[K\r[\x1b[32;1m0\x1b[0m/238 built]\x1b[0m\x1b[K\r" +
			"\x1b[0m\x1b[K\r\x1b[K" +
			"\x1b[31;1merror:\x1b[0m hash mismatch\r\n" +
			"         specified: sha256-deadbeef\x1b[0m\r\n" +
			"            got:    sha256-cafebabe\x1b[0m\x1b[0m\r\n" +
			"\r[8/835 built (1 failed)] building rust\x1b[0m\x1b[K\r",
	)

	chunkSize := 15
	for i := 0; i < len(raw); i += chunkSize {
		end := min(i+chunkSize, len(raw))

		proc.process(raw[i:end], cmdLog)
	}

	finalizeCommandLog(cmdLog)

	require.GreaterOrEqual(t, cmdLog.Output.Len(), 1)
	assert.Contains(t, string(cmdLog.Output.Line(0)), "warning:", "warning must survive chunked reads")
	assert.Contains(t, string(cmdLog.Output.Line(1)), "error:", "error must follow warning")
}

func TestFinalizeCommandLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		reads []string
		want  []string
	}{
		{
			"nix copy progress removed",
			[]string{"[1/614/615 copied (1.2 GiB)] copying 615 paths\r"},
			nil,
		},
		{
			"nix copy with ANSI progress removed",
			[]string{"[\x1b[34;1m1\x1b[0m/\x1b[32;1m614\x1b[0m/615 copied]\r"},
			nil,
		},
		{
			"progress then real output kept",
			[]string{"[1/2 built]\r", "/nix/store/abc\r\n"},
			[]string{"/nix/store/abc"},
		},
		{
			"no CR preserves all lines",
			[]string{"hello\r\n", "world\r\n"},
			[]string{"hello", "world"},
		},
		{
			"pending newline without CR preserves lines",
			[]string{"hello\r\n"},
			[]string{"hello"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cmdLog := newTestCommandLog()
			for _, read := range test.reads {
				processTestData([]byte(read), cmdLog)
			}

			finalizeCommandLog(cmdLog)
			assertLines(t, cmdLog, test.want)
			assert.False(t, cmdLog.CarriageReturn, "CarriageReturn should be cmdLogeared")
			assert.False(t, cmdLog.PendingNewline, "PendingNewline should be cmdLogeared")
		})
	}
}

func TestProcessTerminalOutput_Backspaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"simple backspace", "abc\b\b\bdone\n", []string{"done"}},
		{
			"progress pattern with done",
			"Allocating group tables:  0/80\b\b\b\b\b     \b\b\b\b\bdone                            \n",
			[]string{"Allocating group tables: done                            "},
		},
		{
			"backspace across combined content",
			"Creating filesystem with 2620672 4k blocks and 655360 inodes\n\t32768, 98304\n",
			[]string{
				"Creating filesystem with 2620672 4k blocks and 655360 inodes",
				"    32768, 98304",
			},
		},
		{"backspace with carriage return", "first\rsecond\b\bdone\n", []string{"secodone"}},
		{"backspace at start of segment", "\b\ba\n", []string{"a"}},
		{"backspace erasing everything", "abc\b\b\b\n", nil},
		{
			"multiple progress lines with backspaces",
			"Discarding device blocks:       0/2620672" +
				"\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b               " +
				"\b\b\b\b\b\b\b\b\b\b\b\b\b\b\bdone                            \n",
			[]string{"Discarding device blocks: done                            "},
		},
		{"no backspaces", "normal line\n", []string{"normal line"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cmdLog := newTestCommandLog()
			processTestData([]byte(test.input), cmdLog)
			assertLines(t, cmdLog, test.want)
		})
	}
}

func TestApplyBackspaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no backspace", "hello", "hello"},
		{"simple", "ab\bc", "ac"},
		{"multiple", "abc\b\b\bdone", "done"},
		{"at start", "\b\ba", "a"},
		{"erase all", "abc\b\b\b", ""},
		{"progress done", "0/80\b\b\b\b\b     \b\b\b\b\bdone", "done"},
		{"with newline", "abc\b\b\ndef\ndone", "a\ndef\ndone"},
		{"empty", "", ""},
		{"only backspace", "\b", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := string(applyBackspaces([]byte(test.input)))
			assert.Equal(t, test.want, got, "applyBackspaces(%q)", test.input)
		})
	}
}

func TestProcessTerminalOutput_TabExpansion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"simple tab", "\t32768\n", []string{"    32768"}},
		{"tab mid line", "hello\tworld\n", []string{"hello    world"}},
		{"multiple tabs", "a\tb\tc\n", []string{"a    b    c"}},
		{"tab at start", "\tdone\n", []string{"    done"}},
		{"no tabs", "plain text\n", []string{"plain text"}},
		{"tab with backspaces", "0/80\b\b\t\bdone\n", []string{"0/   done"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cmdLog := newTestCommandLog()
			processTestData([]byte(test.input), cmdLog)
			assertLines(t, cmdLog, test.want)
		})
	}
}

// processTestData is a test helper that runs terminalProcessor.process on
// the given data and CommandLog.
func processTestData(data []byte, cmdLog *command.CommandLog) {
	tp := terminalProcessor{output: cmdLog.Output}
	tp.process(data, cmdLog)
}

func newTestCommandLog() *command.CommandLog {
	return &command.CommandLog{
		Output: buffer.NewLinesBufVer(),
	}
}

func assertLines(t *testing.T, cmdLog *command.CommandLog, want []string) {
	t.Helper()

	if len(want) == 0 {
		assert.Zero(t, cmdLog.Output.Len(), "expected no lines")

		return
	}

	got := make([]string, cmdLog.Output.Len())
	for idx := range cmdLog.Output.Len() {
		got[idx] = string(cmdLog.Output.Line(idx))
	}

	assert.Equal(t, want, got)
}
