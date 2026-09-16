package main

import "github.com/fanmi/go-tla/examples/components/pipeline/good"

func main() {
	input, output := make(chan int), make(chan int)
	_ = good.Run(input, output)
}
