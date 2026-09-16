package main

import "github.com/fanmi/go-tla/examples/components/pipeline/earlyclose"

func main() {
	input, output := make(chan int), make(chan int)
	_ = earlyclose.Run(input, output)
}
