package main

func expensiveCalculation(x int) int { return (x*x + 17) ^ (x << 2) }
func normalize(x int) int            { return x & 255 }
func worker(ch chan int) {
	a := expensiveCalculation(42)
	b := a + 10
	c := normalize(b)
	ch <- c
}
func main() {
	ch := make(chan int)
	go worker(ch)
	<-ch
}
