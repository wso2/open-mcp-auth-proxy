package subprocess

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	logger "github.com/wso2/open-mcp-auth-proxy/internal/logging"
)

// Manager handles starting and graceful shutdown of subprocesses
type Manager struct {
	process       *os.Process
	processGroup  int
	mutex         sync.Mutex
	cmd           *exec.Cmd
	shutdownDelay time.Duration
}

// NewManager creates a new subprocess manager
func NewManager() *Manager {
	return &Manager{
		shutdownDelay: 5 * time.Second,
	}
}

// EnsureDependenciesAvailable checks and installs required package executors
func EnsureDependenciesAvailable(command string) error {
	// Always ensure npx is available regardless of the command
	if _, err := exec.LookPath("npx"); err != nil {
		// npx is not available, check if npm is installed
		if _, err := exec.LookPath("npm"); err != nil {
			return fmt.Errorf("npx not found and npm not available; please install Node.js from https://nodejs.org/")
		}

		// Try to install npx using npm
		logger.Info("npx not found, attempting to install...")
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("npm.cmd", "install", "-g", "npx")
		} else {
			cmd = exec.Command("npm", "install", "-g", "npx")
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install npx: %w", err)
		}

		logger.Info("npx installed successfully")
	}

	// Check if uv is needed based on the command
	if strings.Contains(command, "uv ") {
		if _, err := exec.LookPath("uv"); err != nil {
			return fmt.Errorf("command requires uv but it's not installed; please install it following instructions at https://github.com/astral-sh/uv")
		}
	}

	return nil
}

// SetShutdownDelay sets the maximum time to wait for graceful shutdown
func (m *Manager) SetShutdownDelay(duration time.Duration) {
	m.shutdownDelay = duration
}

// Start launches a subprocess based on the configuration
func (m *Manager) Start(cfg *config.Config) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// If a process is already running, return an error
	if m.process != nil {
		return os.ErrExist
	}

	if !cfg.Stdio.Enabled || cfg.Stdio.UserCommand == "" {
		return nil // Nothing to start
	}

	// Get the full command string
	execCommand := cfg.BuildExecCommand()
	if execCommand == "" {
		return nil // No command to execute
	}

	logger.Info("Starting subprocess with command: %s", execCommand)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Use PowerShell on Windows for better quote handling
		cmd = exec.Command("powershell", "-Command", execCommand)
	} else {
		cmd = exec.Command("sh", "-c", execCommand)
	}

	// Set working directory if specified
	if cfg.Stdio.WorkDir != "" {
		cmd.Dir = cfg.Stdio.WorkDir
	}

	// Set environment variables if specified
	if len(cfg.Stdio.Env) > 0 {
		cmd.Env = append(os.Environ(), cfg.Stdio.Env...)
	}

	// Capture stdout/stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set platform-specific process attributes
	setProcAttr(cmd)

	// Start the process
	if err := cmd.Start(); err != nil {
		return err
	}

	m.process = cmd.Process
	m.cmd = cmd
	logger.Info("Subprocess started with PID: %d", m.process.Pid)

	// Get and store the process group ID (Unix) or PID (Windows)
	pgid, err := getProcessGroup(m.process.Pid)
	if err == nil {
		m.processGroup = pgid
		if runtime.GOOS != "windows" {
			logger.Debug("Process group ID: %d", m.processGroup)
		}
	} else {
		logger.Warn("Failed to get process group ID: %v", err)
		m.processGroup = m.process.Pid
	}

	// Handle process termination in background
	go func() {
		if err := cmd.Wait(); err != nil {
			logger.Error("Subprocess exited with error: %v", err)
		} else {
			logger.Info("Subprocess exited successfully")
		}

		// Clear the process reference when it exits
		m.mutex.Lock()
		m.process = nil
		m.cmd = nil
		m.mutex.Unlock()
	}()

	return nil
}

// IsRunning checks if the subprocess is running
func (m *Manager) IsRunning() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.process != nil
}

// Shutdown gracefully terminates the subprocess
func (m *Manager) Shutdown() {
	m.mutex.Lock()
	processToTerminate := m.process // Local copy of the process reference
	processGroupToTerminate := m.processGroup
	m.mutex.Unlock()

	if processToTerminate == nil {
		return // No process to terminate
	}

	logger.Info("Terminating subprocess...")
	terminateComplete := make(chan struct{})

	go func() {
		defer close(terminateComplete)

		// Try graceful termination first
		terminatedGracefully := false

		if runtime.GOOS == "windows" {
			// Windows: Try to terminate the process
			m.mutex.Lock()
			if m.process != nil {
				err := m.process.Kill()
				if err != nil {
					logger.Warn("Failed to terminate process: %v", err)
				}
			}
			m.mutex.Unlock()

			// Wait a bit to see if it terminates
			for i := 0; i < 10; i++ {
				time.Sleep(200 * time.Millisecond)
				m.mutex.Lock()
				if m.process == nil {
					terminatedGracefully = true
					m.mutex.Unlock()
					break
				}
				m.mutex.Unlock()
			}
		} else {
			// Unix: Use SIGTERM followed by SIGKILL if necessary
			// Try to terminate the process group first
			if processGroupToTerminate != 0 {
				err := killProcessGroup(processGroupToTerminate, syscall.SIGTERM)
				if err != nil {
					logger.Warn("Failed to send SIGTERM to process group: %v", err)

					// Fallback to terminating just the process
					m.mutex.Lock()
					if m.process != nil {
						err = m.process.Signal(syscall.SIGTERM)
						if err != nil {
							logger.Warn("Failed to send SIGTERM to process: %v", err)
						}
					}
					m.mutex.Unlock()
				}
			} else {
				// Try to terminate just the process
				m.mutex.Lock()
				if m.process != nil {
					err := m.process.Signal(syscall.SIGTERM)
					if err != nil {
						logger.Warn("Failed to send SIGTERM to process: %v", err)
					}
				}
				m.mutex.Unlock()
			}

			// Wait for the process to exit gracefully
			for i := 0; i < 10; i++ {
				time.Sleep(200 * time.Millisecond)

				m.mutex.Lock()
				if m.process == nil {
					terminatedGracefully = true
					m.mutex.Unlock()
					break
				}
				m.mutex.Unlock()
			}
		}

		if terminatedGracefully {
			logger.Info("Subprocess terminated gracefully")
			return
		}

		// If the process didn't exit gracefully, force kill
		logger.Warn("Subprocess didn't exit gracefully, forcing termination...")

		if runtime.GOOS == "windows" {
			// On Windows, Kill() is already forceful
			m.mutex.Lock()
			if m.process != nil {
				if err := m.process.Kill(); err != nil {
					logger.Error("Failed to kill process: %v", err)
				}
			}
			m.mutex.Unlock()
		} else {
			// Unix: Try SIGKILL
			// Try to kill the process group first
			if processGroupToTerminate != 0 {
				if err := killProcessGroup(processGroupToTerminate, syscall.SIGKILL); err != nil {
					logger.Warn("Failed to send SIGKILL to process group: %v", err)

					// Fallback to killing just the process
					m.mutex.Lock()
					if m.process != nil {
						if err := m.process.Kill(); err != nil {
							logger.Error("Failed to kill process: %v", err)
						}
					}
					m.mutex.Unlock()
				}
			} else {
				// Try to kill just the process
				m.mutex.Lock()
				if m.process != nil {
					if err := m.process.Kill(); err != nil {
						logger.Error("Failed to kill process: %v", err)
					}
				}
				m.mutex.Unlock()
			}
		}

		// Wait a bit more to confirm termination
		time.Sleep(500 * time.Millisecond)

		m.mutex.Lock()
		if m.process == nil {
			logger.Info("Subprocess terminated by force")
		} else {
			logger.Warn("Failed to terminate subprocess")
		}
		m.mutex.Unlock()
	}()

	// Wait for termination with timeout
	select {
	case <-terminateComplete:
		// Termination completed
	case <-time.After(m.shutdownDelay):
		logger.Warn("Subprocess termination timed out")
	}
}
