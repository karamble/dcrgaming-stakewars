//go:build desktop

package main

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/karamble/dcrgaming-stakewars/pkg/render"
	"github.com/karamble/dcrgaming-stakewars/pkg/sound"
)

type speaker struct {
	context *audio.Context
	players []*audio.Player
	cache   map[[2]int][]byte
	muted   bool
}

func (s *speaker) stop() {
	for _, p := range s.players {
		p.Close()
	}
	s.players = nil
}
func (s *speaker) play(cues []render.SoundCue, center float64) {
	active := s.players[:0]
	for _, p := range s.players {
		if p.IsPlaying() {
			active = append(active, p)
		} else {
			p.Close()
		}
	}
	s.players = active
	if s.muted {
		return
	}
	if len(cues) == 0 {
		return
	}
	if s.context == nil {
		s.context = audio.NewContext(sound.SampleRate)
		s.cache = make(map[[2]int][]byte)
	}
	for _, cue := range cues {
		if len(s.players) >= 16 {
			break
		}
		key := [2]int{int(cue.Kind), int(cue.Weapon)}
		data := s.cache[key]
		if data == nil {
			data = sound.PCM(key[0], key[1])
			s.cache[key] = data
		}
		p := s.context.NewPlayerFromBytes(data)
		volume := .18 / (1 + math.Abs(cue.X-center)/800)
		if cue.Kind == render.SoundStep {
			volume *= .55
		}
		p.SetVolume(volume)
		p.Play()
		s.players = append(s.players, p)
	}
}
