package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// clipboardCommands lists the commands to try, in order, for this OS.
func clipboardCommands(goos string, getenv func(string) string) [][]string {
	switch goos {
	case "darwin":
		return [][]string{{"pbcopy"}}
	case "windows":
		return [][]string{{"clip"}}
	default: // Linux, BSD
		var cmds [][]string
		if getenv("WAYLAND_DISPLAY") != "" {
			cmds = append(cmds, []string{"wl-copy"})
		}
		return append(cmds,
			[]string{"xclip", "-selection", "clipboard"},
			[]string{"xsel", "--clipboard", "--input"},
		)
	}
}

// Swappable in tests, so they never touch the real clipboard.
var (
	lookPath   = exec.LookPath
	runCommand = func(ctx context.Context, path string, args []string, stdin io.Reader) error {
		// Command and args come from the fixed list above, never from user input.
		cmd := exec.CommandContext(ctx, path, args...) //nolint:gosec // G204: fixed, trusted command
		cmd.Stdin = stdin
		return cmd.Run()
	}
)

// clipboardTimeout bounds each tool: they should finish instantly.
const clipboardTimeout = 5 * time.Second

// copyToClipboard puts text on the system clipboard via the OS's own tool.
// If a tool is installed but fails (say xclip with no X display), the next
// one is tried.
func copyToClipboard(text string) error {
	var errs []error
	for _, c := range clipboardCommands(runtime.GOOS, os.Getenv) {
		path, err := lookPath(c[0])
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), clipboardTimeout)
		err = runCommand(ctx, path, c[1:], strings.NewReader(text))
		cancel()
		if err == nil {
			return nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", c[0], err))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return errors.New("no clipboard tool found (install wl-clipboard, xclip or xsel)")
}
