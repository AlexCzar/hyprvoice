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
	tmpDir := t.TempDir()
	fakeDotoolc := filepath.Join(tmpDir, "dotoolc")

	script := `#!/bin/sh
cat > "$1"
`
	if err := os.WriteFile(fakeDotoolc, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake dotoolc: %v", err)
	}

	tests := []struct {
		name     string
		text     string
		delay    int64
		hold     int64
		wantCmds string
	}{
		{
			name:  "single line",
			text:  "hello",
			delay: 1,
			hold:  2,
			wantCmds: "typedelay 1\ntypehold 2\ntype hello\n",
		},
		{
			name:  "multiline preserves newlines",
			text:  "hello\nworld",
			delay: 1,
			hold:  2,
			wantCmds: "typedelay 1\ntypehold 2\ntype hello\nkey enter\ntype world\n",
		},
		{
			name:  "three lines",
			text:  "a\nb\nc",
			delay: 5,
			hold:  10,
			wantCmds: "typedelay 5\ntypehold 10\ntype a\nkey enter\ntype b\nkey enter\ntype c\n",
		},
		{
			name:  "trailing newline",
			text:  "hello\n",
			delay: 1,
			hold:  2,
			wantCmds: "typedelay 1\ntypehold 2\ntype hello\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := NewDotoolBackend(tt.delay, tt.hold)
			db := backend.(*DotoolBackend)

			var input strings.Builder
			lines := strings.Split(tt.text, "\n")
			for i, line := range lines {
				fmt.Fprintf(&input, "type %s\n", line)
				if i < len(lines)-1 {
					fmt.Fprintf(&input, "key enter\n")
				}
			}

			got := fmt.Sprintf("typedelay %v\ntypehold %v\n%s", db.delayMs, db.holdMs, input.String())
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
	tmpDir := t.TempDir()
	stdinFile := filepath.Join(tmpDir, "stdin.txt")

	fakeScript := fmt.Sprintf(`#!/bin/sh
cat > "%s"
exit 0
`, stdinFile)

	fakeDotoolc := filepath.Join(tmpDir, "dotoolc")
	if err := os.WriteFile(fakeDotoolc, []byte(fakeScript), 0755); err != nil {
		t.Fatalf("failed to create fake dotoolc: %v", err)
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	backend := NewDotoolBackend(1, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := backend.Inject(ctx, "hello\nworld", 5*time.Second)
	if err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	got, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("failed to read captured stdin: %v", err)
	}

	want := "typedelay 1\ntypehold 2\ntype hello\nkey enter\ntype world\n"
	if string(got) != want {
		t.Errorf("captured stdin mismatch\nwant:\n%s\ngot:\n%s", want, string(got))
	}
}
