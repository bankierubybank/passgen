package main

import (
	"context"
	"errors"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestClipboardCommands(t *testing.T) {
	none := func(string) string { return "" }
	wayland := func(k string) string {
		if k == "WAYLAND_DISPLAY" {
			return "wayland-0"
		}
		return ""
	}
	first := func(cmds [][]string) []string {
		var out []string
		for _, c := range cmds {
			out = append(out, c[0])
		}
		return out
	}
	cases := []struct {
		goos   string
		getenv func(string) string
		want   []string
	}{
		{"darwin", none, []string{"pbcopy"}},
		{"windows", none, []string{"clip"}},
		{"linux", none, []string{"xclip", "xsel"}},
		{"linux", wayland, []string{"wl-copy", "xclip", "xsel"}},
		{"freebsd", none, []string{"xclip", "xsel"}},
	}
	for _, c := range cases {
		if got := first(clipboardCommands(c.goos, c.getenv)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.goos, got, c.want)
		}
	}
}

type fakeTool struct {
	installed map[string]bool
	fail      map[string]bool
	ran       []string
	got       string
	deadline  bool
}

func swapClipboard(t *testing.T, f *fakeTool) {
	t.Helper()
	oldLook, oldRun := lookPath, runCommand
	lookPath = func(name string) (string, error) {
		if f.installed[name] {
			return "/fake/" + name, nil
		}
		return "", errors.New("not found")
	}
	runCommand = func(ctx context.Context, path string, _ []string, stdin io.Reader) error {
		name := strings.TrimPrefix(path, "/fake/")
		f.ran = append(f.ran, name)
		if dl, ok := ctx.Deadline(); ok && time.Until(dl) <= clipboardTimeout {
			f.deadline = true
		}
		b, _ := io.ReadAll(stdin)
		f.got = string(b)
		if f.fail[name] {
			return errors.New("cannot open display")
		}
		return nil
	}
	t.Cleanup(func() { lookPath, runCommand = oldLook, oldRun })
}

func TestCopyToClipboard(t *testing.T) {
	cmds := clipboardCommands(runtime.GOOS, func(string) string { return "" })
	all := map[string]bool{}
	for _, c := range cmds {
		all[c[0]] = true
	}

	f := &fakeTool{installed: all}
	swapClipboard(t, f)
	if err := copyToClipboard("s3cret"); err != nil {
		t.Fatal(err)
	}
	if f.got != "s3cret" || len(f.ran) != 1 || f.ran[0] != cmds[0][0] {
		t.Errorf("ran %v with %q", f.ran, f.got)
	}
	if !f.deadline {
		t.Error("clipboard tool ran without a timeout")
	}
}

func TestCopyToClipboardNoTool(t *testing.T) {
	swapClipboard(t, &fakeTool{})
	if err := copyToClipboard("x"); err == nil || !strings.Contains(err.Error(), "no clipboard tool") {
		t.Errorf("err %v", err)
	}
}

// A tool that is installed but broken must not stop the next one from working.
func TestCopyToClipboardFallsBack(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		t.Skip("only one clipboard tool on this OS")
	}
	f := &fakeTool{installed: map[string]bool{"xclip": true, "xsel": true}, fail: map[string]bool{"xclip": true}}
	swapClipboard(t, f)
	if err := copyToClipboard("x"); err != nil {
		t.Fatalf("err %v", err)
	}
	if !reflect.DeepEqual(f.ran, []string{"xclip", "xsel"}) {
		t.Errorf("ran %v", f.ran)
	}
	f = &fakeTool{installed: map[string]bool{"xclip": true}, fail: map[string]bool{"xclip": true}}
	swapClipboard(t, f)
	if err := copyToClipboard("x"); err == nil || !strings.Contains(err.Error(), "xclip") {
		t.Errorf("err %v, want the xclip failure", err)
	}
}

func TestCopyFlag(t *testing.T) {
	f := &fakeTool{installed: map[string]bool{"pbcopy": true, "clip": true, "xclip": true, "xsel": true}}
	swapClipboard(t, f)
	code, out, errOut := runCLI(t, "", "-n", "2", "--copy")
	if code != 0 || !strings.Contains(errOut, "copied") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if first := strings.Fields(out)[0]; f.got != first {
		t.Errorf("copied %q, want the first password %q", f.got, first)
	}
}
