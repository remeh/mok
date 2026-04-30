package tools

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// ResolvePath resolves a path relative to CWD.
// Handles ~, relative paths, and absolute paths.
func ResolvePath(path, cwd string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Handle ~ expansion
	if strings.HasPrefix(path, "~/") || path == "~" {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		path = filepath.Join(usr.HomeDir, path[1:])
	}

	// Handle absolute paths
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	// Resolve relative to CWD
	return filepath.Clean(filepath.Join(cwd, path)), nil
}

// IsSafePath checks if a resolved path is within allowed directories.
// For now, we allow any path within the CWD tree or absolute paths.
// This can be extended later with explicit allowlists.
func IsSafePath(resolved, cwd string) bool {
	if resolved == "" || cwd == "" {
		return false
	}

	// Check if the path is within the CWD
	rel, err := filepath.Rel(cwd, resolved)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", it's outside CWD
	// This is a basic check; for production use, consider more robust solutions
	return !strings.HasPrefix(rel, "..")
}

// FileExists checks if a file exists at the given path.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists at the given path.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
