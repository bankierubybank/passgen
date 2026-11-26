package main

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// clipboardCommands lists the commands to try, in order, for this OS.
func clipboardCommands() [][]string {
	switch runtime.GOOS {
	case "darwin":
		return [][]string{{"pbcopy"}}
	case "windows":
		return [][]string{{"clip"}}
	default: // Linux, BSD
		var cmds [][]string
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			cmds = append(cmds, []string{"wl-copy"})
		}
		return append(cmds,
			[]string{"xclip", "-selection", "clipboard"},
			[]string{"xsel", "--clipboard", "--input"},
		)
	}
}

// copyToClipboard puts text on the system clipboard via the OS's own tool.
func copyToClipboard(text string) error {
	for _, c := range clipboardCommands() {
		path, err := exec.LookPath(c[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, c[1:]...)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	return errors.New("no clipboard tool found (install wl-clipboard, xclip or xsel)")
}
