package envyr

import "fmt"

// RunError is returned when envyr exits with a non-zero status.
type RunError struct {
	ExitCode int
	Stderr   string
}

func (e *RunError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("envyr: exit code %d: %s", e.ExitCode, e.Stderr)
	}
	return fmt.Sprintf("envyr: exit code %d", e.ExitCode)
}
