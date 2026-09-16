package simfixture

import "time" // want `outside the pure simulation boundary`

var _ = time.Now()
var floating float64 // want `floating-point or complex value`
type alias = float32 // want `floating-point or complex value`
var _ alias          // want `floating-point or complex value`
type state struct {
	Next   *state // want `pointer or interface field`
	Hidden any    // want `pointer or interface field`
}

func bad() {
	for range map[int]int{} { // want `map iteration`
	}
	go bad()            // want `concurrency construct`
	c := make(chan int) // want `concurrency construct`
	c <- 1              // want `concurrency construct`
	select {}           // want `concurrency construct`
	println("bad")      // want `output in simulation`
}

// Lookup maps and pointers to function arguments remain allowed.
func good(s *state, m map[int]int) int { return m[1] }
