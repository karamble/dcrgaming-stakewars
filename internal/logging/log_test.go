package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSubsystemLevelsAndOutput(t *testing.T) {
	var stdout bytes.Buffer
	b, e := Open(t.TempDir(), "SWAR=warn,SDK=debug", 1024, 2, &stdout)
	if e != nil {
		t.Fatal(e)
	}
	b.Logger("SWAR").Info("hidden")
	b.Logger("SDK").Debug("protocol detail")
	b.Logger("SWAR").Warn("visible warning")
	if e = b.Close(); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(b.Path)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(raw), "hidden") || !strings.Contains(string(raw), "[DBG] SDK:") || !strings.Contains(string(raw), "[WRN] SWAR:") {
		t.Fatalf("log format/levels: %s", raw)
	}
	if !bytes.Equal(stdout.Bytes(), raw) {
		t.Fatal("console and file differ")
	}
	for _, bad := range []string{"bogus", "NOPE=info", "SDK=bogus", "SWAR=info,broken"} {
		if _, e = Levels(bad); e == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
func TestConcurrentRotationIsBounded(t *testing.T) {
	dir := t.TempDir()
	b, e := Open(dir, "info", 1, 2, nil)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 25; n++ {
				b.Logger("SESS").Info(strings.Repeat("x", 100))
			}
		}()
	}
	wg.Wait()
	if e = b.Close(); e != nil {
		t.Fatal(e)
	}
	files, e := filepath.Glob(filepath.Join(dir, "dcrstakewars.log*"))
	if e != nil {
		t.Fatal(e)
	}
	if len(files) < 2 || len(files) > 3 {
		t.Fatalf("rotation count: %v", files)
	}
}
