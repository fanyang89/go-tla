package main

import "strings"

// Analyze with -trust-call strings.HasPrefix: the user supplies a total,
// side-effect-free contract. Its result is deliberately not evaluated.
func worker(ch chan int) {
	if strings.HasPrefix("request", "retry") {
		ch <- 1
	}
}
func main() {
	ch := make(chan int)
	go worker(ch)
	<-ch
}
