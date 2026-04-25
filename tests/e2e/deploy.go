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
	root := findProjectRoot()

	mode := envValue(envVars, "PANIX_TEST_MODE")
	if mode == "" {
		mode = "default"
	}

	panixLogPath := filepath.Join(logDirPath, "panix-"+mode+".log")
	e2eDir := filepath.Join(root, "tests", "e2e")
	goArgs := "run " + root + "/cmd/panix"

	termPath, termExecArgs := findTerminalEmulator()
	if termPath != "" {
		return runPanixInTerminal(termPath, termExecArgs, wrapperPath(mode, goArgs, configPath, panixLogPath, envVars, e2eDir))
	}

	return runPanixInConsole(root, configPath, panixLogPath, envVars, e2eDir)
}

func runPanixInTerminal(termPath string, termExecArgs []string, wrapperPath string) error {
	return errors.Wrap(
		exec.CommandContext(context.Background(), termPath, append(termExecArgs, wrapperPath)...).Run(), //nolint:gosec
		"run panix in terminal",
	)
}

func wrapperPath(mode, goArgs, configPath, panixLogPath string, envVars []string, e2eDir string) string {
	path := filepath.Join(logDirPath, "run-panix-"+mode+".sh")
	envLines := make([]string, 0, len(envVars)+1)

	for _, envVar := range envVars {
		envLines = append(envLines, "export "+envVar)
	}

	envLines = append(envLines, "export PANIX_E2E_DIR="+e2eDir)

	script := "#!/bin/sh\n" +
		strings.Join(envLines, "\n") + "\n" +
		"go " + goArgs + " deploy -c " + configPath + " --exit-on-complete --log --log-file " + panixLogPath + "\n"

	_ = os.WriteFile(path, []byte(script), scriptPerm)

	return path
}

func runPanixInConsole(root, configPath, panixLogPath string, envVars []string, e2eDir string) error {
	cmd := exec.CommandContext(context.Background(), "go", //nolint:gosec
		"run", root+"/cmd/panix",
		"deploy", "-c", configPath,
		"--output=console", "--exit-on-complete",
		"--log", "--log-file", panixLogPath,
	)
	cmd.Dir = root

	cmd.Env = append(os.Environ(), envVars...)
	cmd.Env = append(cmd.Env, "PANIX_E2E_DIR="+e2eDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return errors.Wrap(cmd.Run(), "run panix")
}

func findTerminalEmulator() (string, []string) {
	for _, emulator := range []struct {
		name  string
		flags []string
	}{
		{"kitty", []string{"--hold"}},
		{"alacritty", []string{"-e"}},
		{"konsole", []string{"-e"}},
		{"xfce4-terminal", []string{"-x"}},
		{"xterm", []string{"-e"}},
	} {
		path, err := exec.LookPath(emulator.name)
		if err == nil {
			return path, emulator.flags
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
