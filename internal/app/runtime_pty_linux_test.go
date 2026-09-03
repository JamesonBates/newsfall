//go:build linux

package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

const newsfallPTYHelper = "NEWSFALL_PTY_HELPER"

func TestRuntimeAcceptsQuitAfterIdle(t *testing.T) {
	if os.Getenv(newsfallPTYHelper) == "1" {
		if err := Run(nil, Options{Demo: true, Stdin: os.Stdin, Stdout: os.Stdout}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	master, slave := openLinuxPTY(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestRuntimeAcceptsQuitAfterIdle$")
	cmd.Env = append(os.Environ(), newsfallPTYHelper+"=1", "HOME="+t.TempDir(), "TERM=xterm-256color")
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		master.Close()
		slave.Close()
		t.Fatal(err)
	}
	if err := slave.Close(); err != nil {
		t.Fatal(err)
	}
	defer master.Close()

	ready := make(chan struct{})
	go drainUntilReady(master, ready)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatal("Newsfall did not enter its terminal screen")
	}

	// Newsfall's original VMIN=0/VTIME=1 setup made Go report EOF after
	// roughly 100 ms, permanently ending the keyboard reader. Waiting after
	// the alternate screen appears reproduces the real user's failure without
	// depending on how quickly an instrumented test process starts.
	time.Sleep(300 * time.Millisecond)
	if _, err := master.Write([]byte("q")); err != nil {
		_ = cmd.Process.Kill()
		<-done
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Newsfall exited with error after q: %v", err)
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatal("q was ignored after the terminal sat idle")
	}
}

func openLinuxPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	fail := func(err error) (*os.File, *os.File) {
		master.Close()
		t.Fatal(err)
		return nil, nil
	}

	unlock := int32(0)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), uintptr(syscall.TIOCSPTLCK), uintptr(unsafe.Pointer(&unlock))); errno != 0 {
		return fail(errno)
	}
	var number uint32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), uintptr(syscall.TIOCGPTN), uintptr(unsafe.Pointer(&number))); errno != 0 {
		return fail(errno)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return fail(err)
	}
	return master, slave
}

func drainUntilReady(master *os.File, ready chan<- struct{}) {
	buffer := make([]byte, 32*1024)
	seen := make([]byte, 0, 64*1024)
	signaled := false
	for {
		n, err := master.Read(buffer)
		if n > 0 && !signaled {
			seen = append(seen, buffer[:n]...)
			if bytes.Contains(seen, []byte("\x1b[?1049h")) {
				close(ready)
				signaled = true
				seen = nil
			} else if len(seen) > 64*1024 {
				seen = append(seen[:0], seen[len(seen)-128:]...)
			}
		}
		if err != nil {
			return
		}
	}
}
