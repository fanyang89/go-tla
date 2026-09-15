package main

func send(ch chan int) { ch <- 1 }
func main() {
	a := make(chan int)
	b := make(chan int, 1)
	go send(a)
	go send(b)
	select {
	case <-a:
	case <-b:
	default:
	}
}
