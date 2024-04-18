package enhancedlivenessprobe

import (
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	// Create a token bucket with a rate of 100 token per second and a capacity of 500
	bucketCapacity = 500
	TokenRate      = 100
)

func TestTokenBucketFillRate(t *testing.T) {
	tb := NewTokenBucket(time.Duration(TokenRate), bucketCapacity)
	defer tb.Stop()

	time.Sleep(time.Second)

	actualTokens := float64(tb.currentTokens())
	expectedTokens := float64(100)
	t.Logf("Get actual tokens: %v", actualTokens)
	if !approximatelyEqual(actualTokens, expectedTokens, 5) {
		t.Errorf("Expected %v tokens after 1 second, got %v", expectedTokens, actualTokens)
	}
}

// define a helper function to check if two floats are approximately equal
func approximatelyEqual(a, b float64, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestTokenBucketInitialization(t *testing.T) {
	tb := NewTokenBucket(time.Duration(TokenRate), bucketCapacity)
	defer tb.Stop()

	if tb.rate != 100 {
		t.Errorf("Expected rate %v, got %v", 100, tb.rate)
	}
	if tb.capacity != 500 {
		t.Errorf("Expected capacity %v, got %v", 500, tb.capacity)
	}
}

func TestTokenBucketConsumption(t *testing.T) {
	tb := NewTokenBucket(time.Duration(TokenRate), bucketCapacity)
	time.Sleep(2 * time.Second)

	tokensToConsume := 75
	// try to consume some tokens
	success := tb.Consume(tokensToConsume)
	if !success {
		t.Errorf("Failed to consume %v tokens when they should be available", tokensToConsume)
	}

	remainingTokens := tb.currentTokens()
	if remainingTokens+tokensToConsume > bucketCapacity {
		t.Errorf("Token count not properly decremented; expected %d, got %d",
			bucketCapacity-tokensToConsume, remainingTokens)
	}
}

func TestTokenBucketOverflowProtection(t *testing.T) {
	tb := NewTokenBucket(time.Duration(TokenRate), bucketCapacity)
	defer tb.Stop()

	// 等待足够长时间使桶充满
	time.Sleep(time.Duration(bucketCapacity/TokenRate) * time.Second)

	t.Logf("Current token count: %v", tb.currentTokens())
	if tb.currentTokens() > bucketCapacity {
		t.Errorf("Token count exceeded maximum capacity of %d", bucketCapacity)
	}
}

func TestTokenBucketConcurrency(t *testing.T) {
	// enable to start parallel test
	t.Parallel()

	tb := NewTokenBucket(time.Duration(TokenRate), bucketCapacity)
	defer tb.Stop()

	numConsumers := 10
	numTokensPerConsumer := 20
	consumerDuration := 500 * time.Millisecond

	var wg sync.WaitGroup
	wg.Add(numConsumers)

	for i := 0; i < numConsumers; i++ {
		go func(id int) {
			defer wg.Done()
			startTime := time.Now()
			for time.Since(startTime) < consumerDuration {
				if tb.Consume(numTokensPerConsumer) {
					t.Logf("Consumer %v consumed a token", id)
				} else {
					t.Logf("Consumer %v faield to consume a token", id)
				}
				time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// to check whether total consumed tokens is less than bucket capacity
	assert.GreaterOrEqual(t, tb.currentTokens(), 0, "Bucket token count should never b negative")
	assert.LessOrEqualf(t, tb.currentTokens(), tb.capacity, "Bucket token count should not exceed the bucket capacity")

	expectedConsumedTokens := numConsumers * numTokensPerConsumer
	t.Logf("expectedConsumedTokens: %v", expectedConsumedTokens)
	assert.InEpsilon(t, tb.TotalConsumedTokens(), expectedConsumedTokens, float64(expectedConsumedTokens)*0.1, "Total consumed tokens should be close to the expected value within a 10% margin")

}
