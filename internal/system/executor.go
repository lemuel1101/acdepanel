package system

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Success  bool
}

func Execute(cmd string, args ...string) *ExecResult {
	return ExecuteWithTimeout(30*time.Second, cmd, args...)
}

func ExecuteWithTimeout(timeout time.Duration, cmd string, args ...string) *ExecResult {
	result := &ExecResult{}

	c := exec.Command(cmd, args...)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	done := make(chan error, 1)
	go func() {
		done <- c.Run()
	}()

	select {
	case err := <-done:
		result.Stdout = strings.TrimSpace(stdout.String())
		result.Stderr = strings.TrimSpace(stderr.String())

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = -1
			}
			result.Success = false
		} else {
			result.ExitCode = 0
			result.Success = true
		}

	case <-time.After(timeout):
		c.Process.Kill()
		result.Stderr = "command timed out"
		result.ExitCode = -1
		result.Success = false
	}

	return result
}

func ExecuteBash(script string) *ExecResult {
	return ExecuteWithTimeout(60*time.Second, "bash", "-c", script)
}

func WriteFile(path, content string) error {
	result := ExecuteBash(fmt.Sprintf("cat > %s << 'NOVAPANEL_EOF'\n%s\nNOVAPANEL_EOF", path, content))
	if !result.Success {
		return fmt.Errorf("failed to write file %s: %s", path, result.Stderr)
	}
	return nil
}

func FileExists(path string) bool {
	result := Execute("test", "-f", path)
	return result.Success
}

func ServiceAction(service, action string) *ExecResult {
	return Execute("systemctl", action, service)
}

func ServiceStatus(service string) bool {
	result := Execute("systemctl", "is-active", "--quiet", service)
	return result.Success
}
