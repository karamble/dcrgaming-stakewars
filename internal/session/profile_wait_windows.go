//go:build windows

package session

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func acquireProfileProbe(dir string) (func() error, bool, error) {
	paths := []string{filepath.Join(dir, "spends.json.lock"), filepath.Join(dir, "tables", ".runtime.lock")}
	type lockedFile struct {
		file    *os.File
		overlap *windows.Overlapped
	}
	files := make([]lockedFile, 0, len(paths))
	release := func() error {
		var first error
		for i := len(files) - 1; i >= 0; i-- {
			f := files[i]
			if err := windows.UnlockFileEx(windows.Handle(f.file.Fd()), 0, 1, 0, f.overlap); err != nil && first == nil {
				first = err
			}
			if err := f.file.Close(); err != nil && first == nil {
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
		overlap := new(windows.Overlapped)
		err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlap)
		if err != nil {
			_ = f.Close()
			_ = release()
			if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
				return nil, true, nil
			}
			return nil, false, err
		}
		files = append(files, lockedFile{file: f, overlap: overlap})
	}
	return release, false, nil
}
