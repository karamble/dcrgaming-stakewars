//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package session

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func acquireInstanceLock(dir string) (func() error, bool, error) {
	return acquireLockFiles([]string{filepath.Join(dir, ".instance.lock")})
}

func acquireProfileProbe(dir string) (func() error, bool, error) {
	return acquireLockFiles([]string{filepath.Join(dir, "spends.json.lock"), filepath.Join(dir, "tables", ".runtime.lock")})
}

func acquireLockFiles(paths []string) (func() error, bool, error) {
	files := make([]*os.File, 0, len(paths))
	release := func() error {
		var first error
		for i := len(files) - 1; i >= 0; i-- {
			if err := files[i].Close(); err != nil && first == nil {
				first = err
			}
		}
		return first
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			_ = release()
			return nil, false, err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			_ = release()
			return nil, false, err
		}
		if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			_ = f.Close()
			_ = release()
			if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
				return nil, true, nil
			}
			return nil, false, err
		}
		files = append(files, f)
	}
	return release, false, nil
}
