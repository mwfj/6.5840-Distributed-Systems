package rsm

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"6.5840/kvsrv1/rpc"
	"6.5840/labgob"
	"6.5840/labrpc"
	raft "6.5840/raft1"
	"6.5840/raftapi"
	tester "6.5840/tester1"
)

var useRaftStateMachine bool // to plug in another raft besided raft1

/**
 * In each server instance node, it will run:
 * - A raft instance (consensus algorithm, for strong consistency)
 * - A RSM instance (appling the operatiion into state machine)
 * - A KVServer Instance (persistence)
 */

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Me  int   // server's request id
	Id  int64 // the unique id for each request
	Req any   // the request content
}

// A server (i.e., ../server.go) that wants to replicate itself calls
// MakeRSM and must implement the StateMachine interface.  This
// interface allows the rsm package to interact with the server for
// server-specific operations: the server must implement DoOp to
// execute an operation (e.g., a Get or Put request), and
// Snapshot/Restore to snapshot and restore the server's state.
type StateMachine interface {
	DoOp(any) any
	Snapshot() []byte
	Restore([]byte)
}

// PendingOp is to storing the corresponding operation information for the specific index
type PendingOp struct {
	op     Op
	result any
	done   chan bool
}

type RSM struct {
	mu           sync.Mutex
	me           int
	rf           raftapi.Raft
	applyCh      chan raftapi.ApplyMsg
	maxraftstate int // snapshot if log grows this big
	sm           StateMachine
	// Your definitions here.
	opId        int64              // the unique operation id
	lastApplied int                // last applied Raft log index
	pendingOps  map[int]*PendingOp // persist the operation that not finished
	shutdown    atomic.Bool
	shutdownCh  chan struct{}  // used to signal shutdown to all waiting operations
	activeOps   sync.WaitGroup // track active Submit operations
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
//
// me is the index of the current server in servers[].
//
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// The RSM should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
//
// MakeRSM() must return quickly, so it should start goroutines for
// any long-running work.
func MakeRSM(servers []*labrpc.ClientEnd, me int, persister *tester.Persister, maxraftstate int, sm StateMachine) *RSM {
	rsm := &RSM{
		me:           me,
		maxraftstate: maxraftstate,
		applyCh:      make(chan raftapi.ApplyMsg),
		sm:           sm,
		opId:         0,
		lastApplied:  0,
		pendingOps:   make(map[int]*PendingOp),
		shutdownCh:   make(chan struct{}),
	}
	rsm.shutdown.Store(false)
	if !useRaftStateMachine {
		rsm.rf = raft.Make(servers, me, persister, rsm.applyCh)
	}
	if maxraftstate >= 0 {
		rsm.mu.Lock()
		defer rsm.mu.Unlock()
		snapshot := persister.ReadSnapshot()
		if len(snapshot) > 0 {
			r := bytes.NewBuffer(snapshot)
			d := labgob.NewDecoder(r)
			var opId int64
			var lastApplied int
			var smSnapshot []byte
			if d.Decode(&opId) != nil || d.Decode(&lastApplied) != nil || d.Decode(&smSnapshot) != nil {
				panic("Failed to decode from snapshot")
			}
			atomic.StoreInt64(&rsm.opId, opId)
			// Set lastApplied from snapshot so we know not to re-apply those operations
			rsm.lastApplied = lastApplied
			rsm.sm.Restore(smSnapshot)
		}
	}
	go rsm.reader()
	return rsm
}

func (rsm *RSM) Raft() raftapi.Raft {
	return rsm.rf
}

// Submit a command to Raft, and wait for it to be committed.  It
// should return ErrWrongLeader if client should find new leader and
// try again.
func (rsm *RSM) Submit(req any) (rpc.Err, any) {

	// Submit creates an Op structure to run a command through Raft;
	// for example: op := Op{Me: rsm.me, Id: id, Req: req}, where req
	// is the argument to Submit and id is a unique id for the op.

	// Hard timeout for entire Submit operation during shutdown
	if rsm.shutdown.Load() {
		return rpc.ErrWrongLeader, nil
	}

	rsm.activeOps.Add(1)
	defer rsm.activeOps.Done()

	// Create a timeout context for the entire operation
	done := make(chan struct{})
	var err rpc.Err
	var result any

	go func() {
		defer close(done)
		err, result = rsm.doSubmit(req)
	}()

	// Wait for completion or timeout during shutdown
	timeout := 2 * time.Second // Normal timeout
	if rsm.shutdown.Load() {
		timeout = 50 * time.Millisecond // Short timeout during shutdown
	}

	select {
	case <-done:
		return err, result
	case <-time.After(timeout):
		return rpc.ErrWrongLeader, nil
	case <-rsm.shutdownCh:
		return rpc.ErrWrongLeader, nil
	}
}

// isShutdown checks if the RSM is shutting down
func (rsm *RSM) isShutdown() bool {
	select {
	case <-rsm.shutdownCh:
		return true
	default:
		return rsm.shutdown.Load()
	}
}

// notifyPendingOps notifies all pending operations with the given result
func (rsm *RSM) notifyPendingOps(result bool) {
	for _, pendingOp := range rsm.pendingOps {
		select {
		case pendingOp.done <- result:
		default:
		}
	}
}

// doShutdown performs common shutdown operations
func (rsm *RSM) doShutdown() {
	rsm.shutdown.Store(true)

	// Close shutdown channel to signal all waiting operations
	select {
	case <-rsm.shutdownCh:
		// already closed
	default:
		close(rsm.shutdownCh)
	}

	// Notify all pending operations
	rsm.notifyPendingOps(false)

	// Clear the pending operations map
	rsm.pendingOps = make(map[int]*PendingOp)
}

// doSubmit performs the actual submit logic
func (rsm *RSM) doSubmit(req any) (rpc.Err, any) {
	if rsm.isShutdown() {
		return rpc.ErrWrongLeader, nil
	}

	opId := atomic.AddInt64(&rsm.opId, 1)
	op := Op{
		Me:  rsm.me,
		Id:  opId,
		Req: req,
	}

	index, term, isLeader := rsm.rf.Start(op)
	if !isLeader {
		return rpc.ErrWrongLeader, nil
	}

	pendingOp := &PendingOp{
		op:   op,
		done: make(chan bool, 1),
	}

	rsm.mu.Lock()
	if rsm.isShutdown() {
		rsm.mu.Unlock()
		return rpc.ErrWrongLeader, nil
	}

	if existingOp, exists := rsm.pendingOps[index]; exists {
		select {
		case existingOp.done <- false:
		default:
		}
	}
	rsm.pendingOps[index] = pendingOp
	rsm.mu.Unlock()

	err, result := rsm.waitForResult(pendingOp, term)

	rsm.mu.Lock()
	delete(rsm.pendingOps, index)
	rsm.mu.Unlock()

	return err, result
}

// wait for the operation result
func (rsm *RSM) waitForResult(pendingOp *PendingOp, initialTerm int) (rpc.Err, any) {
	timeout := time.NewTimer(100 * time.Millisecond)
	defer timeout.Stop()

	// Hard timeout for shutdown scenarios only
	var shutdownDeadline time.Time
	shutdownTimeoutSet := false

	for {
		// Check shutdown first - highest priority
		if rsm.shutdown.Load() {
			// Set shutdown deadline only once when shutdown is first detected
			if !shutdownTimeoutSet {
				shutdownDeadline = time.Now().Add(50 * time.Millisecond)
				shutdownTimeoutSet = true
			}
			// If shutdown timeout reached, return immediately
			if time.Now().After(shutdownDeadline) {
				return rpc.ErrWrongLeader, nil
			}
			// Continue checking for quick response, but don't return immediately
		}

		select {
		case <-rsm.shutdownCh:
			return rpc.ErrWrongLeader, nil
		case res := <-pendingOp.done:
			if res {
				return rpc.OK, pendingOp.result
			} else {
				return rpc.ErrWrongLeader, nil
			}
		case <-timeout.C:
			if rsm.shutdown.Load() {
				return rpc.ErrWrongLeader, nil
			}
			currentTerm, isLeader := rsm.rf.GetState()
			if !isLeader || currentTerm != initialTerm {
				return rpc.ErrWrongLeader, nil
			}
			timeout.Reset(100 * time.Millisecond)
		}
	}
}

func (rsm *RSM) reader() {
	for {
		msg, ok := <-rsm.applyCh
		if !ok {
			rsm.handleShutdown()
			return
		}
		if rsm.shutdown.Load() {
			return
		}
		if msg.CommandValid {
			rsm.applyCommand(msg)
		} else if msg.SnapshotValid {
			rsm.applySnapshot(msg)
		}
	}
}

func (rsm *RSM) handleShutdown() {
	rsm.mu.Lock()
	defer rsm.mu.Unlock()
	rsm.doShutdown()
}

func (rsm *RSM) applyCommand(msg raftapi.ApplyMsg) {

	op, ok := msg.Command.(Op)
	if !ok {
		panic("Command is not of type Op")
	}
	rsm.mu.Lock()

	// Only apply if this is a new command (prevents re-applying after snapshot restore)
	if msg.CommandIndex <= rsm.lastApplied {
		rsm.mu.Unlock()
		return
	}

	result := rsm.sm.DoOp(op.Req)
	rsm.lastApplied = msg.CommandIndex

	if pendingOp, exists := rsm.pendingOps[msg.CommandIndex]; exists {
		if pendingOp.op.Id == op.Id && pendingOp.op.Me == rsm.me {
			pendingOp.result = result
			select {
			case pendingOp.done <- true:
			default:
			}
		} else {
			// means that's expired operation, may caused by leader change
			select {
			case pendingOp.done <- false:
			default:
			}
		}
	}
	shouldSnapshot := rsm.maxraftstate > 0 && rsm.rf.PersistBytes() > (rsm.maxraftstate*9)/10
	snapshotIndex := msg.CommandIndex

	rsm.mu.Unlock()

	// Create snapshot outside the lock to avoid blocking other operations
	if shouldSnapshot {
		rsm.createSnapshot(snapshotIndex)
	}

}

func (rsm *RSM) createSnapshot(index int) {
	rsm.mu.Lock()
	defer rsm.mu.Unlock()

	// Always use rsm.lastApplied as the snapshot index
	// This ensures the snapshot data matches the index we report to Raft
	// Even if more commands were applied after we decided to snapshot,
	// we create a snapshot for the current state
	snapshotIndex := rsm.lastApplied

	// Sanity check: snapshot index should be at least as high as requested
	if snapshotIndex < index {
		// This shouldn't happen, but if it does, use the requested index
		snapshotIndex = index
	}

	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	opId := atomic.LoadInt64(&rsm.opId)
	smSnapshot := rsm.sm.Snapshot()

	e.Encode(opId)
	e.Encode(snapshotIndex)
	e.Encode(smSnapshot)

	rsm.rf.Snapshot(snapshotIndex, w.Bytes())
}

func (rsm *RSM) applySnapshot(msg raftapi.ApplyMsg) {
	rsm.mu.Lock()
	defer rsm.mu.Unlock()

	// Only apply snapshot if it's newer than what we've already applied
	// This prevents rolling back to an older state
	if msg.SnapshotIndex <= rsm.lastApplied {
		return
	}

	r := bytes.NewBuffer(msg.Snapshot)
	d := labgob.NewDecoder(r)
	var opId int64
	var snapshotIndex int
	var smSnapshot []byte
	if d.Decode(&opId) != nil || d.Decode(&snapshotIndex) != nil || d.Decode(&smSnapshot) != nil {
		panic("Failed to decode from snapshot")
	}

	// Verify the snapshot index matches what Raft told us
	if snapshotIndex != msg.SnapshotIndex {
		panic(fmt.Sprintf("Snapshot index mismatch: encoded=%d, msg=%d", snapshotIndex, msg.SnapshotIndex))
	}

	// Restore the snapshot
	atomic.StoreInt64(&rsm.opId, opId)
	rsm.lastApplied = msg.SnapshotIndex
	rsm.sm.Restore(smSnapshot)

	// Notify pending operations that are covered by this snapshot
	for index, pendingOp := range rsm.pendingOps {
		if index <= msg.SnapshotIndex {
			select {
			case pendingOp.done <- false:
			default:
			}
		}
	}
}

// Kill shuts down the RSM instance
func (rsm *RSM) Kill() {
	rsm.mu.Lock()
	rsm.doShutdown()
	rsm.mu.Unlock()

	if rsm.rf != nil {
		rsm.rf.Kill()
	}
}
