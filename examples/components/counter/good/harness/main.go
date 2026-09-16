package main

import "github.com/fanmi/go-tla/examples/components/counter/good"

func main() {
	counter := &good.Counter{}
	_ = counter.RunBatch()
}
