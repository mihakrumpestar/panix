package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	panixDeploySubcommand   = "deploy"
	panixSecretsSubcommand  = "secrets"
	panixRollbackSubcommand = "rollback"
)

func runPanixDeployWithArgs(configPath string, extraArgs []string, envVars ...string) error {
	return runPanixCommandWithArgs(panixDeploySubcommand, configPath, extraArgs, envVars...)
}

// runPanixSecretsWithArgs runs the Inspect+Secrets phases only: used to prove
// the conditional write skip on an already bootstrapped machine.
func runPanixSecretsWithArgs(configPath string, extraArgs []string, envVars ...string) error {
	return runPanixCommandWithArgs(panixSecretsSubcommand, configPath, extraArgs, envVars...)
}

func runPanixCommandWithArgs(subcommand, configPath string, extraArgs []string, envVars ...string) error {
	stopSequentialMgr()

	return errors.Wrap(panixCommandSpec(subcommand, configPath, extraArgs, envVars, nil).Run(), "run panix")
}

// panixCommandSpec builds (but does not start) one panix process: the shared
// construction for the sequential deploy steps and the concurrent-deploy leg's
// parallel processes. A nil sink inherits the harness's stdout/stderr; a sink
// captures the process output for assertions and keeps concurrent processes'
// TUIs off the shared terminal.
func panixCommandSpec(subcommand, configPath string, extraArgs []string, envVars []string, sink io.Writer) *exec.Cmd {
	root := findProjectRoot()

	mode := envValue(envVars, "PANIX_TEST_MODE")
	if mode == "" {
		mode = "default"
	}

	panixLogPath := filepath.Join(logDirPath, "panix-"+mode+".log")
	e2eDir := filepath.Join(root, "tests", "e2e")

	bin, baseArgs := panixExecArgs(root)

	var cmdArgs []string

	cmdArgs = append(cmdArgs, baseArgs...)
	cmdArgs = append(cmdArgs,
		subcommand, "-c", configPath,
		"--exit-on-complete",
		"--log", "--log-file", panixLogPath,
	)
	cmdArgs = append(cmdArgs, extraArgs...)

	cmd := exec.CommandContext(context.Background(), bin, cmdArgs...) //nolint:gosec
	cmd.Dir = root

	cleanEnv := sanitizedOsEnviron("PANIX_TEST_MODE")
	cmd.Env = append(append(cleanEnv, envVars...), "PANIX_E2E_DIR="+e2eDir)
	cmd.Stdin = os.Stdin

	if sink == nil {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = sink
		cmd.Stderr = sink
	}

	return cmd
}

func envValue(envVars []string, key string) string {
	prefix := key + "="

	for _, envVar := range envVars {
		if strings.HasPrefix(envVar, prefix) {
			result, _ := strings.CutPrefix(envVar, prefix)

			return result
		}
	}

	return ""
}

func panixExecArgs(root string) (string, []string) {
	if bin := os.Getenv("PANIX_BIN"); bin != "" {
		return bin, nil
	}

	return "go", []string{"run", root + "/cmd/panix"}
}

func sanitizedOsEnviron(stripKeys ...string) []string {
	stripSet := make(map[string]struct{}, len(stripKeys))
	for _, k := range stripKeys {
		stripSet[k] = struct{}{}
	}

	var result []string

	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if _, strip := stripSet[key]; strip {
			continue
		}

		result = append(result, env)
	}

	return result
}
