package main

func main() {
	ch := make(chan int)
	ch <- 1 // No receiver: TLC reports a deadlock here.
}
