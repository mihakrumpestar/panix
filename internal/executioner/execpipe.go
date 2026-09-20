package executioner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/pkg/errors"
)

// ErrPipeCommandFailed is the sentinel wrapped around a failed pipe process pair.
var ErrPipeCommandFailed = errors.New("pipe command failed")

// pipeStderrCaptureLimit caps captured stderr at 64 KiB; excess is discarded.
const pipeStderrCaptureLimit = 64 * 1024

// pipeBufferLimit is the largest payload buffered for the hash comparison; larger payloads stream.
const pipeBufferLimit = 4 << 20 // 4 MiB

// PipeSpec is the ExecPipe contract: Source runs on the control host and its
// stdout is the payload, Probe reports the target's current sha256, and Write
// receives the payload on stdin when the target must change.
type PipeSpec struct {
	Source []string // control-host argv whose stdout is the payload; also the logged command
	Probe  []string // destination-side argv; first stdout token must be the target's sha256
	Write  []string // destination-side argv receiving the payload on stdin
}

// ExecPipe streams spec.Source's stdout into spec.Write's stdin through an OS
// pipe, without a PTY and without a temporary plaintext file. Only spec.Source
// is logged.
//
// Hard guarantee: the payload never enters the zerolog stream,
// CommandLog.Output or error strings; only bounded stderr excerpts do. The
// guarantee does not cover destination file descriptors, so destinations must
// never echo their stdin to stderr.
//
// spec.Probe and spec.Write are destination-side argv, routed exactly once
// through the machine's active transport (ssh when remote, direct otherwise).
// A matching probe hash skips the write and preserves the target's inode and
// mtime while still enforcing metadata. Payloads up to pipeBufferLimit bytes
// are buffered for the comparison; larger payloads stream and always write.
// Any probe failure means write, never transfer failure.
//
// SkipIfLocal, DisableAutoSSHCommand and Trim are rejected. Dry-run starts
// nothing and requires OnDryRun whenever OnSuccess is provided.
func (ex *Executioner) ExecPipe(
	description, statusIfRunning, statusIfFailed string,
	spec PipeSpec,
	opts ...ExecOption,
) error {
	err := validatePipeSpec(spec)
	if err != nil {
		return err
	}

	excOpt := &ExecOptions{}
	for _, opt := range opts {
		opt(excOpt)
	}

	err = validatePipeOptions(excOpt)
	if err != nil {
		return err
	}

	err = validateExecOptions(excOpt)
	if err != nil {
		return err
	}

	commandLog := ex.conf.PhaseLog.NewCommand(
		ex.phaseXpath, description, statusIfRunning, statusIfFailed,
		spec.Source, 0,
	)
	endLog := ex.startCommandLog(commandLog, description, statusIfRunning, commandLog.Command)

	cmdCtx, cancel := context.WithTimeout(ex.conf.Ctx, ex.conf.Timeout)
	defer cancel()

	var execErr error

	defer func() {
		endLog(execErr, commandLog)
	}()

	ex.conf.OnUpdateHook()

	if ex.conf.DryRun {
		return ex.handleDryRun(excOpt)
	}

	run := &pipeRun{ex: ex, ctx: cmdCtx, spec: spec}

	execErr = run.execute()

	execErr = finalizeExecError(execErr, commandLog, excOpt)
	if execErr != nil {
		return errors.Wrap(execErr, statusIfFailed)
	}

	return nil
}

// validatePipeSpec rejects empty argv before prepareCommand can index them.
func validatePipeSpec(spec PipeSpec) error {
	if len(spec.Source) == 0 {
		return errors.New("exec pipe: source argv is empty")
	}

	if len(spec.Probe) == 0 {
		return errors.New("exec pipe: probe argv is empty")
	}

	if len(spec.Write) == 0 {
		return errors.New("exec pipe: write argv is empty")
	}

	return nil
}

// validatePipeOptions rejects options ExecPipe does not honor.
func validatePipeOptions(excOpt *ExecOptions) error {
	switch {
	case excOpt.skipIfLocal:
		return errors.New("exec pipe: SkipIfLocal is not supported")
	case excOpt.disableAutoSSHCommand:
		return errors.New("exec pipe: DisableAutoSSHCommand is not supported")
	case excOpt.trim:
		return errors.New("exec pipe: Trim is not supported")
	default:
		return nil
	}
}

// pipeRun carries the state of one ExecPipe execution.
type pipeRun struct {
	ex        *Executioner
	ctx       context.Context
	spec      PipeSpec
	source    *exec.Cmd
	sourceOut io.ReadCloser
	sourceErr *boundedCapture
	writeArgv []string
}

// execute starts the source, then commits the payload as a skip, a buffered write or a stream.
func (run *pipeRun) execute() error {
	err := run.startSource()
	if err != nil {
		return err
	}

	buf, err := io.ReadAll(io.LimitReader(run.sourceOut, pipeBufferLimit+1))
	if err != nil {
		run.abort()

		return errors.Wrap(err, "failed to read source stdout")
	}

	// The write argv is routed exactly once; the probe routes itself.
	run.writeArgv = run.ex.routeDestination(run.spec.Write)

	if len(buf) > pipeBufferLimit {
		return run.stream(buf)
	}

	return run.commit(buf)
}

// startSource starts the source with bounded stderr capture.
func (run *pipeRun) startSource() error {
	run.source = run.ex.prepareCommand(run.ctx, run.spec.Source)
	run.sourceErr = new(boundedCapture)

	run.source.Stderr = run.sourceErr

	sourceOut, err := run.source.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to create source stdout pipe")
	}

	run.sourceOut = sourceOut

	err = run.source.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start source command")
	}

	return nil
}

// abort closes the read end and kills/reaps the source while it may still be live.
func (run *pipeRun) abort() {
	_ = run.sourceOut.Close()

	terminatePipeProcess(run.source)
}

// startDestination starts one destination argv. A nil stdout means the write:
// stdout goes to the null device and stderr is captured for failure text. A
// set stdout means the probe: stdout is captured and stderr stays nil, so no
// unused capture is allocated.
func (run *pipeRun) startDestination(argv []string, stdin io.Reader, stdout io.Writer) (*exec.Cmd, *boundedCapture, error) {
	cmd := run.ex.prepareCommand(run.ctx, argv)
	cmd.Stdin = stdin
	cmd.Stdout = stdout

	var stderr *boundedCapture
	if stdout == nil {
		stderr = new(boundedCapture)
		cmd.Stderr = stderr
	}

	err := cmd.Start()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start destination command")
	}

	return cmd, stderr, nil
}

// commit hashes buffered content only after a successful source Wait, then skips or writes it.
func (run *pipeRun) commit(buf []byte) error {
	err := run.source.Wait()
	if err != nil {
		return run.failure(err, nil, "")
	}

	sum := sha256.Sum256(buf)

	if run.probeMatches(hex.EncodeToString(sum[:])) {
		return nil
	}

	writeCmd, writeStderr, err := run.startDestination(run.writeArgv, bytes.NewReader(buf), nil)
	if err != nil {
		return err
	}

	return run.failure(nil, writeCmd.Wait(), writeStderr.String())
}

// stream pipes content larger than the buffer straight through. The write is
// waited first because its stdin copier must drain the source, and only then is
// the read end closed, so an early write failure cannot stall the source until
// the context timeout.
func (run *pipeRun) stream(buf []byte) error {
	writeCmd, writeStderr, err := run.startDestination(run.writeArgv, io.MultiReader(bytes.NewReader(buf), run.sourceOut), nil)
	if err != nil {
		run.abort()

		return err
	}

	writeErr := writeCmd.Wait()

	_ = run.sourceOut.Close()

	sourceErr := run.source.Wait()

	return run.failure(sourceErr, writeErr, writeStderr.String())
}

// probeMatches reports whether the routed probe output carries wantHash; any
// failure means needs update.
func (run *pipeRun) probeMatches(wantHash string) bool {
	probeStdout := new(boundedCapture)

	probeCmd, _, err := run.startDestination(run.ex.routeDestination(run.spec.Probe), nil, probeStdout)
	if err != nil {
		return false
	}

	if probeCmd.Wait() != nil {
		return false
	}

	return parseProbeHash(probeStdout.String()) == wantHash
}

// failure names every failed process, source first, with its bounded stderr;
// stdout is never touched because it may be the payload. The context error is
// folded in only alongside a process failure, so a completed operation is
// always success.
func (run *pipeRun) failure(sourceErr, destinationErr error, destinationStderr string) error {
	failures := make([]string, 0, 2)

	if sourceErr != nil {
		failures = append(failures, failureLine("source", sourceErr, run.sourceErr.String()))
	}

	if destinationErr != nil {
		failures = append(failures, failureLine("destination", destinationErr, destinationStderr))
	}

	if len(failures) == 0 {
		return nil
	}

	processErr := errors.Wrapf(ErrPipeCommandFailed, "%s", strings.Join(failures, "; "))

	ctxErr := run.ctx.Err()
	if ctxErr != nil {
		return errors.Wrapf(processErr, "context: %v", ctxErr)
	}

	return processErr
}

// failureLine renders one process failure with its trimmed stderr, omitted when empty.
func failureLine(process string, err error, stderr string) string {
	if stderr == "" {
		return fmt.Sprintf("%s command failed: %v", process, err)
	}

	return fmt.Sprintf("%s command failed: %v (stderr: %s)", process, err, stderr)
}

// routeDestination routes destination-side argv exactly once; never pass already-routed argv.
func (ex *Executioner) routeDestination(commandWithArgs []string) []string {
	if ex.conf.Machine == nil {
		return commandWithArgs
	}

	return pipeCommands(ex.conf.Machine.GetActiveSSH(), commandWithArgs)
}

// pipeCommands leaves local destinations unwrapped and ssh-wraps remote ones.
func pipeCommands(sshClient ssh.SSHClient, destinationCommandWithArgs []string) []string {
	if sshClient.IsLocal() {
		return destinationCommandWithArgs
	}

	return sshCommandWithArgs(sshClient, destinationCommandWithArgs)
}

// terminatePipeProcess kills and reaps a peer whose partner failed to start.
func terminatePipeProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

// parseProbeHash extracts the sha256 hex from the probe output, or "" when absent.
func parseProbeHash(output string) string {
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return ""
	}

	hash := strings.ToLower(fields[0])

	decoded, err := hex.DecodeString(hash)
	if err != nil || len(decoded) != sha256.Size {
		return ""
	}

	return hash
}

// boundedCapture keeps at most pipeStderrCaptureLimit bytes; excess is
// acknowledged and discarded so writers never see a short write.
type boundedCapture struct {
	buf bytes.Buffer
}

func (capture *boundedCapture) Write(data []byte) (int, error) {
	if remaining := pipeStderrCaptureLimit - capture.buf.Len(); remaining > 0 {
		_, _ = capture.buf.Write(data[:min(len(data), remaining)])
	}

	return len(data), nil
}

func (capture *boundedCapture) String() string {
	return strings.TrimSpace(capture.buf.String())
}
