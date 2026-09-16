// Package good implements a single-use, two-worker batch processor.
package good

import "sync"

const Workers = 2

// Pool requires distinct, initially empty Jobs and Results channels. RunBatch
// has one caller; the caller transfers channel ownership for the batch lifetime.
// The explicit fields also let the analysis harness provide static identities.
type Pool struct {
	Jobs    chan int
	Results chan int
	group   sync.WaitGroup
}

// RunBatch submits one job per worker, collects every result and joins workers.
// Reusing a Pool or concurrently calling RunBatch is outside this API contract.
func (p *Pool) RunBatch() int {
	p.group.Add(Workers)
	for range Workers {
		go p.work()
	}
	for range Workers {
		p.Jobs <- 21
	}
	total := 0
	for range Workers {
		total += <-p.Results
	}
	p.group.Wait()
	return total
}

func (p *Pool) work() {
	defer p.group.Done()
	job := <-p.Jobs
	p.Results <- job * 2
}
