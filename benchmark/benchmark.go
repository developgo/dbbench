package benchmark

import (
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"text/template"
	"time"
)

// Bencher is the interface a benchmark has to impelement.
type Bencher interface {
	Setup()
	Cleanup()
	Benchmarks() []Benchmark
	Exec(string)
}

// BenchType determines if the particular benchmark should be run several times or only once.
type BenchType int

const (
	// TypeLoop executes the benchmark several times.
	TypeLoop BenchType = iota
	// TypeOnce executes the benchmark once.
	TypeOnce BenchType = iota
)

// Benchmark contains the benchmark name, its db statement and its type.
type Benchmark struct {
	Name     string
	Type     BenchType
	Parallel bool
	Stmt     string
}

// Run executes the benchmark.
func Run(bencher Bencher, b Benchmark, iter, threads int) time.Duration {
	t := template.New(b.Name)
	t, err := t.Parse(b.Stmt)
	if err != nil {
		log.Fatalf("failed to parse template: %v", err)
	}

	start := time.Now()
	switch b.Type {
	case TypeOnce:
		if b.Parallel {
			go once(bencher, t)
		} else {
			once(bencher, t)
		}
	case TypeLoop:
		if b.Parallel {
			go loop(bencher, t, iter, threads)
		} else {
			loop(bencher, t, iter, threads)
		}
	}

	return time.Since(start)
}

// loop runs the benchmark concurrently several times.
func loop(bencher Bencher, t *template.Template, iterations, threads int) {
	wg := &sync.WaitGroup{}
	wg.Add(threads)
	defer wg.Wait()

	// start as many routines as specified
	for routine := 0; routine < threads; routine++ {
		// calculate the amount of iterations to execute in this routine
		from := ((iterations / threads) * routine) + 1
		to := (iterations / threads) * (routine + 1)

		// Add the remainder of iterations to the last routine.
		if routine == threads-1 {
			remainder := iterations - to
			to += remainder
		}

		// start the routine
		go func(gofrom, togo int) {
			defer wg.Done()
			// notify channel for SIGINT (ctrl-c)
			sigchan := make(chan os.Signal, 1)
			signal.Notify(sigchan, os.Interrupt)

			for i := gofrom; i <= togo; i++ {
				select {
				case <-sigchan:
					// got SIGINT, stop benchmarking
					return
				default:
					// build and execute the statement
					stmt := buildStmt(t, i)
					bencher.Exec(stmt)
				}
			}
		}(from, to)
	}
}

// once runs the benchmark a single time.
func once(bencher Bencher, t *template.Template) {
	stmt := buildStmt(t, 1)
	bencher.Exec(stmt)
}

// buildStmt parses the given template with variables and functions to a pure DB statement.
func buildStmt(t *template.Template, i int) string {
	sb := &strings.Builder{}

	data := struct {
		Iter            int
		Seed            func(int64)
		RandInt63       func() int64
		RandInt63n      func(int64) int64
		RandFloat32     func() float32
		RandFloat64     func() float64
		RandExpFloat64  func() float64
		RandNormFloat64 func() float64
	}{
		Iter:            i,
		Seed:            rand.Seed,
		RandInt63:       rand.Int63,
		RandInt63n:      rand.Int63n,
		RandFloat32:     rand.Float32,
		RandFloat64:     rand.Float64,
		RandExpFloat64:  rand.ExpFloat64,
		RandNormFloat64: rand.NormFloat64,
	}
	if err := t.Execute(sb, data); err != nil {
		log.Fatalf("failed to execute template: %v", err)
	}
	return sb.String()
}
