// Package good implements a close-driven producer, transformation and reduction pipeline.
package good

import "sync"

// Run owns distinct initially empty channels for one invocation. No other
// goroutine may use them. Stages finish by closing their output, not by asking
// downstream consumers to guess a receive count.
func Run(input, output chan int) int {
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		produce(input)
	}()
	go func() {
		defer workers.Done()
		double(input, output)
	}()
	total := sum(output)
	workers.Wait()
	return total
}

func produce(output chan<- int) {
	for range 2 {
		output <- 21
	}
	close(output)
}

func double(input <-chan int, output chan<- int) {
	for value := range input {
		output <- value * 2
	}
	close(output)
}

func sum(input <-chan int) int {
	total := 0
	for value := range input {
		total += value
	}
	return total
}
