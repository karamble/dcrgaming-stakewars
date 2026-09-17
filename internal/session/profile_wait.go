package session

import (
	"context"
	"errors"
	"time"
)

var ErrProfileInUse = errors.New("this player profile is already open in another StakeWars window")

// AcquireProfileInstance prevents two desktop processes from presenting the
// same player profile. The lease is released by the kernel even after a crash;
// the empty lock file may safely remain on disk.
func AcquireProfileInstance(dir string) (func() error, error) {
	release, busy, err := acquireInstanceLock(dir)
	if err != nil {
		return nil, err
	}
	if busy {
		return nil, ErrProfileInUse
	}
	return release, nil
}

// WaitForProfile waits until no other StakeWars runtime owns this player's
// durable table and spend stores. It performs no bridge calls while waiting,
// so an accidental second launch cannot flap the remote game connection.
func WaitForProfile(ctx context.Context, dir string) (waited bool, err error) {
	for {
		release, busy, err := acquireProfileProbe(dir)
		if err != nil {
			return waited, err
		}
		if !busy {
			if err := release(); err != nil {
				return waited, err
			}
			return waited, nil
		}
		waited = true
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return waited, ctx.Err()
		case <-timer.C:
		}
	}
}
