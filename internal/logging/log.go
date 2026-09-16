// Package logging supplies Decred slog subsystems with bounded log rotation.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
)

var IDs = []string{"SWAR", "SESS", "SDK", "BRDG"}

func Levels(spec string) (map[string]slog.Level, error) {
	levels := make(map[string]slog.Level, len(IDs))
	for _, id := range IDs {
		levels[id] = slog.LevelInfo
	}
	if !strings.ContainsAny(spec, ",=") {
		l, ok := slog.LevelFromString(spec)
		if !ok {
			return nil, fmt.Errorf("invalid log level %q", spec)
		}
		for id := range levels {
			levels[id] = l
		}
		return levels, nil
	}
	for _, part := range strings.Split(spec, ",") {
		pair := strings.Split(part, "=")
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid subsystem level %q", part)
		}
		id := strings.TrimSpace(pair[0])
		if _, ok := levels[id]; !ok {
			return nil, fmt.Errorf("unknown log subsystem %q", id)
		}
		level, ok := slog.LevelFromString(strings.TrimSpace(pair[1]))
		if !ok {
			return nil, fmt.Errorf("invalid log level %q", pair[1])
		}
		levels[id] = level
	}
	return levels, nil
}
func Subsystems() string {
	ids := append([]string(nil), IDs...)
	sort.Strings(ids)
	return strings.Join(ids, " ")
}

type Backend struct {
	mu       sync.Mutex
	rotation *rotator.Rotator
	stdout   io.Writer
	closed   bool
	loggers  map[string]slog.Logger
	Path     string
}

func Open(dir, spec string, sizeKB int64, maxFiles int, stdout io.Writer) (*Backend, error) {
	levels, e := Levels(spec)
	if e != nil {
		return nil, e
	}
	if sizeKB < 1 || maxFiles < 1 {
		return nil, fmt.Errorf("invalid log rotation limits")
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	path := filepath.Join(dir, "dcrstakewars.log")
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	r, e := rotator.New(path, sizeKB, false, maxFiles)
	if e != nil {
		return nil, e
	}
	// Synchronous uncompressed rotation keeps the retention bound even during
	// rapid writes; asynchronous compression can leave old archives behind.
	r.SetCompressor(nil, "")
	b := &Backend{rotation: r, stdout: stdout, Path: path, loggers: map[string]slog.Logger{}}
	backend := slog.NewBackend(b)
	for id, level := range levels {
		l := backend.Logger(id)
		l.SetLevel(level)
		b.loggers[id] = l
	}
	return b, nil
}
func (b *Backend) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return len(p), nil
	}
	if b.stdout != nil {
		_, _ = b.stdout.Write(p)
	}
	return b.rotation.Write(p)
}
func (b *Backend) Logger(id string) slog.Logger {
	if l := b.loggers[id]; l != nil {
		return l
	}
	return slog.Disabled
}
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	return b.rotation.Close()
}
