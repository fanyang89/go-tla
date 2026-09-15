package main

import "sync"

func worker(mu *sync.Mutex, done chan int) {
	mu.Lock()
	mu.Unlock()
	done <- 1
}
func main() {
	var mu sync.Mutex
	done := make(chan int)
	go worker(&mu, done)
	go worker(&mu, done)
	<-done
	<-done
}
