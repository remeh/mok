package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathAbsolute(t *testing.T) {
	resolved, err := ResolvePath("/tmp/test.txt", "/home/user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "/tmp/test.txt" {
		t.Errorf("resolved = %q, want '/tmp/test.txt'", resolved)
	}
}

func TestResolvePathRelative(t *testing.T) {
	resolved, err := ResolvePath("foo/bar.txt", "/home/user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "/home/user/foo/bar.txt" {
		t.Errorf("resolved = %q, want '/home/user/foo/bar.txt'", resolved)
	}
}

func TestResolvePathEmpty(t *testing.T) {
	resolved, err := ResolvePath("", "/home/user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != "" {
		t.Errorf("resolved = %q, want ''", resolved)
	}
}

func TestResolvePathTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("could not get home dir")
	}

	resolved, err := ResolvePath("~/test.txt", "/home/user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(home, "test.txt")
	if resolved != expected {
		t.Errorf("resolved = %q, want %q", resolved, expected)
	}
}

func TestIsSafePath(t *testing.T) {
	tests := []struct {
		resolved string
		cwd      string
		want     bool
	}{
		{"/home/user/file.txt", "/home/user", true},
		{"/home/user/sub/file.txt", "/home/user", true},
		{"/tmp/file.txt", "/home/user", false},
		{"/home/other/file.txt", "/home/user", false},
		{"", "/home/user", false},
		{"/home/user", "", false},
	}

	for _, tt := range tests {
		got := IsSafePath(tt.resolved, tt.cwd)
		if got != tt.want {
			t.Errorf("IsSafePath(%q, %q) = %v, want %v", tt.resolved, tt.cwd, got, tt.want)
		}
	}
}

func TestFileExists(t *testing.T) {
	// Create a temp file
	tmpfile, err := os.CreateTemp("", "test_*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	if !FileExists(tmpfile.Name()) {
		t.Error("FileExists returned false for existing file")
	}
	if FileExists("/nonexistent/file.txt") {
		t.Error("FileExists returned true for nonexistent file")
	}
}

func TestDirExists(t *testing.T) {
	if !DirExists("/") {
		t.Error("DirExists returned false for /")
	}
	if DirExists("/nonexistent/dir") {
		t.Error("DirExists returned true for nonexistent dir")
	}
}
