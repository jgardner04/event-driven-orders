package circuitbreaker

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
)

type Config struct {
	Name            string
	MaxFailures     int
	Timeout         time.Duration
	MaxRequests     int
	OnStateChange   func(name string, from State, to State)
}

type CircuitBreaker struct {
	name         string
	maxFailures  int
	timeout      time.Duration
	maxRequests  int
	onStateChange func(name string, from State, to State)

	mutex      sync.RWMutex
	state      State
	failures   int
	requests   int
	lastFailTime time.Time
	
	// Metrics
	totalRequests   int64
	totalFailures   int64
	totalSuccesses  int64
	stateChanges    int64
	lastStateChange time.Time
	
	logger *logrus.Logger
}

func New(config Config, logger *logrus.Logger) *CircuitBreaker {
	if config.MaxFailures == 0 {
		config.MaxFailures = 5
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRequests == 0 {
		config.MaxRequests = 1
	}

	return &CircuitBreaker{
		name:         config.Name,
		maxFailures:  config.MaxFailures,
		timeout:      config.Timeout,
		maxRequests:  config.MaxRequests,
		onStateChange: config.OnStateChange,
		state:        StateClosed,
		logger:       logger,
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.totalRequests++
	
	if cb.state == StateOpen {
		if time.Since(cb.lastFailTime) > cb.timeout {
			cb.setState(StateHalfOpen)
			cb.requests = 0
		} else {
			cb.logger.WithFields(logrus.Fields{
				"circuit_breaker": cb.name,
				"state": cb.state.String(),
			}).Debug("Circuit breaker is open, rejecting request")
			return ErrCircuitBreakerOpen
		}
	}

	if cb.state == StateHalfOpen && cb.requests >= cb.maxRequests {
		cb.logger.WithFields(logrus.Fields{
			"circuit_breaker": cb.name,
			"state": cb.state.String(),
			"requests": cb.requests,
			"max_requests": cb.maxRequests,
		}).Debug("Circuit breaker half-open max requests reached")
		return ErrCircuitBreakerOpen
	}

	cb.requests++
	
	// Execute the function
	cb.mutex.Unlock()
	err := fn()
	cb.mutex.Lock()

	if err != nil {
		cb.onFailure()
		cb.totalFailures++
		return err
	}

	cb.onSuccess()
	cb.totalSuccesses++
	return nil
}

func (cb *CircuitBreaker) onSuccess() {
	cb.failures = 0
	
	if cb.state == StateHalfOpen {
		cb.setState(StateClosed)
		cb.requests = 0
	}
}

func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailTime = time.Now()
	
	if cb.state == StateClosed && cb.failures >= cb.maxFailures {
		cb.setState(StateOpen)
		cb.requests = 0
	} else if cb.state == StateHalfOpen {
		cb.setState(StateOpen)
		cb.requests = 0
	}
}

func (cb *CircuitBreaker) setState(newState State) {
	if cb.state == newState {
		return
	}
	
	oldState := cb.state
	cb.state = newState
	cb.stateChanges++
	cb.lastStateChange = time.Now()
	
	cb.logger.WithFields(logrus.Fields{
		"circuit_breaker": cb.name,
		"from_state": oldState.String(),
		"to_state": newState.String(),
	}).Info("Circuit breaker state changed")
	
	if cb.onStateChange != nil {
		go cb.onStateChange(cb.name, oldState, newState)
	}
}

func (cb *CircuitBreaker) State() State {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) Metrics() map[string]interface{} {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	
	return map[string]interface{}{
		"name":             cb.name,
		"state":            cb.state.String(),
		"failures":         cb.failures,
		"requests":         cb.requests,
		"total_requests":   cb.totalRequests,
		"total_failures":   cb.totalFailures,
		"total_successes":  cb.totalSuccesses,
		"state_changes":    cb.stateChanges,
		"max_failures":     cb.maxFailures,
		"timeout_seconds":  cb.timeout.Seconds(),
		"max_requests":     cb.maxRequests,
		"last_failure":     cb.lastFailTime.Format(time.RFC3339),
		"last_state_change": cb.lastStateChange.Format(time.RFC3339),
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.setState(StateClosed)
	cb.failures = 0
	cb.requests = 0
	cb.lastFailTime = time.Time{}
}

func (cb *CircuitBreaker) String() string {
	return fmt.Sprintf("CircuitBreaker(name=%s, state=%s, failures=%d/%d)", 
		cb.name, cb.state.String(), cb.failures, cb.maxFailures)
}