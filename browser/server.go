package browser

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

// Start launches a headless Chrome instance and persists the connection info.
// Chrome is launched with Leakless(false) so it persists after the CLI exits.
func Start(local bool) (*State, error) {
	// Check if already running
	existing, err := ReadState()
	if err == nil && existing != nil {
		// Verify it's actually alive
		if isProcessAlive(existing.PID) {
			return existing, fmt.Errorf("browser already running (pid %d)", existing.PID)
		}
		// Stale state, clean up
		_ = RemoveState()
	}

	// Ensure state directory exists
	if _, err := EnsureStateDir(local); err != nil {
		return nil, fmt.Errorf("creating state directory: %w", err)
	}

	// Launch Chrome via rod's launcher.
	// Leakless(false) prevents the leakless wrapper from killing Chrome when CLI exits.
	// KeepUserDataDir() prevents cleanup of the user data dir on close.
	l := launcher.New().
		Headless(true).
		Leakless(false).
		Set("disable-gpu").
		Set("no-sandbox")

	wsURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launching Chrome: %w", err)
	}

	// Get the Chrome PID from the launcher
	pid := l.PID()

	state := &State{
		WSEndpoint: wsURL,
		PID:        pid,
		StartedAt:  time.Now(),
		Viewport:   &Viewport{Width: 1280, Height: 720},
	}

	if err := WriteState(state); err != nil {
		return nil, fmt.Errorf("writing state: %w", err)
	}

	return state, nil
}

// Stop kills the Chrome process and removes state.
func Stop() error {
	state, err := ReadState()
	if err != nil {
		return fmt.Errorf("no browser running: %w", err)
	}

	// Kill the Chrome process tree
	proc, err := os.FindProcess(state.PID)
	if err == nil {
		// Kill the process group to get Chrome and all children
		_ = syscall.Kill(-state.PID, syscall.SIGKILL)
		_ = proc.Kill()
	}

	return RemoveState()
}

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
