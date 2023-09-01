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

func Throttle(e Effector, max uint, refill uint, d time.Duration) Effector {
	tokens := max
	var once sync.Once

	return func(ctx context.Context) (int, error) {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		once.Do(func() {
			ticker := time.NewTicker(d)

			go func() {
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return

					case <-ticker.C:
						t := tokens + refill
						if t > max {
							t = max
						}
						tokens = t
					}
				}
			}()
		})
		if tokens <= 0 {
			return 0, errors.New("too many calls")
		}
		tokens--
		return e(ctx)
	}
}

func EmulateThrottle(ctx context.Context) (int, error) {
	i++
	return i, nil
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
	border("[ Debounce pattern ]")
	tries := 6
	delay := 100 * time.Millisecond

	debouncer := DebounceFirst(IncrementFunc, delay)
	fmt.Println("With debouncer:")
	for i := 0; i < tries; i++ {
		res := debouncer(context.Background())
		fmt.Printf("\t[%d] i=%d\n", i, res)
		if i%2 == 0 {
			time.Sleep(delay + 50*time.Millisecond)
		} else {
			time.Sleep(delay - 50*time.Millisecond)
		}
	}
	i = 0
	fmt.Println("Without debouncer:")
	for i := 0; i < tries; i++ {
		res := IncrementFunc(context.Background())
		fmt.Printf("\t[%d] i=%d\n", i, res)
		time.Sleep(250 * time.Millisecond)
	}
	border("[ Retry pattern ]")
	retrier := Retry(EmulateRetry, 5, 200*time.Millisecond)
	_, err := retrier(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	border("[ Throttle pattern ]")

	throttler := Throttle(EmulateThrottle, 5, 1, delay)

	for j := 0; j < 20; j++ {
		time.Sleep(delay / 2)
		val, err := throttler(context.Background())
		if err != nil {
			fmt.Printf("Err: %v\n", err)
			continue
		}
		fmt.Printf("[%d] val=%d\n", j, val)
	}
}

func border(msg string) {
	for i := 0; i < 80; i++ {
		fmt.Print("-")
	}
	fmt.Println()
	fmt.Println(msg)
	for i := 0; i < 80; i++ {
		fmt.Print("-")
	}
	fmt.Println()
}
