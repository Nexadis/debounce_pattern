package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var i int

type Circuit func(ctx context.Context) int

func DebounceFirst(circuit Circuit, d time.Duration) Circuit {
	var threshold time.Time
	var result int
	var m sync.Mutex
	return func(ctx context.Context) int {
		m.Lock()
		defer m.Unlock()

		if time.Now().Before(threshold) {
			fmt.Println("Return cached result")
			return result
		}
		result = circuit(ctx)
		threshold = time.Now().Add(d)
		return result
	}
}

func IncrementFunc(ctx context.Context) int {
	i++
	return i
}

func main() {
	tries := 20
	delay := 1 * time.Second

	debouncer := DebounceFirst(IncrementFunc, delay)
	fmt.Println("With debouncer:")
	for i := 0; i < tries; i++ {
		res := debouncer(context.Background())
		fmt.Printf("\t[%d] i=%d\n", i, res)
		if i%2 == 0 {
			time.Sleep(250 * time.Millisecond)
		} else {
			time.Sleep(150 * time.Millisecond)
		}
	}
	i = 0
	fmt.Println("Without debouncer:")
	for i := 0; i < tries; i++ {
		res := IncrementFunc(context.Background())
		fmt.Printf("\t[%d] i=%d\n", i, res)
		time.Sleep(250 * time.Millisecond)
	}
}
