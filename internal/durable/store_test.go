package durable

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

func TestConcurrentImmutablePublication(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Put([32]byte{1}, []byte("first")); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err = s.Put([32]byte{1}, []byte("second")); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	b, err := s.Get([32]byte{1})
	if err != nil || !bytes.Equal(b, []byte("first")) {
		t.Fatal("record overwritten", err)
	}
}
