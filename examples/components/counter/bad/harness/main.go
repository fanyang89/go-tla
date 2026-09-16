package main

import "github.com/fanmi/go-tla/examples/components/counter/bad"

func main() {
	counter := &bad.Counter{}
	_ = counter.RunBatch()
}
