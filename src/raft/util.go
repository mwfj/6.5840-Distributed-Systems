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
