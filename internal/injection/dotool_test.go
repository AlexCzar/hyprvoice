package injection

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDotoolMultilineHandling(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		delay    int64
		hold     int64
		wantCmds string
	}{
		{
			name:     "single line",
			text:     "hello",
			delay:    1,
			hold:     2,
			wantCmds: "typedelay 1\ntypehold 2\ntype hello\n",
		},
		{
			name:     "multiline preserves newline with enter",
			text:     "hello\nworld",
			delay:    1,
			hold:     2,
			wantCmds: "typedelay 1\ntypehold 2\ntype hello\nkey enter\ntype world\n",
		},
		{
			name:     "three lines",
			text:     "a\nb\nc",
			delay:    5,
			hold:     10,
			wantCmds: "typedelay 5\ntypehold 10\ntype a\nkey enter\ntype b\nkey enter\ntype c\n",
		},
		{
			name:     "blank line",
			text:     "a\n\nb",
			delay:    1,
			hold:     2,
			wantCmds: "typedelay 1\ntypehold 2\ntype a\nkey enter\nkey enter\ntype b\n",
		},
		{
			name:     "trailing newline",
			text:     "hello\n",
			delay:    1,
			hold:     2,
			wantCmds: "typedelay 1\ntypehold 2\ntype hello\nkey enter\n",
		},
		{
			name:     "crlf newline",
			text:     "hello\r\nworld",
			delay:    1,
			hold:     2,
			wantCmds: "typedelay 1\ntypehold 2\ntype hello\nkey enter\ntype world\n",
		},
		{
			name:     "newline does not inject dotool command",
			text:     "hello\nkey super",
			delay:    1,
			hold:     2,
			wantCmds: "typedelay 1\ntypehold 2\ntype hello\nkey enter\ntype key super\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := NewDotoolBackend(tt.delay, tt.hold)
			got := captureDotoolStdin(t, backend, tt.text)
			if got != tt.wantCmds {
				t.Errorf("command stream mismatch\nwant:\n%s\ngot:\n%s", tt.wantCmds, got)
			}
		})
	}
}

func TestDotoolConfigurableDelays(t *testing.T) {
	tests := []struct {
		delay int64
		hold  int64
	}{
		{1, 2},
		{5, 10},
		{0, 0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("delay=%d_hold=%d", tt.delay, tt.hold), func(t *testing.T) {
			backend := NewDotoolBackend(tt.delay, tt.hold)
			db := backend.(*DotoolBackend)

			if db.delayMs != tt.delay {
				t.Errorf("delayMs mismatch: want %d, got %d", tt.delay, db.delayMs)
			}
			if db.holdMs != tt.hold {
				t.Errorf("holdMs mismatch: want %d, got %d", tt.hold, db.holdMs)
			}
		})
	}
}

func TestDotoolName(t *testing.T) {
	backend := NewDotoolBackend(1, 2)
	if name := backend.Name(); name != "dotool" {
		t.Errorf("Name() = %q, want %q", name, "dotool")
	}
}

func TestDotoolAvailableCheck(t *testing.T) {
	backend := NewDotoolBackend(1, 2)

	err := backend.Available()
	if err == nil {
		t.Skip("dotoolc is installed, skipping availability check")
	}
	if !strings.Contains(err.Error(), "dotoolc") {
		t.Errorf("Expected error mentioning dotoolc, got: %v", err)
	}
}

func TestDotoolInjectWithFakeCommand(t *testing.T) {
	backend := NewDotoolBackend(1, 2)
	got := captureDotoolStdin(t, backend, "hello\nworld")

	want := "typedelay 1\ntypehold 2\ntype hello\nkey enter\ntype world\n"
	if got != want {
		t.Errorf("captured stdin mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func captureDotoolStdin(t *testing.T, backend Backend, text string) string {
	t.Helper()

	tmpDir := t.TempDir()
	stdinFile := filepath.Join(tmpDir, "stdin.txt")
	fakeDotoolc := filepath.Join(tmpDir, "dotoolc")

	fakeScript := fmt.Sprintf(`#!/bin/sh
cat > "%s"
exit 0
`, stdinFile)
	if err := os.WriteFile(fakeDotoolc, []byte(fakeScript), 0755); err != nil {
		t.Fatalf("failed to create fake dotoolc: %v", err)
	}

	t.Setenv("PATH", tmpDir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := backend.Inject(ctx, text, 5*time.Second); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	got, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("failed to read captured stdin: %v", err)
	}

	return string(got)
}
