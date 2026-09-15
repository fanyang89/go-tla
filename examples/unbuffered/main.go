package main

func worker(ch chan int) { ch <- 1 }
func main() {
	ch := make(chan int)
	go worker(ch)
	<-ch
}
