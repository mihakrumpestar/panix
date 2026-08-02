package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const scriptPerm = 0o755

func runPanixDeploy(configPath string, envVars ...string) error {
	return runPanixDeployWithArgs(configPath, nil, envVars...)
}

func runPanixDeployWithArgs(configPath string, extraArgs []string, envVars ...string) error {
	root := findProjectRoot()

	mode := envValue(envVars, "PANIX_TEST_MODE")
	if mode == "" {
		mode = "default"
	}

	panixLogPath := filepath.Join(logDirPath, "panix-"+mode+".log")
	panixOutputPath := filepath.Join(logDirPath, "panix-"+mode+".out")
	e2eDir := filepath.Join(root, "tests", "e2e")

	termPath, termExecArgs := findTerminalEmulator()
	if termPath != "" {
		exitCodePath := filepath.Join(logDirPath, "panix-"+mode+".exitcode")

		return runPanixInTerminal(termPath, termExecArgs,
			wrapperPath(mode, root, configPath, panixLogPath, panixOutputPath, extraArgs, envVars, e2eDir, exitCodePath), exitCodePath, panixOutputPath)
	}

	return runPanixInConsole(root, configPath, panixLogPath, extraArgs, envVars, e2eDir)
}

func runPanixInTerminal(termPath string, termExecArgs []string, wrapperPath, exitCodePath, panixOutputPath string) error {
	_ = os.Remove(exitCodePath)
	_ = os.Remove(panixOutputPath)

	err := exec.CommandContext(context.Background(), termPath, append(termExecArgs, wrapperPath)...).Run() //nolint:gosec
	if err != nil {
		return errors.Wrap(err, "run panix in terminal")
	}

	data, readErr := os.ReadFile(exitCodePath) //nolint:gosec
	if readErr != nil {
		return errors.New("panix exit code file not found, panix may have crashed before writing it")
	}

	code := strings.TrimSpace(string(data))
	if code != "0" {
		return errors.Errorf("panix exited with code %s\n%s", code, tailFile(panixOutputPath))
	}

	return nil
}

func tailFile(path string) string {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return "(output file not found)"
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	const maxLines = 30

	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	return strings.Join(lines, "\n")
}

func wrapperPath(
	mode, root, configPath, panixLogPath, panixOutputPath string,
	extraArgs []string,
	envVars []string,
	e2eDir, exitCodePath string,
) string {
	path := filepath.Join(logDirPath, "run-panix-"+mode+".sh")

	envLines := make([]string, 0, len(envVars)+2)
	envLines = append(envLines, "unset PANIX_TEST_MODE")

	for _, envVar := range envVars {
		envLines = append(envLines, "export "+envVar)
	}

	envLines = append(envLines, "export PANIX_E2E_DIR="+e2eDir)

	goArgs := "go run " + root + "/cmd/panix"
	panixCmd := "${PANIX_BIN:-" + goArgs + "}"

	extraArgsStr := ""
	if len(extraArgs) > 0 {
		extraArgsStr = " " + strings.Join(extraArgs, " ")
	}

	script := "#!/bin/sh\n" +
		strings.Join(envLines, "\n") + "\n" +
		panixCmd + " deploy -c " + configPath + " --exit-on-complete --log --log-file " + panixLogPath +
		extraArgsStr +
		" 2> " + panixOutputPath + "\n" +
		"echo $? > " + exitCodePath + "\n"

	_ = os.WriteFile(path, []byte(script), scriptPerm)

	return path
}

func runPanixInConsole(root, configPath, panixLogPath string, extraArgs []string, envVars []string, e2eDir string) error {
	bin, baseArgs := panixExecArgs(root)

	var cmdArgs []string

	cmdArgs = append(cmdArgs, baseArgs...)
	cmdArgs = append(cmdArgs,
		"deploy", "-c", configPath,
		"--output=console", "--exit-on-complete",
		"--log", "--log-file", panixLogPath,
	)
	cmdArgs = append(cmdArgs, extraArgs...)

	cmd := exec.CommandContext(context.Background(), bin, cmdArgs...) //nolint:gosec
	cmd.Dir = root

	cleanEnv := sanitizedOsEnviron("PANIX_TEST_MODE")
	cmd.Env = append(append(cleanEnv, envVars...), "PANIX_E2E_DIR="+e2eDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return errors.Wrap(cmd.Run(), "run panix")
}

func findTerminalEmulator() (string, []string) {
	path, err := exec.LookPath("konsole")
	if err == nil {
		return path, []string{
			"-p", "TerminalColumns=200",
			"-p", "TerminalRows=100",
			"-e",
		}
	}

	return "", nil
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
