package circuitbreaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestHalfOpenAtomicBehavior tests that the increment-then-check pattern
// correctly prevents race conditions in half-open state
func TestHalfOpenAtomicBehavior(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Run test many times to catch race conditions
	const iterations = 100
	raceDetected := false

	for i := 0; i < iterations && !raceDetected; i++ {
		cb := New(Config{
			Name:        "atomic-test",
			MaxFailures: 1,
			Timeout:     100 * time.Millisecond,
			MaxRequests: 2, // Critical: small number to make race more likely
		}, logger)

		// Force circuit to open state
		cb.Execute(func() error {
			return errors.New("force open")
		})

		// Wait for timeout to transition to half-open
		time.Sleep(110 * time.Millisecond)

		// Use many goroutines to maximize race condition probability
		const numGoroutines = 50
		var wg sync.WaitGroup
		var startBarrier sync.WaitGroup
		startBarrier.Add(1)

		// Track actual executions
		executionCounter := atomic.Int32{}
		
		// Track which goroutines got past the check
		passedCheckCount := atomic.Int32{}
		
		for j := 0; j < numGoroutines; j++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Wait at barrier to ensure all goroutines start simultaneously
				startBarrier.Wait()
				
				err := cb.Execute(func() error {
					// If we're here, we passed all checks
					passedCheckCount.Add(1)
					executionCounter.Add(1)
					
					// Keep circuit in half-open by returning error
					return errors.New("keep half-open")
				})
				
				// Count only actual executions, not rejections
				if err != nil && err != ErrCircuitBreakerOpen {
					// This was an execution that returned an error
				}
			}(j)
		}

		// Release all goroutines at exactly the same time
		startBarrier.Done()
		wg.Wait()

		executed := executionCounter.Load()
		passed := passedCheckCount.Load()

		// The critical assertion: executions should NEVER exceed maxRequests
		if executed > int32(cb.maxRequests) {
			raceDetected = true
			t.Errorf("RACE CONDITION: Iteration %d - %d functions executed, %d passed checks (max allowed: %d)",
				i, executed, passed, cb.maxRequests)
		}

		// Log for debugging
		if executed == int32(cb.maxRequests) {
			// This is the expected case - exactly maxRequests executed
			t.Logf("Iteration %d: Correctly limited to %d executions", i, executed)
		}
	}

	if !raceDetected {
		t.Logf("No race condition detected after %d iterations", iterations)
	}
}

// TestHalfOpenStrictOrdering validates that the circuit breaker maintains
// strict ordering and counting in half-open state
func TestHalfOpenStrictOrdering(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cb := New(Config{
		Name:        "ordering-test",
		MaxFailures: 1,
		Timeout:     100 * time.Millisecond,
		MaxRequests: 3,
	}, logger)

	// Force open
	cb.Execute(func() error {
		return errors.New("force open")
	})

	// Wait for half-open
	time.Sleep(110 * time.Millisecond)

	// Create a controlled execution environment
	const numRequests = 10
	results := make([]struct {
		executed bool
		order    int
	}, numRequests)
	
	executionOrder := atomic.Int32{}
	var orderMutex sync.Mutex
	
	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			
			<-startSignal // Wait for signal
			
			err := cb.Execute(func() error {
				// Record execution order
				order := executionOrder.Add(1)
				orderMutex.Lock()
				results[idx].executed = true
				results[idx].order = int(order)
				orderMutex.Unlock()
				
				// Simulate work
				time.Sleep(10 * time.Millisecond)
				
				// Keep in half-open by failing
				return errors.New("fail")
			})
			
			if err == ErrCircuitBreakerOpen {
				orderMutex.Lock()
				results[idx].executed = false
				orderMutex.Unlock()
			}
		}(i)
	}

	// Start all goroutines
	close(startSignal)
	wg.Wait()

	// Count executions
	executedCount := 0
	var executedOrders []int
	for i, r := range results {
		if r.executed {
			executedCount++
			executedOrders = append(executedOrders, r.order)
			t.Logf("Request %d: executed (order=%d)", i, r.order)
		} else {
			t.Logf("Request %d: rejected", i)
		}
	}

	// Validate results
	if executedCount > cb.maxRequests {
		t.Errorf("Too many executions: %d (max: %d)", executedCount, cb.maxRequests)
	}

	// Validate that we have unique sequential orders (though not necessarily in request order)
	orderSet := make(map[int]bool)
	for _, order := range executedOrders {
		if orderSet[order] {
			t.Errorf("Duplicate execution order detected: %d", order)
		}
		orderSet[order] = true
		
		if order < 1 || order > executedCount {
			t.Errorf("Invalid execution order: %d (should be between 1 and %d)", order, executedCount)
		}
	}

	t.Logf("Total executed: %d/%d (max allowed: %d)", executedCount, numRequests, cb.maxRequests)
}

// TestHalfOpenBoundaryConditions tests edge cases in half-open state
func TestHalfOpenBoundaryConditions(t *testing.T) {
	testCases := []struct {
		name        string
		maxRequests int
		concurrent  int
		expected    string
	}{
		{
			name:        "single_request_allowed",
			maxRequests: 1,
			concurrent:  10,
			expected:    "exactly 1 execution",
		},
		{
			name:        "zero_requests_allowed",  
			maxRequests: 0,
			concurrent:  10,
			expected:    "0 executions (configuration edge case)",
		},
		{
			name:        "high_concurrency",
			maxRequests: 5,
			concurrent:  1000,
			expected:    "at most 5 executions",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			// Note: maxRequests of 0 will be adjusted to 1 by validation
			cb := New(Config{
				Name:        tc.name,
				MaxFailures: 1,
				Timeout:     100 * time.Millisecond,
				MaxRequests: tc.maxRequests,
			}, logger)

			// Force open
			cb.Execute(func() error {
				return errors.New("force open")
			})

			// Wait for half-open
			time.Sleep(110 * time.Millisecond)

			// Launch concurrent requests
			var wg sync.WaitGroup
			executionCount := atomic.Int32{}
			startSignal := make(chan struct{})

			for i := 0; i < tc.concurrent; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					
					<-startSignal
					
					err := cb.Execute(func() error {
						executionCount.Add(1)
						return errors.New("keep half-open")
					})
					
					// We only care about counting executions
					_ = err
				}()
			}

			close(startSignal)
			wg.Wait()

			executed := executionCount.Load()
			
			// Account for validation adjusting 0 to 1
			expectedMax := cb.maxRequests
			if tc.maxRequests == 0 {
				expectedMax = 1 // Config validation changes 0 to 1
			}

			if executed > int32(expectedMax) {
				t.Errorf("Boundary condition failed: %d executed, max allowed: %d (%s)",
					executed, expectedMax, tc.expected)
			} else {
				t.Logf("Boundary test passed: %d executed, max allowed: %d (%s)",
					executed, expectedMax, tc.expected)
			}
		})
	}
}