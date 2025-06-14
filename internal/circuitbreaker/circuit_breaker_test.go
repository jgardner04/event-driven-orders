package circuitbreaker

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestExecuteConcurrentAccess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	cb := New(Config{
		Name:        "test",
		MaxFailures: 3,
		Timeout:     100 * time.Millisecond,
		MaxRequests: 2,
	}, logger)

	// Test concurrent access without race conditions
	const numGoroutines = 100
	const numIterations = 10

	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines*numIterations)

	// Function that sometimes fails
	testFunc := func() error {
		time.Sleep(1 * time.Millisecond) // Simulate some work
		if time.Now().UnixNano()%3 == 0 {
			return errors.New("simulated failure")
		}
		return nil
	}

	// Launch concurrent goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				err := cb.Execute(testFunc)
				if err != nil {
					errorChan <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errorChan)

	// Collect results
	var errorCount int
	for err := range errorChan {
		if err != nil {
			errorCount++
		}
	}

	// Verify metrics are consistent
	metrics := cb.Metrics()
	totalRequests := metrics["total_requests"].(int64)
	totalFailures := metrics["total_failures"].(int64)
	totalSuccesses := metrics["total_successes"].(int64)

	// Basic sanity checks
	if totalRequests != totalFailures+totalSuccesses {
		t.Errorf("Inconsistent metrics: total_requests=%d, total_failures=%d, total_successes=%d",
			totalRequests, totalFailures, totalSuccesses)
	}

	if totalRequests <= 0 {
		t.Error("Expected some requests to be processed")
	}

	t.Logf("Processed %d requests with %d failures and %d successes",
		totalRequests, totalFailures, totalSuccesses)
	t.Logf("Circuit breaker final state: %s", cb.State().String())
}

func TestExecuteChannelBasedExecution(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cb := New(Config{
		Name:        "channel-test",
		MaxFailures: 2,
		Timeout:     50 * time.Millisecond,
		MaxRequests: 1,
	}, logger)

	// Test that function execution happens asynchronously but results are properly collected
	executionOrder := make([]int, 0)
	var mu sync.Mutex

	slowFunc := func(id int) func() error {
		return func() error {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			executionOrder = append(executionOrder, id)
			mu.Unlock()
			return nil
		}
	}

	// Execute multiple functions
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := cb.Execute(slowFunc(id))
			if err != nil {
				t.Errorf("Unexpected error for execution %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all executions completed
	mu.Lock()
	if len(executionOrder) != 3 {
		t.Errorf("Expected 3 executions, got %d", len(executionOrder))
	}
	mu.Unlock()

	// Verify metrics
	metrics := cb.Metrics()
	if metrics["total_requests"].(int64) != 3 {
		t.Errorf("Expected 3 total requests, got %d", metrics["total_requests"])
	}
	if metrics["total_successes"].(int64) != 3 {
		t.Errorf("Expected 3 successes, got %d", metrics["total_successes"])
	}
}

func TestExecuteHalfOpenConcurrency(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cb := New(Config{
		Name:        "half-open-test",
		MaxFailures: 1,
		Timeout:     50 * time.Millisecond,
		MaxRequests: 2, // Allow 2 requests in half-open
	}, logger)

	// Force circuit breaker to open
	err := cb.Execute(func() error {
		return errors.New("force failure")
	})
	if err == nil {
		t.Error("Expected failure to open circuit breaker")
	}

	if cb.State() != StateOpen {
		t.Errorf("Expected circuit breaker to be open, got %s", cb.State().String())
	}

	// Wait for timeout to transition to half-open
	time.Sleep(60 * time.Millisecond)

	// Test concurrent access in half-open state
	var wg sync.WaitGroup
	results := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cb.Execute(func() error {
				time.Sleep(5 * time.Millisecond)
				return nil // Success
			})
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// Count results
	var successCount, rejectedCount int
	for err := range results {
		if err == ErrCircuitBreakerOpen {
			rejectedCount++
		} else if err == nil {
			successCount++
		} else {
			t.Errorf("Unexpected error: %v", err)
		}
	}

	// In half-open state with MaxRequests=2, we should have exactly 2 successes
	// and 3 rejections
	if successCount != 2 {
		t.Errorf("Expected 2 successes in half-open state, got %d", successCount)
	}
	if rejectedCount != 3 {
		t.Errorf("Expected 3 rejections in half-open state, got %d", rejectedCount)
	}

	// Circuit breaker should now be closed after successful executions
	if cb.State() != StateClosed {
		t.Errorf("Expected circuit breaker to be closed after successes, got %s", cb.State().String())
	}
}
