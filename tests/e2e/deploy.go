package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func runPanixDeployWithArgs(configPath string, extraArgs []string, envVars ...string) error {
	root := findProjectRoot()

	mode := envValue(envVars, "PANIX_TEST_MODE")
	if mode == "" {
		mode = "default"
	}

	panixLogPath := filepath.Join(logDirPath, "panix-"+mode+".log")
	e2eDir := filepath.Join(root, "tests", "e2e")

	return runPanixInConsole(root, configPath, panixLogPath, extraArgs, envVars, e2eDir)
}

func runPanixInConsole(root, configPath, panixLogPath string, extraArgs []string, envVars []string, e2eDir string) error {
	stopSequentialMgr()

	bin, baseArgs := panixExecArgs(root)

	var cmdArgs []string

	cmdArgs = append(cmdArgs, baseArgs...)
	cmdArgs = append(cmdArgs,
		"deploy", "-c", configPath,
		"--exit-on-complete",
		"--log", "--log-file", panixLogPath,
	)
	cmdArgs = append(cmdArgs, extraArgs...)

	cmd := exec.CommandContext(context.Background(), bin, cmdArgs...) //nolint:gosec
	cmd.Dir = root

	cleanEnv := sanitizedOsEnviron("PANIX_TEST_MODE")
	cmd.Env = append(append(cleanEnv, envVars...), "PANIX_E2E_DIR="+e2eDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return errors.Wrap(cmd.Run(), "run panix")
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
