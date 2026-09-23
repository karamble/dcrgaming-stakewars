package main

import (
	"flag"
	"fmt"
	"github.com/karamble/dcrgaming-stakewars/pkg/replay"
	tr "github.com/karamble/dcrgaming-stakewars/research/turntrace"
	"os"
)

func main() {
	file := flag.String("record", "pkg/replay/testdata/wide-six-squads.json", "replay fixture")
	ticks := flag.Uint("ticks", 512, "prefix ticks, <=20000")
	flag.Parse()
	if *ticks > tr.MaxTicks {
		panic("too many ticks")
	}
	f, e := os.Open(*file)
	must(e)
	defer f.Close()
	r, e := replay.Read(f)
	must(e)
	a, size, e := tr.Prefix(r, uint32(*ticks))
	must(e)
	lie := append([]tr.Hash(nil), a.States...)
	at := len(lie) / 2
	for i := at; i < len(lie); i++ {
		lie[i][0] ^= 1
	}
	b, e := tr.New(a.Context, lie)
	must(e)
	d, e := tr.Narrow(a.Commitment, b.Commitment, a.Open, b.Open)
	must(e)
	fmt.Printf("actual ticks=%d midpoint rounds=%d disputed states=%d->%d max state bytes=%d single opening bytes=%d\n", *ticks, d.Rounds, d.Before, d.After, size, 36+32*len(a.Open(d.After).Siblings))
	fmt.Println("Local experiment only: no on-chain tick verifier, canonical-history proof, timeout protocol, or transaction-count guarantee.")
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
