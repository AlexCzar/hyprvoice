package injection

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type DotoolBackend struct {
	delayMs int64
	holdMs  int64
}

func NewDotoolBackend(delayMs int64, holdMs int64) Backend {
	return &DotoolBackend{delayMs: delayMs, holdMs: holdMs}

}

func (c *DotoolBackend) Name() string {
	return "dotool"
}

func (c *DotoolBackend) Available() error {
	if _, err := exec.LookPath("dotoolc"); err != nil {
		return fmt.Errorf("dotoolc not found: %w (install dotool)", err)
	}

	return nil
}

func (c *DotoolBackend) Inject(ctx context.Context, text string, timeout time.Duration) error {
	if err := c.Available(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dotoolc")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	input, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("dotoolc failed: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("dotoolc failed: %w", err)
	}

	var typeCommands strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		fmt.Fprintf(&typeCommands, "type %s\n", line)
	}

	if _, err := fmt.Fprintf(input, "typedelay %v\ntypehold %v\n%s", c.delayMs, c.holdMs, typeCommands.String()); err != nil {
		input.Close()
		return fmt.Errorf("writing to dotoolc stdin failed: %w", err)
	}

	input.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("dotoolc failed: %w\n%s", err, stderr.String())
	}

	return nil
}
