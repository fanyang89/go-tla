package main

func worker(ch chan int) { ch <- 1; ch <- 2 }
func main() {
	ch := make(chan int, 2)
	go worker(ch)
	<-ch
	<-ch
	close(ch)
}
