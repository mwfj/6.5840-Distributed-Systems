package raft

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

// Debugging
const Debug = false

func DPrintf(format string, a ...interface{}) {
	if Debug {
		log.Printf(format, a...)
	}
}

// as the paper recommended,
// the interval of election timeout should between 150ms ~ 300ms,
// In here, we use 150ms as the basic, the then do the randomization based on it
const ElectionTimeout = 150

// for the timeout of heartbeat message, it should less than ElectionTimeout
// to make sure leader can spreading heatbeat message before other server ElectionTimeout
const HeartbeatTimeout = 50

// Make generating randomization thread safe
type SafeRand struct {
	mu   sync.Mutex // lock
	rand *rand.Rand
}

func (sr *SafeRand) Intn(n int) int {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.rand.Intn(n)
}

var SafeRandNum = &SafeRand{
	rand: rand.New(rand.NewSource(time.Now().UnixNano())),
}

func GeneratingElectionTimeout() time.Duration {
	return time.Duration(ElectionTimeout+SafeRandNum.Intn(ElectionTimeout)) * time.Millisecond
}

func GeneratingHearbeatMsgTimeout() time.Duration {
	return time.Duration(HeartbeatTimeout) * time.Millisecond
}

// quickSelect returns the k‑th smallest element (0‑based) in a.
func QuickSelect(array []int, k int) int {
	left, right := 0, len(array)-1
	for {
		if left == right {
			return array[left]
		}

		// choose middle element, move it to the end
		idx := (left + right) >> 1
		pivot := array[idx]
		array[idx], array[right] = array[right], array[idx]

		i := left
		for j := left; j < right; j++ {
			if array[j] < pivot {
				array[i], array[j] = array[j], array[i]
				i++
			}
		}
		array[i], array[right] = array[right], array[i] // pivot to its final spot

		switch {
		case k == i:
			return array[i]
		case k < i:
			right = i - 1
		default:
			left = i + 1
		}
	}
}
