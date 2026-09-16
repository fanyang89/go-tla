package main

import "github.com/fanmi/go-tla/examples/components/workerpool/good"

func main() {
	pool := &good.Pool{Jobs: make(chan int), Results: make(chan int)}
	_ = pool.RunBatch()
}
