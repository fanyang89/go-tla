package main

import "sync"

func worker(wg *sync.WaitGroup) { wg.Done() }
func main() {
	var wg sync.WaitGroup
	wg.Add(2)
	go worker(&wg)
	go worker(&wg)
	wg.Wait()
}
