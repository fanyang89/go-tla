package main

import "github.com/fanmi/go-tla/examples/components/workerpool/bad"

func main() {
	pool := &bad.Pool{Jobs: make(chan int), Results: make(chan int)}
	_ = pool.RunBatch()
}
