package main

import "github.com/fanmi/go-tla/examples/components/pipeline/missingclose"

func main() {
	input, output := make(chan int), make(chan int)
	_ = missingclose.Run(input, output)
}
