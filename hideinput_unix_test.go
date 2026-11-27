//go:build !windows

package main

import (
	"io"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

// fakeEcho records echo changes instead of running stty.
type fakeEcho struct {
	mu    sync.Mutex
	calls []bool
}

func (f *fakeEcho) set(_ *os.File, on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, on)
	return nil
}

func (f *fakeEcho) last() []bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]bool(nil), f.calls...)
}

func swapEchoAndExit(t *testing.T, echo *fakeEcho, exits chan int) {
	t.Helper()
	oldEcho, oldExit := echoFn, exitFn
	echoFn = echo.set
	exitFn = func(code int) { exits <- code }
	t.Cleanup(func() { echoFn, exitFn = oldEcho, oldExit })
}

// Ctrl-C while the password prompt hides input must turn echo back on
// before exiting; otherwise the user's terminal stays silent.
func TestHideInputRestoresEchoOnCtrlC(t *testing.T) {
	echo := &fakeEcho{}
	exits := make(chan int, 1)
	swapEchoAndExit(t, echo, exits)

	restore := hideInput(nil, io.Discard)
	defer restore()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-exits:
		if code != 130 {
			t.Errorf("exit %d, want 130", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Ctrl-C was not handled")
	}
	if got := echo.last(); len(got) != 2 || got[0] || !got[1] {
		t.Errorf("echo calls %v, want [off on]", got)
	}
}

// The normal path restores echo exactly once and stops catching signals.
func TestHideInputRestoreIsIdempotent(t *testing.T) {
	echo := &fakeEcho{}
	exits := make(chan int, 1)
	swapEchoAndExit(t, echo, exits)

	restore := hideInput(nil, io.Discard)
	restore()
	if got := echo.last(); len(got) != 2 || got[0] || !got[1] {
		t.Errorf("echo calls %v, want [off on]", got)
	}
	select {
	case code := <-exits:
		t.Errorf("unexpected exit %d", code)
	default:
	}
}
