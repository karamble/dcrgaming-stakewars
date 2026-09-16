// Package durable provides immutable, bounded records for local experimental
// protocol journals. A successful Put survives restart; conflicting writes
// never replace an existing record. The directory must be trusted local storage.
package durable

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

var ErrConflict = errors.New("durable record conflicts with existing intent")

const MaxRecord = 1 << 20

type Store struct{ dir string }

func Open(dir string) (*Store, error) {
	var err error
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	// MkdirAll may have created several ancestors; each directory entry must
	// be durable before a caller can rely on records stored beneath it.
	for path := dir; ; path = filepath.Dir(path) {
		if err := syncDir(path); err != nil {
			return nil, err
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	return &Store{dir: dir}, nil
}
func syncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func (s *Store) path(key [32]byte) string {
	return filepath.Join(s.dir, hex.EncodeToString(key[:])+".record")
}
func (s *Store) Get(key [32]byte) ([]byte, error) {
	f, e := os.Open(s.path(key))
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, MaxRecord+1))
	if e != nil {
		return nil, e
	}
	if len(b) > MaxRecord {
		return nil, errors.New("oversize durable record")
	}
	return b, nil
}
func (s *Store) Put(key [32]byte, data []byte) error {
	_, err := s.Create(key, data)
	return err
}

// Create reports whether this caller published the record first. A false result
// never grants permission to repeat an external side effect.
func (s *Store) Create(key [32]byte, data []byte) (bool, error) {
	if len(data) > MaxRecord {
		return false, errors.New("oversize durable record")
	}
	f, e := os.CreateTemp(s.dir, ".pending-")
	if e != nil {
		return false, e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(data); e != nil {
		f.Close()
		return false, e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return false, e
	}
	if e = f.Close(); e != nil {
		return false, e
	}
	// Linking publishes a fully written inode atomically without overwriting.
	created := true
	if e = os.Link(name, s.path(key)); e != nil {
		if !errors.Is(e, os.ErrExist) {
			return false, e
		}
		created = false
		old, err := s.Get(key)
		if err != nil {
			return false, err
		}
		if !bytes.Equal(old, data) {
			return false, ErrConflict
		}
	}
	if err := syncDir(s.dir); err != nil {
		return false, err
	}
	return created, nil
}
