"""Bounded checkpoint-claim control model, NOT a Decred script or OTS verifier.

Assumptions abstracted here:
* Start authenticates one fixed-roster participant's transaction signature.
* `valid` is an ideal full-certificate predicate, not implemented cryptography.
* A verification transition enforces the exact fragment and immutable statement.
* Abort/finalize transitions become available after their actual chain windows.
* One honest participant retains a complete valid target certificate and state,
  submits on time, and its transactions get timely chain inclusion.
* Counter/roster/window/fee bounds were agreed before funding.

No wall-clock timeout, chain, fee, data-publication or script-budget proof is
implied. Timer waits are collapsed into their eventual eligible transition.
"""
from dataclasses import dataclass
from collections import deque
import unittest


@dataclass(frozen=True)
class State:
    best: int = 0
    failed: int = 0
    claimant: int = -1
    candidate: int = 0
    remaining: int = 0
    valid: bool = False


class Model:
    def __init__(self, seats=6, maximum=3, fragments=3):
        self.n, self.maximum, self.fragments = seats, maximum, fragments

    def start(self, s, actor, seq, valid):
        if s.claimant >= 0 or not 0 <= actor < self.n:
            raise ValueError("unknown participant or busy verifier")
        if s.failed & (1 << actor):
            raise ValueError("participant attempt exhausted at this certified head")
        if not s.best < seq <= self.maximum:
            raise ValueError("candidate must be strictly newer and within agreed bound")
        return State(s.best, s.failed, actor, seq, self.fragments, valid)

    def advance(self, s):
        if s.claimant < 0:
            raise ValueError("no candidate")
        if s.remaining > 1:
            return State(s.best, s.failed, s.claimant, s.candidate,
                         s.remaining-1, s.valid)
        if not s.valid:
            raise ValueError("ideal full-certificate predicate rejects candidate")
        # Only complete verification can change best or clear failure attempts.
        return State(best=s.candidate)

    def abort(self, s):
        if s.claimant < 0:
            raise ValueError("no candidate")
        return State(s.best, s.failed | (1 << s.claimant))

    def rank(self, s):
        outer = ((self.maximum-s.best)*(self.n+1)
                 + self.n-s.failed.bit_count())
        inner = self.fragments+1 if s.claimant < 0 else s.remaining
        return outer*(self.fragments+2)+inner

    def successors(self, s, honest=False):
        if s.claimant < 0:
            for actor in range(self.n):
                if s.failed & (1 << actor):
                    continue
                # Honest actor 0 already holds the complete target certificate.
                if honest and actor == 0:
                    if s.best < self.maximum:
                        yield self.start(s, actor, self.maximum, True)
                    continue
                for seq in range(s.best+1, self.maximum+1):
                    for valid in (False, True):
                        yield self.start(s, actor, seq, valid)
        else:
            # Timely honest proof supply/inclusion rules out its timeout abort.
            if not (honest and s.claimant == 0):
                yield self.abort(s)
            if s.remaining > 1 or s.valid:
                yield self.advance(s)


class Claims(unittest.TestCase):
    def test_every_progress_edge_strictly_decreases_finite_rank(self):
        m = Model()
        queue, seen = deque([State()]), {State()}
        while queue:
            state = queue.popleft()
            for nxt in m.successors(state):
                self.assertLess(m.rank(nxt), m.rank(state))
                self.assertGreaterEqual(nxt.best, state.best)
                if nxt not in seen:
                    seen.add(nxt)
                    queue.append(nxt)
        print(f"explored {len(seen)} abstract states; every progress edge decreases rank")

    def test_one_honest_against_five_has_no_nonterminal_dead_end(self):
        m = Model()
        queue, seen = deque([State()]), {State()}
        terminal = 0
        while queue:
            state = queue.popleft()
            self.assertFalse(state.failed & 1, "other actors consumed honest slot")
            nxts = tuple(m.successors(state, honest=True))
            if not nxts:
                self.assertEqual(state.best, m.maximum)
                self.assertEqual(state.claimant, -1)
                terminal += 1
            for nxt in nxts:
                self.assertLess(m.rank(nxt), m.rank(state))
                if nxt not in seen:
                    seen.add(nxt)
                    queue.append(nxt)
        self.assertGreater(terminal, 0)
        print(f"one-honest model explored {len(seen)} states; all maximal paths reach verified target")

    def test_invalid_partial_claim_never_advances_certified_head(self):
        m=Model(fragments=102)
        s=m.start(State(), 5, m.maximum, False)
        for _ in range(101):
            s=m.advance(s)
            self.assertEqual(s.best,0)
        with self.assertRaises(ValueError):
            m.advance(s)
        s=m.abort(s)
        self.assertEqual(s.best,0)
        self.assertEqual(s.failed,1<<5)
        with self.assertRaises(ValueError):
            m.start(s,5,1,False)

    def test_sybil_and_digest_changes_do_not_restore_attempt(self):
        m=Model()
        s=m.abort(m.start(State(),3,1,False))
        for seq in range(1,m.maximum+1):
            with self.assertRaises(ValueError):
                m.start(s,3,seq,False)
        for actor in (-1,6,99):
            with self.assertRaises(ValueError):
                m.start(s,actor,1,False)

    def test_unsafe_abort_reset_reproduces_infinite_retry_cycle(self):
        m=Model()
        initial=State()
        partial=m.start(initial,5,1,False)
        # Rejected design: forgetting failed claimant returns to exact initial.
        unsafe_abort=State(best=partial.best)
        self.assertEqual(unsafe_abort,initial)
        self.assertEqual(m.start(unsafe_abort,5,1,False),partial)
        print("counterexample: reset-on-abort recreates identical claim cycle")

    def test_unsafe_header_high_watermark_excludes_real_certificate(self):
        m=Model()
        partial=m.start(State(),5,m.maximum,False)
        # Rejected design: advancing best from unverified nomination header.
        unsafe_abort=State(best=partial.candidate,failed=1<<5)
        with self.assertRaises(ValueError):
            m.start(unsafe_abort,0,m.maximum,True)
        print("counterexample: unverified maximum sequence blocks honest target certificate")


if __name__ == "__main__":
    unittest.main(verbosity=2)
