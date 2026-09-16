package sound

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestSoundBuffersAreBoundedStereoAndDistinct(t *testing.T) {
	for kind := Step; kind <= Ready; kind++ {
		data := PCM(kind, 0)
		if len(data) == 0 || len(data)%4 != 0 || len(data) > SampleRate*4 {
			t.Fatal("invalid or unbounded sound")
		}
		audible := false
		for i := 0; i < len(data); i += 4 {
			left := int16(binary.LittleEndian.Uint16(data[i:]))
			right := int16(binary.LittleEndian.Uint16(data[i+2:]))
			if left != right || left > 18000 || left < -18000 {
				t.Fatal("invalid stereo amplitude")
			}
			audible = audible || left != 0
		}
		if !audible {
			t.Fatal("silent sound")
		}
	}
	if bytes.Equal(PCM(Fire, 0), PCM(Fire, 12)) {
		t.Fatal("weapon sounds indistinguishable")
	}
}
