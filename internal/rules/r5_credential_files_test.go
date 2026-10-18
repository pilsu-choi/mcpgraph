package rules

import (
	"path/filepath"
	"testing"

	"github.com/pilsuchoi/mcpgraph/internal/model"
	"github.com/pilsuchoi/mcpgraph/internal/testutil"
)

func TestCredentialFiles(t *testing.T) {
	// A fake home on disk, so the rule's real stat/glob code is exercised.
	home := t.TempDir()
	testutil.WriteFile(t, filepath.Join(home, ".aws", "credentials"), []byte("[default]\n"))
	testutil.WriteFile(t, filepath.Join(home, ".ssh", "id_ed25519"), []byte("fake"))
	testutil.WriteFile(t, filepath.Join(home, ".ssh", "id_ed25519.pub"), []byte("fake"))
	testutil.WriteFile(t, filepath.Join(home, "code", "app", ".env"), []byte("X=1"))
	testutil.WriteFile(t, filepath.Join(home, "code", "lib", "main.go"), []byte("package main"))

	tests := []struct {
		name string
		root string
		want int
	}{
		{"home exposes aws, ssh key and nothing else", home, 2},
		{"ssh dir exposes private key only", filepath.Join(home, ".ssh"), 1},
		{"project with .env", filepath.Join(home, "code", "app"), 1},
		{"clean project", filepath.Join(home, "code", "lib"), 0},
		{"unrelated dir", "/nonexistent-root", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkFindings(t, CredentialFiles{Home: home}, fsServer(tt.root), tt.want, model.SeverityHigh)
		})
	}
}

func TestWithin(t *testing.T) {
	tests := []struct {
		root, path string
		want       bool
	}{
		{"/home/u", "/home/u/.aws/credentials", true},
		{"/", "/home/u/.netrc", true},
		{"/home/u/code", "/home/u/.aws/credentials", false},
		{"/home/u", "/home/u2/.env", false},
		{"/home/u", "/home/u/..env", true},
	}
	for _, tt := range tests {
		if got := within(tt.root, tt.path); got != tt.want {
			t.Errorf("within(%q, %q) = %v, want %v", tt.root, tt.path, got, tt.want)
		}
	}
}
