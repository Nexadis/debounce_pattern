package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

var (
	i        int
	attempts int
)

type (
	Circuit  func(ctx context.Context) int
	Effector func(ctx context.Context) (int, error)
)

func Retry(effector Effector, retries int, delay time.Duration) Effector {
	return func(ctx context.Context) (int, error) {
		for r := 0; ; r++ {
			response, err := effector(ctx)
			if err == nil || r >= retries {
				return response, err
			}
			log.Printf("Attempt %d failed; retrying in %v", r+1, delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
	}
}

func EmulateRetry(ctx context.Context) (int, error) {
	attempts++
	if attempts <= 3 {
		return 0, errors.New("service unavailable")
	}
	return attempts, nil
}

func DebounceFirst(circuit Circuit, d time.Duration) Circuit {
	var threshold time.Time
	var result int
	var m sync.Mutex
	return func(ctx context.Context) int {
		m.Lock()
		defer func() {
			threshold = time.Now().Add(d)
			m.Unlock()
		}()

		if time.Now().Before(threshold) {
			return result
		}
		result = circuit(ctx)
		return result
	}
}

func DebounceLast(circuit Circuit, d time.Duration) Circuit {
	var threshold time.Time = time.Now()
	var ticker *time.Ticker
	var result int
	var once sync.Once
	var m sync.Mutex
	return func(ctx context.Context) int {
		m.Lock()
		defer m.Unlock()
		threshold = time.Now().Add(d)

		once.Do(func() {
			ticker = time.NewTicker(time.Millisecond * 100)

			go func() {
				defer func() {
					m.Lock()
					ticker.Stop()
					once = sync.Once{}
					m.Unlock()
				}()
				for {
					select {
					case <-ticker.C:
						m.Lock()
						if time.Now().After(threshold) {
							result = circuit(ctx)
							m.Unlock()
							return
						}
						m.Unlock()
					case <-ctx.Done():
						m.Lock()
						result = 0
						m.Unlock()
						return
					}
				}
			}()
		})
		return result
	}
}

func IncrementFunc(ctx context.Context) int {
	i++
	return i
}

func main() {
	tries := 6
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

	retrier := Retry(EmulateRetry, 5, 200*time.Millisecond)
	_, err := retrier(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}
