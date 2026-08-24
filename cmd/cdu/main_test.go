package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pottom/cdu/cmd/cdu/app"
)

// cdu introduced itself as `gdu` for six PRs. This file is upstream's and is our
// merge conflict surface, so an upstream sync can hand the text straight back —
// which is exactly why this is a test and not a one-off fix.
//
// The rename script deliberately does not cover this: its scope is import paths
// and directory names, and its own header says flag help is rewritten on `main`.
func TestTheCommandDoesNotIntroduceItselfAsGdu(t *testing.T) {
	if got, want := strings.Fields(rootCmd.Use)[0], "cdu"; got != want {
		t.Errorf("rootCmd.Use starts with %q, want %q", got, want)
	}
	if strings.Contains(rootCmd.Short, "gdu") || strings.Contains(rootCmd.Short, "Gdu") {
		t.Errorf("the one-line summary names gdu: %q", rootCmd.Short)
	}
	if strings.Contains(rootCmd.Long, "Gdu is intended") {
		t.Error("rootCmd.Long is still upstream's description")
	}

	// gdu is named in Long on purpose — cdu is its fork and says so, which the
	// licence and NOTICE both require. What it must not do is claim to be it.
	for _, want := range []string{"fork of gdu", "not the official gdu"} {
		if !strings.Contains(rootCmd.Long, want) {
			t.Errorf("rootCmd.Long should say %q", want)
		}
	}

	// Flag help is user-facing text too, and it is where the last one hid.
	for line := range strings.SplitSeq(rootCmd.Flags().FlagUsages(), "\n") {
		if strings.Contains(line, "Gdu ") {
			t.Errorf("flag help calls the program Gdu:%s", line)
		}
	}
}

func TestNoViewFileFlagRegistered(t *testing.T) {
	flag := rootCmd.Flags().Lookup("no-view-file")
	if flag == nil {
		t.Fatal("expected no-view-file flag to be registered")
	}
}

func TestNoViewFileFlagCanBeSet(t *testing.T) {
	t.Cleanup(func() {
		_ = rootCmd.Flags().Set("no-view-file", "false")
	})

	err := rootCmd.Flags().Set("no-view-file", "true")
	if err != nil {
		t.Fatalf("expected setting no-view-file flag to succeed: %v", err)
	}

	if !af.NoViewFile {
		t.Fatal("expected NoViewFile to be true after setting flag")
	}
}

func TestShowSymlinkTargetFlagRegistered(t *testing.T) {
	flag := rootCmd.Flags().Lookup("show-symlink-target")
	if flag == nil {
		t.Fatal("expected show-symlink-target flag to be registered")
	}
	if flag.DefValue != "false" {
		t.Fatalf("expected show-symlink-target to default to false, got %q", flag.DefValue)
	}
}

func TestShowSymlinkTargetFlagCanBeSet(t *testing.T) {
	t.Cleanup(func() {
		_ = rootCmd.Flags().Set("show-symlink-target", "false")
	})

	err := rootCmd.Flags().Set("show-symlink-target", "true")
	if err != nil {
		t.Fatalf("expected setting show-symlink-target flag to succeed: %v", err)
	}

	if !af.ShowSymlinkTarget {
		t.Fatal("expected ShowSymlinkTarget to be true after setting flag")
	}
}

func TestInteractiveFlagRegistered(t *testing.T) {
	flag := rootCmd.Flags().Lookup("interactive")
	if flag == nil {
		t.Fatal("expected interactive flag to be registered")
	}
}

func TestInteractiveFlagCanBeSet(t *testing.T) {
	t.Cleanup(func() {
		_ = rootCmd.Flags().Set("interactive", "false")
	})

	err := rootCmd.Flags().Set("interactive", "true")
	if err != nil {
		t.Fatalf("expected setting interactive flag to succeed: %v", err)
	}

	if !af.Interactive {
		t.Fatal("expected Interactive to be true after setting flag")
	}
}

func TestSetConfigFilePathWithSpaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir with spaces")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}
	path := filepath.Join(dir, "my config.yaml")

	origArgs := os.Args
	origErr := configErr
	origAf := af
	t.Cleanup(func() {
		os.Args = origArgs
		configErr = origErr
		af = origAf
	})

	for _, args := range [][]string{
		{"gdu", "--config-file=" + path},
		{"gdu", "--config-file", path},
	} {
		af = &app.Flags{}
		configErr = nil
		os.Args = args

		setConfigFilePath()

		if af.CfgFile != path {
			t.Errorf("expected config file path %q for args %v, got %q", path, args, af.CfgFile)
		}
	}
}

func TestInitConfigMalformedUserConfig(t *testing.T) {
	// cdu has no /etc system config to layer (unlike gdu); its config comes from
	// the user file, named here via --config-file. A malformed one must surface as
	// configErr, not be swallowed.
	tmp := filepath.Join(t.TempDir(), "user.yaml")
	if err := os.WriteFile(tmp, []byte(":\tinvalid: yaml: {"), 0o600); err != nil {
		t.Fatalf("could not write temp config: %v", err)
	}

	origArgs := os.Args
	origErr := configErr
	origAf := af
	t.Cleanup(func() {
		os.Args = origArgs
		configErr = origErr
		af = origAf
	})

	os.Args = []string{"cdu", "--config-file=" + tmp}
	af = &app.Flags{}
	configErr = nil

	initConfig()

	if configErr == nil {
		t.Fatal("expected configErr to be set for malformed user config, got nil")
	}
}
