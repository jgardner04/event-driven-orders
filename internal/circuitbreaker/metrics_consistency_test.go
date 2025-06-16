package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestMetricsConsistency verifies that totalRequests = totalSuccesses + totalFailures
// across all scenarios including context cancellations
func TestMetricsConsistency(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("context_cancellation", func(t *testing.T) {
		cb := New(Config{
			Name:        "ctx-cancel-test",
			MaxFailures: 10,
			Timeout:     100 * time.Millisecond,
			MaxRequests: 5,
		}, logger)

		// Execute with immediate context cancellation
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := cb.ExecuteContext(ctx, func() error {
			t.Error("Function should not be executed with cancelled context")
			return nil
		})

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}

		// Check metrics - cancelled request should not be counted
		metrics := cb.Metrics()
		totalReq := metrics["total_requests"].(int64)
		totalSucc := metrics["total_successes"].(int64)
		totalFail := metrics["total_failures"].(int64)

		if totalReq != 0 {
			t.Errorf("Context cancelled request was counted: totalRequests=%d", totalReq)
		}

		if totalReq != totalSucc+totalFail {
			t.Errorf("Metrics inconsistency: totalRequests=%d != successes=%d + failures=%d",
				totalReq, totalSucc, totalFail)
		}
	})

	t.Run("mixed_operations", func(t *testing.T) {
		cb := New(Config{
			Name:        "mixed-ops-test",
			MaxFailures: 5,
			Timeout:     100 * time.Millisecond,
			MaxRequests: 3,
		}, logger)

		var wg sync.WaitGroup
		const numOps = 20
		
		successCount := atomic.Int32{}
		failureCount := atomic.Int32{}
		cancelCount := atomic.Int32{}
		openRejectCount := atomic.Int32{}

		for i := 0; i < numOps; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Mix of different operations
				switch id % 4 {
				case 0: // Success
					err := cb.Execute(func() error {
						return nil
					})
					if err == nil {
						successCount.Add(1)
					} else if err == ErrCircuitBreakerOpen {
						openRejectCount.Add(1)
					}

				case 1: // Failure
					err := cb.Execute(func() error {
						return errors.New("test error")
					})
					if err != nil && err != ErrCircuitBreakerOpen {
						failureCount.Add(1)
					} else if err == ErrCircuitBreakerOpen {
						openRejectCount.Add(1)
					}

				case 2: // Context timeout
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
					defer cancel()
					
					err := cb.ExecuteContext(ctx, func() error {
						time.Sleep(10 * time.Millisecond) // Longer than timeout
						return nil
					})
					
					if err == context.DeadlineExceeded {
						cancelCount.Add(1)
					} else if err == ErrCircuitBreakerOpen {
						openRejectCount.Add(1)
					}

				case 3: // Quick operation
					err := cb.Execute(func() error {
						return nil
					})
					if err == nil {
						successCount.Add(1)
					} else if err == ErrCircuitBreakerOpen {
						openRejectCount.Add(1)
					}
				}
			}(i)
		}

		wg.Wait()
		
		// Give time for any async operations to complete
		time.Sleep(50 * time.Millisecond)

		// Verify metrics consistency
		metrics := cb.Metrics()
		totalReq := metrics["total_requests"].(int64)
		totalSucc := metrics["total_successes"].(int64)
		totalFail := metrics["total_failures"].(int64)

		t.Logf("Operations: success=%d, failure=%d, cancelled=%d, rejected=%d",
			successCount.Load(), failureCount.Load(), cancelCount.Load(), openRejectCount.Load())
		t.Logf("Metrics: totalRequests=%d, successes=%d, failures=%d",
			totalReq, totalSucc, totalFail)

		// The key assertion: total = successes + failures
		if totalReq != totalSucc+totalFail {
			t.Errorf("Metrics inconsistency: totalRequests=%d != successes=%d + failures=%d (diff=%d)",
				totalReq, totalSucc, totalFail, totalReq-(totalSucc+totalFail))
		}

		// Verify cancelled requests were not counted
		actualExecuted := int64(successCount.Load() + failureCount.Load())
		if totalReq != actualExecuted {
			t.Errorf("Total requests mismatch: metrics=%d, actual executed=%d",
				totalReq, actualExecuted)
		}
	})

	t.Run("half_open_rejection", func(t *testing.T) {
		cb := New(Config{
			Name:        "half-open-metrics",
			MaxFailures: 1,
			Timeout:     100 * time.Millisecond,
			MaxRequests: 2,
		}, logger)

		// Force to open state
		cb.Execute(func() error {
			return errors.New("force open")
		})

		// Wait for half-open
		time.Sleep(110 * time.Millisecond)

		// Track what happens
		var results []string
		var resultsMu sync.Mutex

		// Launch many concurrent requests
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				err := cb.Execute(func() error {
					resultsMu.Lock()
					results = append(results, "executed")
					resultsMu.Unlock()
					return errors.New("keep half-open")
				})
				
				if err == ErrCircuitBreakerOpen {
					resultsMu.Lock()
					results = append(results, "rejected")
					resultsMu.Unlock()
				}
			}(i)
		}

		wg.Wait()

		metrics := cb.Metrics()
		totalReq := metrics["total_requests"].(int64)
		totalSucc := metrics["total_successes"].(int64) 
		totalFail := metrics["total_failures"].(int64)

		executedCount := 0
		for _, r := range results {
			if r == "executed" {
				executedCount++
			}
		}

		t.Logf("Half-open results: executed=%d, total results=%d", executedCount, len(results))
		t.Logf("Metrics: totalRequests=%d, successes=%d, failures=%d", totalReq, totalSucc, totalFail)

		// Key assertion
		if totalReq != totalSucc+totalFail {
			t.Errorf("Half-open metrics inconsistency: totalRequests=%d != successes=%d + failures=%d",
				totalReq, totalSucc, totalFail)
		}

		// Initial failure + executed requests
		expectedTotal := int64(1 + executedCount)
		if totalReq != expectedTotal {
			t.Errorf("Total requests mismatch: expected=%d, got=%d", expectedTotal, totalReq)
		}
	})
}

// TestMetricsAccuracyUnderLoad verifies metrics remain consistent under high load
func TestMetricsAccuracyUnderLoad(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cb := New(Config{
		Name:        "load-test",
		MaxFailures: 50,
		Timeout:     100 * time.Millisecond,
		MaxRequests: 10,
	}, logger)

	const numGoroutines = 100
	const opsPerGoroutine = 50

	var wg sync.WaitGroup
	actualSuccesses := atomic.Int64{}
	actualFailures := atomic.Int64{}
	actualCancelled := atomic.Int64{}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < opsPerGoroutine; j++ {
				// Vary operation types
				switch (id + j) % 3 {
				case 0: // Success
					if err := cb.Execute(func() error { return nil }); err == nil {
						actualSuccesses.Add(1)
					}
				case 1: // Failure
					if err := cb.Execute(func() error { return errors.New("fail") }); err != nil && err != ErrCircuitBreakerOpen {
						actualFailures.Add(1)
					}
				case 2: // Context cancellation
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
					cb.ExecuteContext(ctx, func() error {
						time.Sleep(time.Millisecond)
						return nil
					})
					cancel()
					actualCancelled.Add(1)
				}
				
				// Small delay to spread load
				if j%10 == 0 {
					time.Sleep(time.Microsecond * 100)
				}
			}
		}(i)
	}

	wg.Wait()

	// Final metrics check
	metrics := cb.Metrics()
	totalReq := metrics["total_requests"].(int64)
	totalSucc := metrics["total_successes"].(int64)
	totalFail := metrics["total_failures"].(int64)

	t.Logf("Load test complete:")
	t.Logf("  Actual: successes=%d, failures=%d, cancelled=%d",
		actualSuccesses.Load(), actualFailures.Load(), actualCancelled.Load())
	t.Logf("  Metrics: totalRequests=%d, successes=%d, failures=%d",
		totalReq, totalSucc, totalFail)

	// The invariant must hold
	if totalReq != totalSucc+totalFail {
		t.Errorf("Metrics inconsistency under load: totalRequests=%d != successes=%d + failures=%d (diff=%d)",
			totalReq, totalSucc, totalFail, totalReq-(totalSucc+totalFail))
	}
}