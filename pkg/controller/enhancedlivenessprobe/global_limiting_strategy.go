package enhancedlivenessprobe

import (
	"fmt"
	"sync"
	"time"
)

type Interface interface {
	// the process is satified with the global limiting strategy
	LimitingStrategyAllowed() bool
}

// For global limiting strategy, the token bucket is used.
// The interface limitingStrategyAllowed is used to check if the process is satisfied with the global limiting strategy.
type TokenBucket struct {
	rate                  time.Duration //token generate rate, r/s
	capacity              int           //bucket capacity
	tokens                chan struct{} //numbers of token in bucket
	fillTicker            *time.Ticker
	stopCh                chan struct{} // stop the refill goroutine
	lock                  sync.Mutex
	totalConsumedTokens   int // total consumed tokens
	totalConsumedTokensMu sync.Mutex
}

func NewTokenBucket(rate time.Duration, capacity int) *TokenBucket {
	tb := &TokenBucket{
		rate:                  rate,
		capacity:              capacity,
		tokens:                make(chan struct{}, capacity),
		fillTicker:            time.NewTicker(time.Second / time.Duration(rate)),
		stopCh:                make(chan struct{}),
		totalConsumedTokens:   0,
		totalConsumedTokensMu: sync.Mutex{},
	}
	// start a dependent goroutine to refill the bucket
	go tb.refill()
	return tb
}

func (t *TokenBucket) currentTokens() int {
	t.lock.Lock()
	defer t.lock.Unlock()
	return len(t.tokens)
}

func (t *TokenBucket) refill() {
	for {
		select {
		case <-t.fillTicker.C:
			t.lock.Lock()
			if len(t.tokens) < t.capacity {
				t.tokens <- struct{}{} // 填充一个令牌
			}
			t.lock.Unlock()
		case <-t.stopCh:
			t.fillTicker.Stop()
			return
		}
	}
}

func (t *TokenBucket) addToken() {
	t.lock.Lock()
	defer t.lock.Unlock()

	select {
	case t.tokens <- struct{}{}:
	default:
		fmt.Println("Token bucket is full, discarding a token")
	}
}

func (t *TokenBucket) Consume(numTokens int) bool {
	t.lock.Lock()
	defer t.lock.Unlock()

	// the capacity of the bucket is less than the numTokens
	if numTokens <= 0 || numTokens > len(t.tokens) {
		return false
	}
	for i := 0; i < numTokens; i++ {
		<-t.tokens
	}

	t.totalConsumedTokensMu.Lock()
	t.totalConsumedTokens += numTokens
	t.totalConsumedTokensMu.Unlock()

	return true
}

func (t *TokenBucket) Stop() {
	close(t.stopCh)
}

func (t *TokenBucket) TotalConsumedTokens() int {
	t.totalConsumedTokensMu.Lock()
	defer t.totalConsumedTokensMu.Unlock()

	return t.totalConsumedTokens
}

// just consume one token
func (t *TokenBucket) LimitingStrategyAllowed() bool {
	return globalBucket.Consume(1)
}
