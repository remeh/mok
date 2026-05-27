package session

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const sessionDirName = "sessions"

// GetSessionDir returns the path to the session directory (~/.mok/sessions/).
func GetSessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".mok", sessionDirName), nil
}

// EnsureSessionDir creates the session directory if it doesn't exist.
func EnsureSessionDir() error {
	dir, err := GetSessionDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating session directory: %w", err)
	}
	return nil
}

// GenerateSessionFilename generates a unique filename based on the current
// timestamp (e.g., session_20240115_143022.json).
func GenerateSessionFilename() string {
	now := time.Now()
	return fmt.Sprintf("session_%s.json", now.Format("20060102_150405"))
}

// ListSessions returns a sorted list of session file paths, newest first.
func ListSessions() ([]string, error) {
	dir, err := GetSessionDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading session directory: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		sessions = append(sessions, filepath.Join(dir, entry.Name()))
	}

	return sessions, nil
}

// DeleteSession removes a session file from disk.
func DeleteSession(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("deleting session file: %w", err)
	}
	return nil
}
