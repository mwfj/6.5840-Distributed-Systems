# Replicated State Machine (RSM) - Knowledge Background

## Table of Contents
- [Academic Foundation](#academic-foundation)
- [Core Concepts](#core-concepts)
- [Architectural Role](#architectural-role)
- [Required Reading](#required-reading)
- [Lab Learning Objectives](#lab-learning-objectives)
- [Theoretical Properties](#theoretical-properties)
- [Design Principles](#design-principles)
- [References](#references)

---

## Academic Foundation

### Primary Theory Source

**Fred Schneider's Foundational Paper (1990)**
- "Implementing Fault-Tolerant Services Using the State Machine Approach: A Tutorial"
- Paper: https://pdos.csail.mit.edu/6.824/papers/schneider-rsm.pdf
- This is THE fundamental paper on State Machine Replication
- Originally described by Lamport (1978), elaborated by Schneider (1990)
- **Core idea**: Deterministic state machines executing the same sequence of commands produce identical outputs

### Course Context

This RSM implementation is part of **MIT 6.5840/6.824 (Distributed Systems)**:
- **Lab 4 Part A**: Fault-tolerant Key/Value Service
- Lab instructions: https://pdos.csail.mit.edu/6.824/labs/lab-kvraft1.html
- Builds on **Lab 3 (Raft)** consensus implementation
- See [course diagram](https://pdos.csail.mit.edu/6.824/figs/kvraft.pdf) showing Clerk → Service → RSM → Raft layers

---

## Core Concepts

### What is State Machine Replication?

**Definition**: A technique for implementing fault-tolerant services by replicating servers and coordinating client interactions with server replicas.

**Key Principle**:
- Multiple servers independently implement the same deterministic state machine
- All servers execute **the same sequence of commands** via consensus (Raft)
- Results in **identical states and outputs** across all replicas
- Provides **fault tolerance** - system continues despite server failures

### The RSM-Consensus Relationship

1. **Consensus** = Agreement on a single value by independent entities
2. **State Machine Replication** = Running consensus repeatedly to agree on a sequence of commands
3. **Consensus protocols (like Raft)** are used to implement SMR

**Key Insight**: Consensus and State Machine Replication are generally considered equivalent problems, though completing an SMR command can be more expensive than solving a single consensus instance.

### Deterministic State Machines

For SMR to work correctly:
- State machines must be **deterministic** (same input → same output)
- All replicas must start from the same initial state
- All replicas must execute commands in the **same order** (total ordering)
- Non-deterministic operations (random numbers, timestamps) must be handled specially

---

## Architectural Role

### Three-Layer Architecture

The RSM sits as a middleware layer between the service and consensus:

```
Client/Clerk
    ↓
    [RPC calls: Get, Put, Append]
    ↓
Service Layer (Key/Value Database)
    ↓ (implements StateMachine interface: DoOp, Snapshot, Restore)
    ↓
RSM Package ← Abstraction Layer
    ↓ (uses Raft for consensus)
    ↓
Raft Consensus Layer
    ↓ (maintains replicated log)
    ↓
Persistent Storage
```

### RSM Responsibilities

1. **Abstracts Raft complexity** from the service layer
   - Service doesn't need to understand Raft internals
   - Clean interface through `Submit()` and `StateMachine`

2. **Manages Submit/Apply interaction**:
   - Service calls `rsm.Submit()` with operations
   - RSM calls `raft.Start()` to propose to consensus
   - Reader goroutine receives committed operations from `applyCh`
   - Calls service's `DoOp()` to execute
   - Returns result to waiting `Submit()` call

3. **Handles edge cases**:
   - Wrong leader detection → return `ErrWrongLeader`
   - Leadership changes mid-operation → retry detection
   - Operation deduplication via unique IDs
   - Graceful shutdown coordination

4. **Snapshot coordination**:
   - Monitors Raft state size
   - Triggers snapshots at threshold (90% of maxraftstate)
   - Coordinates with service's snapshot/restore

---

## Required Reading

### Primary References

1. **[Extended Raft Paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf)**
   - Focus on Section 7 (Log Compaction)
   - Describes snapshot mechanism
   - InstallSnapshot RPC specification

2. **[Schneider's RSM Tutorial](https://pdos.csail.mit.edu/6.824/papers/schneider-rsm.pdf)**
   - Fundamental theory of state machine replication
   - Properties and correctness conditions
   - Implementation approaches

3. **[Bolosky et al.](http://static.usenix.org/event/nsdi11/tech/full_papers/Bolosky.pdf)**
   - Practical considerations for SMR systems
   - Performance optimizations
   - Real-world deployment lessons

### Related Systems (for broader perspective)

- **Chubby**: Google's lock service using Paxos
- **Paxos Made Live**: Google's experience with Paxos
- **Spanner**: Google's globally-distributed database
- **Zookeeper**: Apache's coordination service
- **Harp**: Replicated file system
- **Viewstamped Replication**: Early consensus protocol

---

## Lab Learning Objectives

### Part A Goals (RSM Implementation)

From Lab 4 Part A instructions:

1. **Implement reader goroutine**
   - Reads from Raft's `applyCh` channel
   - Processes committed commands
   - Handles snapshot messages
   - Runs continuously until shutdown

2. **Implement Submit() function**:
   - Wraps operations with unique IDs (`Op{Me, Id, Req}`)
   - Calls `raft.Start()` to propose operation
   - Waits for commit via pending operations map
   - Returns result or `ErrWrongLeader`
   - Handles timeouts appropriately

3. **Handle tricky scenarios**:
   - Leadership changes after `Start()` but before commit
   - Operation lost/never committed (term change detection)
   - Matching `applyCh` messages with waiting `Submit()` calls
   - Concurrent operations from multiple clients
   - Server crashes and restarts

### Expected Operation Flow

**Normal case (successful operation)**:

```
1. Client sends request → Service leader
2. Service → rsm.Submit(request)
3. rsm.Submit() → raft.Start(request) → WAIT
4. Raft → commits → sends to all peers' applyCh
5. RSM reader → reads applyCh → service.DoOp()
6. Leader's reader → hands result to Submit() → returns to client
```

**Error cases**:
- Not leader → return `ErrWrongLeader` immediately
- Lost leadership → detect via term change, return `ErrWrongLeader`
- Operation superseded → notify pending op, return error
- Timeout → periodically check leadership status

---

## Theoretical Properties

### Essential Properties for Correctness

1. **Determinism**
   - State machines must be deterministic (same input → same output)
   - All replicas produce identical results given same command sequence
   - Non-deterministic operations must be made deterministic (e.g., leader decides timestamp)

2. **Total Ordering**
   - All replicas execute commands in the same order
   - Provided by Raft's replicated log
   - Log index determines execution order

3. **Fault Tolerance**
   - System tolerates f failures with 2f+1 replicas
   - Raft requires majority (quorum) for progress
   - Example: 3 servers tolerate 1 failure, 5 servers tolerate 2 failures

4. **Linearizability**
   - Operations appear to execute atomically
   - Operations appear in real-time order
   - Once a client reads a value, it never reads an older value
   - Achieved by waiting for commit before returning

5. **At-Most-Once Semantics**
   - Each operation committed and executed exactly once
   - Duplicate detection via unique operation IDs
   - Critical for operations with side effects (Put, Append)

### Safety vs Liveness

**Safety** (must always hold):
- Never return incorrect results
- Never violate linearizability
- Never execute operation twice

**Liveness** (should eventually happen):
- Operations eventually complete (if majority alive)
- May block during network partitions
- May timeout and require retry

---

## Design Principles

### Applied in This Implementation

Based on `src/kvraft1/rsm/rsm.go`:

#### 1. Operation Deduplication (lines 26-33)
```go
type Op struct {
    Me  int   // server's request id
    Id  int64 // unique id for each request
    Req any   // request content
}
```
- Unique `{Me, Id}` tuple per operation
- Prevents duplicate execution
- Allows matching committed ops with pending submissions

#### 2. Pending Operations Tracking (lines 47-52, 63)
```go
type PendingOp struct {
    op     Op
    result any
    done   chan bool
}
pendingOps map[int]*PendingOp  // index → PendingOp
```
- Maps log index to waiting operation
- Enables notification when operation commits
- Supports concurrent submissions

#### 3. Linearizability via Wait (lines 125-163)
```go
func (rsm *RSM) Submit(req any) (rpc.Err, any) {
    // ... create op, call raft.Start() ...
    // Wait for commit before returning
}
```
- Client waits for commit
- Returns result only after execution
- Ensures linearizable semantics

#### 4. Snapshot Coordination (line 347-349)
```go
if rsm.maxraftstate > 0 && rsm.rf.PersistBytes() > (rsm.maxraftstate*9)/10 {
    go rsm.createSnapshot(msg.CommandIndex)
}
```
- Monitors Raft state size
- Triggers at 90% threshold (not 100% to avoid overshoot)
- Asynchronous snapshot creation

#### 5. Graceful Shutdown (lines 131-162, 186-202)
```go
shutdown   atomic.Bool
shutdownCh chan struct{}
activeOps  sync.WaitGroup
```
- Coordinates cleanup across goroutines
- Notifies pending operations
- Prevents deadlocks during shutdown

#### 6. Wrong Leader Detection (lines 252-294)
```go
func (rsm *RSM) waitForResult(pendingOp *PendingOp, initialTerm int) {
    // Periodically check term/leadership
    // Return ErrWrongLeader if changed
}
```
- Periodic leadership checks
- Term change detection
- Timeout mechanism

---

## Implementation Challenges

### Common Pitfalls

1. **Matching operations to results**
   - Challenge: Multiple concurrent operations, log indices may be reused
   - Solution: Use unique operation IDs, check term matches

2. **Leadership changes**
   - Challenge: Operation proposed but leadership lost before commit
   - Solution: Detect term changes, notify pending operations to retry

3. **Duplicate execution**
   - Challenge: Same operation might appear multiple times
   - Solution: Track executed operation IDs, skip duplicates

4. **Snapshot timing**
   - Challenge: When to snapshot? Too early wastes space, too late risks overflow
   - Solution: Trigger at 90% threshold asynchronously

5. **Shutdown coordination**
   - Challenge: Multiple goroutines need to terminate cleanly
   - Solution: Shutdown channel, atomic flags, WaitGroups

6. **Stale reads**
   - Challenge: Followers might have outdated state
   - Solution: All operations go through leader via Raft

---

## Testing Strategy

### Test Coverage (from `rsm_test.go`)

1. **TestBasic4A**: Basic functionality
   - Sequential operations
   - State verification across replicas

2. **TestConcurrent4A**: Concurrent submissions
   - Many concurrent operations
   - Correctness under load

3. **TestLeaderFailure4A**: Leader failure handling
   - Disconnect leader
   - New leader election
   - Old leader catch-up

4. **TestLeaderPartition4A**: Minority can't commit
   - Network partitions
   - Majority/minority behavior
   - Partition healing

5. **TestRestartReplay4A**: Persistence & replay
   - Full system shutdown/restart
   - State preservation via persistence

6. **TestShutdown4A**: Graceful shutdown
   - Pending operations during shutdown
   - Clean termination without deadlocks

7. **TestRestartSubmit4A**: Complex restart scenarios
   - Multiple shutdown/restart cycles
   - Pending ops across restarts

8. **TestSnapshot4C**: Snapshotting
   - Log compaction verification
   - Snapshot restore correctness
   - Size limits enforced

---

## References

### Academic Papers
- [State machine replication - Wikipedia](https://en.wikipedia.org/wiki/State_machine_replication)
- [Schneider RSM Paper](https://pdos.csail.mit.edu/6.824/papers/schneider-rsm.pdf)
- [Extended Raft Paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf)
- [Consensus for State Machine Replication](https://decentralizedthoughts.github.io/2019-10-15-consensus-for-state-machine-replication/)

### Course Materials
- [MIT 6.5840/6.824 Course](https://pdos.csail.mit.edu/6.824/)
- [Lab 4: Fault-tolerant Key/Value Service](https://pdos.csail.mit.edu/6.824/labs/lab-kvraft1.html)
- [KVRaft Diagram](https://pdos.csail.mit.edu/6.824/figs/kvraft.pdf)
- [6.824 Lab 7: Replicated State Machine](https://pdos.csail.mit.edu/archive/6.824-2012/labs/lab-7.html)

### Additional Resources
- [Lab 3: Raft Implementation](http://nil.csail.mit.edu/6.824/2022/labs/lab-kvraft.html)
- [Tutorial on SMR](https://www.di.fc.ul.pt/~bessani/publications/tutorial-smr.pdf)

---

## Summary

**Replicated State Machine (RSM)** is a fundamental distributed systems pattern that provides:
- **Fault tolerance** through replication
- **Strong consistency** via consensus (Raft)
- **Linearizability** for client operations
- **Clean abstraction** between service logic and consensus

The implementation in `src/kvraft1/rsm/` demonstrates:
- Proper interaction with Raft consensus
- Handling of concurrent operations
- Snapshot coordination for log compaction
- Graceful error handling and recovery

This forms the foundation for building fault-tolerant distributed services like replicated key-value stores, configuration services, and distributed databases.
