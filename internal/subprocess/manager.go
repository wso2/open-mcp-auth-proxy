package subprocess

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"strings"

	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/logging"
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
        cmd := exec.Command("npm", "install", "-g", "npx")
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

// Start launches a subprocess based on the command configuration
func (m *Manager) Start(cmdConfig *config.Command) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// If a process is already running, return an error
	if m.process != nil {
		return os.ErrExist
	}

	if !cmdConfig.Enabled || cmdConfig.UserCommand == "" {
		return nil // Nothing to start
	}

	// Get the full command string
	execCommand := cmdConfig.GetExec()
	if execCommand == "" {
		return nil // No command to execute
	}

	logger.Info("Starting subprocess with command: %s", execCommand)

	// Use the shell to execute the command
	cmd := exec.Command("sh", "-c", execCommand)

	// Set working directory if specified
	if cmdConfig.WorkDir != "" {
		cmd.Dir = cmdConfig.WorkDir
	}

	// Set environment variables if specified
	if len(cmdConfig.Env) > 0 {
		cmd.Env = append(os.Environ(), cmdConfig.Env...)
	}

	// Capture stdout/stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set the process group for proper termination
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Start the process
	if err := cmd.Start(); err != nil {
		return err
	}

	m.process = cmd.Process
	m.cmd = cmd
	logger.Info("Subprocess started with PID: %d", m.process.Pid)

	// Get and store the process group ID
	pgid, err := syscall.Getpgid(m.process.Pid)
	if err == nil {
		m.processGroup = pgid
		logger.Debug("Process group ID: %d", m.processGroup)
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
	processToTerminate := m.process           // Local copy of the process reference
	processGroupToTerminate := m.processGroup
	m.mutex.Unlock()

	if processToTerminate == nil {
		return // No process to terminate
	}

	logger.Info("Terminating subprocess...")
	terminateComplete := make(chan struct{})

	go func() {
		defer close(terminateComplete)

		// Try graceful termination first with SIGTERM
		terminatedGracefully := false

		// Try to terminate the process group first
		if processGroupToTerminate != 0 {
			err := syscall.Kill(-processGroupToTerminate, syscall.SIGTERM)
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

		if terminatedGracefully {
			logger.Info("Subprocess terminated gracefully")
			return
		}

		// If the process didn't exit gracefully, force kill
		logger.Warn("Subprocess didn't exit gracefully, forcing termination...")

		// Try to kill the process group first
		if processGroupToTerminate != 0 {
			if err := syscall.Kill(-processGroupToTerminate, syscall.SIGKILL); err != nil {
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