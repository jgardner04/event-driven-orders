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

func TestTotalRequestsAccuracy(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cb := New(Config{
		Name:        "accuracy-test",
		MaxFailures: 1,
		Timeout:     100 * time.Millisecond,
		MaxRequests: 1,
	}, logger)

	// Force circuit breaker to open by causing a failure
	err := cb.Execute(func() error {
		return errors.New("force failure")
	})
	if err == nil {
		t.Error("Expected failure to open circuit breaker")
	}

	if cb.State() != StateOpen {
		t.Errorf("Expected circuit breaker to be open, got %s", cb.State().String())
	}

	// Get initial metrics
	initialMetrics := cb.Metrics()
	initialTotalRequests := initialMetrics["total_requests"].(int64)

	// Try to execute while circuit breaker is open (should be rejected)
	err = cb.Execute(func() error {
		return nil // This should never execute
	})

	if err != ErrCircuitBreakerOpen {
		t.Errorf("Expected ErrCircuitBreakerOpen, got %v", err)
	}

	// Check if totalRequests was incremented for the rejected request
	finalMetrics := cb.Metrics()
	finalTotalRequests := finalMetrics["total_requests"].(int64)

	if finalTotalRequests != initialTotalRequests {
		t.Errorf("totalRequests was incremented for rejected request: initial=%d, final=%d",
			initialTotalRequests, finalTotalRequests)
	}

	t.Logf("Initial total_requests: %d", initialTotalRequests)
	t.Logf("Final total_requests: %d", finalTotalRequests)
	t.Logf("Circuit breaker correctly did not count rejected request")
}

func TestTotalRequestsAccuracyComprehensive(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cb := New(Config{
		Name:        "comprehensive-test",
		MaxFailures: 1,
		Timeout:     50 * time.Millisecond,
		MaxRequests: 2, // Allow 2 requests in half-open
	}, logger)

	// Test 1: Force circuit breaker to open
	err := cb.Execute(func() error {
		return errors.New("force failure")
	})
	if err == nil {
		t.Error("Expected failure to open circuit breaker")
	}

	metrics1 := cb.Metrics()
	totalRequests1 := metrics1["total_requests"].(int64)
	t.Logf("After opening circuit breaker: total_requests=%d", totalRequests1)

	// Test 2: Try multiple requests while open (should all be rejected)
	for i := 0; i < 3; i++ {
		err = cb.Execute(func() error {
			t.Error("This function should never execute while circuit breaker is open")
			return nil
		})
		if err != ErrCircuitBreakerOpen {
			t.Errorf("Expected ErrCircuitBreakerOpen, got %v", err)
		}
	}

	metrics2 := cb.Metrics()
	totalRequests2 := metrics2["total_requests"].(int64)
	t.Logf("After 3 rejected requests (open): total_requests=%d", totalRequests2)

	if totalRequests2 != totalRequests1 {
		t.Errorf("totalRequests incremented for open circuit breaker rejections: %d -> %d",
			totalRequests1, totalRequests2)
	}

	// Test 3: Wait for timeout to transition to half-open
	time.Sleep(60 * time.Millisecond)

	// Test 4: Execute exactly MaxRequests (2) successful requests in half-open
	for i := 0; i < 2; i++ {
		err = cb.Execute(func() error {
			return nil // Success
		})
		if err != nil {
			t.Errorf("Expected success in half-open state, got %v", err)
		}
	}

	metrics3 := cb.Metrics()
	totalRequests3 := metrics3["total_requests"].(int64)
	t.Logf("After 2 successful half-open requests: total_requests=%d", totalRequests3)

	// Should be closed now (standard circuit breaker behavior)
	if cb.State() != StateClosed {
		t.Errorf("Expected circuit breaker to be closed, got %s", cb.State().String())
	}

	// Test 5: Verify that requests in closed state work normally and increment totalRequests
	err = cb.Execute(func() error {
		return nil // Success in closed state
	})
	if err != nil {
		t.Errorf("Expected success in closed state, got %v", err)
	}

	metrics4 := cb.Metrics()
	totalRequests4 := metrics4["total_requests"].(int64)
	t.Logf("After request in closed state: total_requests=%d", totalRequests4)

	if totalRequests4 != totalRequests3+1 {
		t.Errorf("totalRequests should increment for successful requests in closed state: %d -> %d",
			totalRequests3, totalRequests4)
	}

	// Test 6: Test concurrent half-open quota rejection behavior
	// Force open again to test half-open rejection behavior
	err = cb.Execute(func() error {
		return errors.New("force failure for concurrent test")
	})
	if err == nil {
		t.Error("Expected failure to open circuit breaker again")
	}

	// Wait for timeout to go to half-open
	time.Sleep(60 * time.Millisecond)

	// Get metrics before concurrent test
	metrics5 := cb.Metrics()
	totalRequests5 := metrics5["total_requests"].(int64)

	// Launch concurrent requests to test half-open quota behavior
	// With MaxRequests=2, only 2 should succeed, others should be rejected
	const numConcurrentRequests = 5
	var wg sync.WaitGroup
	results := make(chan error, numConcurrentRequests)

	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := cb.Execute(func() error {
				time.Sleep(10 * time.Millisecond) // Simulate work
				return nil
			})
			results <- err
		}(i)
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

	metrics6 := cb.Metrics()
	totalRequests6 := metrics6["total_requests"].(int64)
	requestsIncrement := totalRequests6 - totalRequests5

	t.Logf("Concurrent test - Success count: %d", successCount)
	t.Logf("Concurrent test - Rejected count: %d", rejectedCount)
	t.Logf("Concurrent test - Total requests increment: %d", requestsIncrement)
	t.Logf("Final total_requests: %d", totalRequests6)

	// Verify that totalRequests only incremented for successful requests
	if requestsIncrement > int64(successCount) {
		t.Errorf("totalRequests incremented more than successful requests: increment=%d, successes=%d",
			requestsIncrement, successCount)
		t.Error("This indicates rejected requests are being counted in totalRequests")
	}

	// With MaxRequests=2, we should have at most 2 successes
	if successCount > 2 {
		t.Errorf("Expected at most 2 successes with MaxRequests=2, got %d", successCount)
	}

	// Should have some rejections
	if rejectedCount == 0 {
		t.Error("Expected some requests to be rejected due to half-open quota")
	}

	t.Logf("Final verification: totalRequests correctly incremented only for executed requests")
}

func TestTotalRequestsRaceCondition(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cb := New(Config{
		Name:        "race-test",
		MaxFailures: 1,
		Timeout:     50 * time.Millisecond,
		MaxRequests: 1, // Only allow 1 request in half-open
	}, logger)

	// Force circuit breaker to open
	err := cb.Execute(func() error {
		return errors.New("force failure")
	})
	if err == nil {
		t.Error("Expected failure to open circuit breaker")
	}

	// Wait for timeout to transition to half-open
	time.Sleep(60 * time.Millisecond)

	// Get initial metrics
	initialMetrics := cb.Metrics()
	initialTotalRequests := initialMetrics["total_requests"].(int64)
	t.Logf("Initial total_requests in half-open: %d", initialTotalRequests)

	// Launch multiple concurrent requests to half-open circuit breaker
	// Only 1 should succeed (MaxRequests=1), others should be rejected
	const numConcurrentRequests = 5
	var wg sync.WaitGroup
	results := make(chan error, numConcurrentRequests)

	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := cb.Execute(func() error {
				time.Sleep(10 * time.Millisecond) // Simulate work
				return nil
			})
			results <- err
		}(i)
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

	finalMetrics := cb.Metrics()
	finalTotalRequests := finalMetrics["total_requests"].(int64)
	requestsIncrement := finalTotalRequests - initialTotalRequests

	t.Logf("Success count: %d", successCount)
	t.Logf("Rejected count: %d", rejectedCount)
	t.Logf("Total requests increment: %d", requestsIncrement)
	t.Logf("Final total_requests: %d", finalTotalRequests)

	// The issue: totalRequests might be incremented for requests that should be rejected
	// due to race conditions in the half-open state
	if requestsIncrement > int64(successCount) {
		t.Errorf("totalRequests incremented more than successful requests: increment=%d, successes=%d",
			requestsIncrement, successCount)
		t.Error("This indicates rejected requests are being counted in totalRequests")
	}

	// With MaxRequests=1, we should have exactly 1 success and 4 rejections
	expectedSuccesses := 1
	expectedRejections := numConcurrentRequests - expectedSuccesses

	if successCount != expectedSuccesses {
		t.Errorf("Expected %d successes, got %d", expectedSuccesses, successCount)
	}
	if rejectedCount != expectedRejections {
		t.Errorf("Expected %d rejections, got %d", expectedRejections, rejectedCount)
	}
}
