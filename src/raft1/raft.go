package raft

// The file raftapi/raft.go defines the interface that raft must
// expose to servers (or the tester), but see comments below for each
// of these functions for more details.
//
// Make() creates a new raft peer that implements the raft interface.

import (
	"bytes"
	// "math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raftapi"
	tester "6.5840/tester1"
)

type ServerState int

// ServerState states enmu
const (
	Follower ServerState = iota
	Candidate
	Leader
)

// 3B define the log struct
type LogEntry struct {
	Command interface{}
	Term    int // the corresponding term
	Index   int // the position of the server's log
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.RWMutex        // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *tester.Persister   // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (3A, 3B, 3C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm int // latest term server has seen
	voteFor     int // candidatedId that received vote in current state
	commitIndex int // index of hightest log entry know to be committed
	lastApplied int // index of hightest log entry appiled to state machine

	state ServerState // the current server state in the election

	nextIndex  []int // index of the next log entry to send to that server
	matchIndex []int // index of highest log entry applied to state machine

	electionTimer  *time.Timer // the timer for election timeout, for follower
	heartbeatTimer *time.Timer // the timer for heartbeat timeout, for leader

	applyCh chan raftapi.ApplyMsg // channel to send

	// 3B new add
	logs          []LogEntry
	applyCond     *sync.Cond   // the condition variable for applyCh
	replicateCond []*sync.Cond // the condition variable for notifing all of peers to send log

	// 3D Snapshot include
	lastIncludedIndex int
	lastIncludedTerm  int

	// A pending snapshot that must be delivered on applyCh by the applier
	// goroutine. This avoids blocking RPC handlers on applyCh and ensures
	// applyCh delivery ordering is consistent.
	pendingSnapshot *raftapi.ApplyMsg
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	// Your code here (3A).
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.currentTerm, rf.state == Leader
}

func (rf *Raft) ChangeState(newstate ServerState) {
	// do nothing when the state is not changed
	if rf.state == newstate {
		return
	}
	DPrintf("{Node %v} changes state from %v to %v", rf.me, rf.state, newstate)
	rf.state = newstate
	switch newstate {
	case Follower:
		// follower election timout only
		rf.resetElectionTimer()
		rf.heartbeatTimer.Stop()
	case Leader:
		// leader use the heartbeat timeout only
		rf.electionTimer.Stop()
		rf.heartbeatTimer.Reset(GeneratingHearbeatMsgTimeout())

		last := rf.getLatestLog().Index

		for i := range rf.peers {
			rf.nextIndex[i] = last + 1 // what the Raft paper says
			rf.matchIndex[i] = 0       // unknown for followers
		}
		rf.updateLogIndices(rf.me, last) // but the leader has every entry itself
	// do nothing for candidate
	default:
	}
}

type InstallSnapShotArgs struct {
	Term              int
	LeaderId          int
	LastIncludedIndex int
	LastIncludedTerm  int
	Data              []byte // Raw bytes of the snapshot chunk
}

type InstallSnapShotReply struct {
	// currentTerm, for leader to update itself
	Term int
}

func (rf *Raft) makeInstallSnapShotArgs() *InstallSnapShotArgs {
	firstLog := rf.getFirstLog()

	args := &InstallSnapShotArgs{
		Term:              rf.currentTerm,
		LeaderId:          rf.me,
		LastIncludedIndex: firstLog.Index,
		LastIncludedTerm:  firstLog.Term,
		Data:              rf.persister.ReadSnapshot(),
	}
	return args
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist(snapshot []byte) {
	// Your code here (3C).
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.voteFor)
	e.Encode(rf.logs)
	e.Encode(rf.lastIncludedIndex)
	e.Encode(rf.lastIncludedTerm)
	raftstate := w.Bytes()

	if snapshot != nil {
		rf.persister.Save(raftstate, snapshot)
	} else {
		rf.persister.Save(raftstate, rf.persister.ReadSnapshot())
	}
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (3C).
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)

	var currentTerm, voteFor int
	var logs []LogEntry
	var lastIncludedIndex, lastIncludedTerm int

	if d.Decode(&currentTerm) != nil || d.Decode(&voteFor) != nil ||
		d.Decode(&logs) != nil || d.Decode(&lastIncludedIndex) != nil || d.Decode(&lastIncludedTerm) != nil {
		DPrintf("[Node %v] fail to decode the persistent state, original data %v", rf.me, data)
		return
	}

	rf.currentTerm, rf.voteFor, rf.logs = currentTerm, voteFor, logs
	rf.lastApplied, rf.commitIndex = rf.getFirstLog().Index, rf.getFirstLog().Index
	rf.lastIncludedIndex, rf.lastIncludedTerm = lastIncludedIndex, lastIncludedTerm
}

// replace slice by creating new undereline array
// inorder to prevent the capacity of the original slice growing too large,
// risking to cause OOM
func truncateLogs(entries []LogEntry) []LogEntry {
	const lenMultiple = 2
	if cap(entries) > len(entries)*lenMultiple {
		newEntries := make([]LogEntry, len(entries))
		copy(newEntries, entries)
		return newEntries
	}
	return entries
}

func generateSingleLog(lastIncludedIndex, lastIncludedTerm int) []LogEntry {
	return []LogEntry{{
		Term:  lastIncludedTerm,
		Index: lastIncludedIndex}}
}

// how many bytes in Raft's persisted log?
func (rf *Raft) PersistBytes() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.persister.RaftStateSize()
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (3D).
	// write lock needed
	rf.mu.Lock()
	defer rf.mu.Unlock()

	firstLogIdx := rf.getFirstLog().Index

	// check the idx validation
	if index <= firstLogIdx || index > rf.getLatestLog().Index {
		DPrintf("[Node %v] applied snapshot failed, reason: invalid index %v", rf.me, index)
		return
	}

	// compact the log
	newBeginEntry := index - firstLogIdx
	rf.lastIncludedIndex = rf.logs[newBeginEntry].Index
	rf.lastIncludedTerm = rf.logs[newBeginEntry].Term

	rf.logs = truncateLogs(rf.logs[newBeginEntry:])
	rf.logs[0].Command = nil
	rf.persist(snapshot)
}

func (rf *Raft) InstallSnapShot(args *InstallSnapShotArgs, reply *InstallSnapShotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm

	// ignore stale snapshot
	if args.Term < rf.currentTerm {
		DPrintf("[Node %v] snapshot term is stale, ignored. snapshot term %v, node current term %v",
			rf.me, args.Term, rf.currentTerm)
		return
	}

	if args.Term > rf.currentTerm {
		rf.currentTerm, rf.voteFor = args.Term, -1
	}

	rf.ChangeState(Follower)
	rf.resetElectionTimer()

	if args.LastIncludedIndex <= rf.lastApplied {
		DPrintf("[Node %v] snapshot index is stale, ignored. snapshot index %v, last applied index %v",
			rf.me, args.LastIncludedIndex, rf.lastApplied)
		return
	}

	firstLogIndex := rf.getFirstLog().Index

	// update the local state
	rf.lastIncludedIndex = args.LastIncludedIndex
	rf.lastIncludedTerm = args.LastIncludedTerm

	// if snapshot contains entries beyond our log,
	// or if there's a conflict, discard conflicting entries
	if args.LastIncludedIndex >= rf.getLatestLog().Index {
		// snapshot contains more entries than our log, replace entire log
		rf.logs = []LogEntry{{
			Term:  args.LastIncludedTerm,
			Index: args.LastIncludedIndex,
		}}
	} else if args.LastIncludedIndex >= firstLogIndex {
		// check if we have a conflicting entry at the snapshot index
		logIndex := args.LastIncludedIndex - firstLogIndex
		if logIndex < len(rf.logs) && rf.logs[logIndex].Term != args.LastIncludedTerm {
			// conflict detected: discard conflicting and subsequent entries
			rf.logs = generateSingleLog(args.LastIncludedIndex, args.LastIncludedTerm)
		} else {
			// keep entries after snapshot
			offset := args.LastIncludedIndex - firstLogIndex + 1
			if offset < len(rf.logs) {
				newLogs := generateSingleLog(args.LastIncludedIndex, args.LastIncludedTerm)
				newLogs = append(newLogs, rf.logs[offset:]...)
				rf.logs = truncateLogs(newLogs)
			} else {
				rf.logs = generateSingleLog(args.LastIncludedIndex, args.LastIncludedTerm)
			}
		}
	}

	// update commit and applied indices
	if args.LastIncludedIndex > rf.commitIndex {
		rf.commitIndex = args.LastIncludedIndex
	}

	// must add, the order will mess otherwise
	if args.LastIncludedIndex > rf.lastApplied {
		rf.lastApplied = args.LastIncludedIndex
	}

	// these must be persisted before acknowledging the InstallSnapshot RPC to ensure crash safety.
	// sync snapshot later through appy channel asynchronized
	rf.persist(args.Data)

	// Notify the service layer via the applier goroutine (never block while holding rf.mu).
	// This is to avoid block for sending message from apply cauing whole raft hanging
	snapshotCopy := make([]byte, len(args.Data))
	copy(snapshotCopy, args.Data)
	rf.pendingSnapshot = &raftapi.ApplyMsg{
		SnapshotValid: true,
		Snapshot:      snapshotCopy,
		SnapshotTerm:  args.LastIncludedTerm,
		SnapshotIndex: args.LastIncludedIndex,
	}
	rf.applyCond.Signal()
}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (3A, 3B).
	// 3A
	Term        int // candidate's term
	CandidateId int // candidate requesting vote
	// 3B
	LastLogIndex int // index of candidate's last log entry
	LastLogTerm  int // term of candidate's last log entry
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (3A).
	Term        int  // candidate's term
	VoteGranted bool // true means candidate received vote
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// §5.1 reply false if term < current Term
	// §5.2 If votedFor is null or not the candidateId,
	///     then reject the vote
	if (args.Term < rf.currentTerm) ||
		(args.Term == rf.currentTerm && rf.voteFor != -1 && rf.voteFor != args.CandidateId) {
		reply.Term, reply.VoteGranted = rf.rejectVote()
		return
	}

	// if the current term that this server hold is not the latest
	// revert itself to follower
	rf.updateTermAndStepDown(args.Term)

	// 3B
	// §5.4: If candidate's log is not up-to-date, reject the vote request
	if !rf.isLogUpdateToDate(args.LastLogIndex, args.LastLogTerm) {
		reply.Term, reply.VoteGranted = rf.rejectVote()
		return
	}

	// vote for the first election request that has potential become leader
	// first come first serve
	rf.voteFor = args.CandidateId
	// 3C
	rf.persistState()
	rf.resetElectionTimer()

	reply.Term, reply.VoteGranted = rf.currentTerm, true
}

type AppendEntryArgs struct {
	// Your data here (3A).
	// 3A
	Term         int // leader's term
	LeaderId     int // follower can redirect clients
	PrevLogIndex int // index of log entry immediately procedingi new ones

	PrevLogTerm int        // term of prevLogIndex Entry
	Entries     []LogEntry // log entry to store

	LeaderCommit int // leader's commitIndex

}

type AppendEntryReply struct {
	// Your data here (3A).
	// current term, for leader to update itself true
	// if follower contained entry matching prevLogIndex and prevLogTerm
	Term int
	// true if follower contained entry matching prevLogIndex and prevLogTerm
	Success bool // candidate requesting vote

	ConflictIndex int
	ConflictTerm  int
}

func (rf *Raft) genAppendEntryArgs(prevLogIndex int) *AppendEntryArgs {
	firstLogIndex := rf.getFirstLog().Index
	lastLogIndex := rf.getLatestLog().Index

	// deal with the corner case
	// prevent follower get something like rf.log[-1]
	if prevLogIndex < (firstLogIndex - 1) {
		prevLogIndex = firstLogIndex - 1
	}
	if prevLogIndex > lastLogIndex {
		prevLogIndex = lastLogIndex
	}

	// boundary check before accessing rf.logs for prevLogTerm
	var prevLogTerm int
	if prevLogIndex >= firstLogIndex && (prevLogIndex-firstLogIndex) < len(rf.logs) {
		prevLogTerm = rf.logs[prevLogIndex-firstLogIndex].Term
	} else {
		// this should not happened, but do the last check
		prevLogTerm = 0
	}

	entries := make([]LogEntry, len(rf.logs[prevLogIndex-firstLogIndex+1:]))
	copy(entries, rf.logs[prevLogIndex-firstLogIndex+1:])
	args := &AppendEntryArgs{
		Term:         rf.currentTerm,
		LeaderId:     rf.me,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: rf.commitIndex,
	}

	return args
}

// Initialed by leaders to replicate log entries and to provide a form of heartbeat,
// where heartbeat message is an AppendEntry that carry no log entry
func (rf *Raft) AppendEntry(args *AppendEntryArgs, reply *AppendEntryReply) {
	// 3A
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer DPrintf("[Node %v] is appending entry, role %v, args %v, reply %v", rf.me, rf.state, args, reply)

	// §5.1 return false if term < currentTerm
	// which is a stale term number
	if args.Term < rf.currentTerm {
		reply.Term, reply.Success = rf.rejectAppendEntry()
		return
	}

	// find out the current term is not the latest, then update it
	rf.updateTermAndStepDown(args.Term)

	// change state to follower
	if rf.state != Follower {
		rf.ChangeState(Follower)
	}
	// here make the test case happy, reset election timer again
	// otherwise, warning throwed when test case check the term
	rf.resetElectionTimer()

	// 3B
	// §5.3: reply false if log doesn't contain an entry at prevLogIndex
	//       whose term matches preLogTerm
	if args.PrevLogIndex < rf.getFirstLog().Index {
		reply.Term, reply.Success = rf.rejectAppendEntry()
		return
	}

	firstLogIdx := rf.getFirstLog().Index

	// §5.3: If an existing entry conflict with a new one
	//       (same index but different terms),
	//       delete the existing entry and all that follow it
	if !rf.isLogMatched(args.PrevLogIndex, args.PrevLogTerm) {
		firstLogIdx := rf.getFirstLog().Index
		lastLogIdx := rf.getLatestLog().Index

		reply.Term, reply.Success = rf.rejectAppendEntry()
		// the current server doesn't have that high index, return its the next index
		if lastLogIdx < args.PrevLogIndex {
			reply.ConflictIndex, reply.ConflictTerm = lastLogIdx+1, -1
		} else {
			// boundary check here before accessing rf.logs
			if !rf.isValidLogIndex(args.PrevLogIndex) {
				DPrintf("Node[%v] invalid index, treat as missing entry, index %v", rf.me, args.PrevLogIndex)
				reply.ConflictIndex, reply.ConflictTerm = firstLogIdx, -1
				return
			}

			conflictTerm := rf.logs[args.PrevLogIndex-firstLogIdx].Term
			reply.ConflictTerm = conflictTerm
			// find the first conflict term in follower's log
			offset := sort.Search(args.PrevLogIndex-firstLogIdx+1, func(index int) bool {
				if index >= len(rf.logs) {
					// out of bound
					return true
				}
				return rf.logs[index].Term >= conflictTerm
			})
			reply.ConflictIndex = firstLogIdx + offset
		}

		return
	}

	// append any new entries that not already in the log
	for index, entry := range args.Entries {
		if (entry.Index-firstLogIdx) >= len(rf.logs) || (rf.logs[entry.Index-firstLogIdx].Term != entry.Term) {
			rf.logs = append(rf.logs[:entry.Index-firstLogIdx], args.Entries[index:]...)
			// 3C
			rf.persistState()
			break
		}
	}
	// note that: go support built-in min function since go 1.21
	newCommitIdx := min(args.LeaderCommit, rf.getLatestLog().Index)
	if newCommitIdx > rf.commitIndex {
		// new commit idx generated, need to notify all of its peers
		DPrintf("[Node %v] generate new index term %v, old index %v. current term %v",
			rf.me, newCommitIdx, rf.matchIndex, rf.currentTerm)
		rf.commitIndex = newCommitIdx
		rf.applyCond.Signal()
	}
	reply.Term, reply.Success = rf.currentTerm, true

}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) sendAppendEntry(server int, args *AppendEntryArgs, reply *AppendEntryReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntry", args, reply)
	return ok
}

func (rf *Raft) sendInstallSnapShot(server int, args *InstallSnapShotArgs, reply *InstallSnapShotReply) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapShot", args, reply)
	return ok
}

func (rf *Raft) LaunchElection() {
	// vote for himself first
	rf.voteFor = rf.me
	grantedVotes := 1
	// 3C
	rf.persistState()

	voteArgs := &RequestVoteArgs{
		Term:         rf.currentTerm,
		CandidateId:  rf.me,
		LastLogIndex: rf.getLatestLog().Index,
		LastLogTerm:  rf.getLatestLog().Term,
	}

	DPrintf("[Node %v] launch a new leader election with requestvote args %v", rf.me, voteArgs)

	// send the launch election to all the rest of peers
	for peer := range rf.peers {

		if peer == rf.me {
			continue
		}

		go func(peer int) {
			voteReply := &RequestVoteReply{}

			if rf.sendRequestVote(peer, voteArgs, voteReply) {
				rf.mu.Lock()
				defer rf.mu.Unlock()
				DPrintf("[Node %v] after got reply for sendRequestVote from node %v, vote reply %v", rf.me, peer, voteReply)
				if rf.isValidCandidate(voteArgs.Term) {
					// received vote from other server
					if voteReply.VoteGranted {
						grantedVotes++
						// claim as leader if this server received vote from most of peers
						if grantedVotes > len(rf.peers)/2 {
							DPrintf("[Node %v] claim as leader with the number of votes %v", rf.me, grantedVotes)
							rf.ChangeState(Leader)
							// send heart beat message
							rf.BroadcastAppendEntryMsg(true)
						}
					} else if voteReply.Term > rf.currentTerm {
						// rollback to follower otherwise
						rf.updateTermAndStepDown(voteReply.Term)
					}
				}
			}
		}(peer)
	}
}

/**
 * for the heartbeat message, we will send the AppendEntry RPC immdiatly
 * Otherwise, the RPC will be sent only when new term added
 */
func (rf *Raft) BroadcastAppendEntryMsg(isHeartBeatMsg bool) {
	for peer := range rf.peers {
		if peer == rf.me {
			continue
		}
		// send heartbeat message
		// to the rest of peers
		if isHeartBeatMsg {
			go rf.doSendAppendEntry(peer)
		} else {
			// new log entry is available to do the replication
			rf.replicateCond[peer].Signal()
		}
	}
}

func (rf *Raft) acceptNewLogEntries(peer int, args *AppendEntryArgs) {
	matchIdx := args.PrevLogIndex + len(args.Entries)
	rf.updateLogIndices(peer, matchIdx)

	matchLen := len(rf.matchIndex)
	matchIndexDup := make([]int, matchLen)
	copy(matchIndexDup, rf.matchIndex)

	k := matchLen - (matchLen/2 + 1)
	newCommitIdx := QuickSelect(matchIndexDup, k)

	if newCommitIdx > rf.commitIndex {
		if rf.isLogMatched(newCommitIdx, rf.currentTerm) {
			// new commit idx generated, need to notify all of its peers
			DPrintf("[Leader %v] generate new index term %v, old index %v. current term %v",
				rf.me, newCommitIdx, rf.matchIndex, rf.currentTerm)
			rf.commitIndex = newCommitIdx
			rf.applyCond.Signal()
		}
	}
}

// §5.3 / extended Fig. 7.
func (rf *Raft) rollBackToConflictTerm(peer int, args *AppendEntryArgs, reply *AppendEntryReply) {
	// rollback the term to the committed term that match with peer
	rf.nextIndex[peer] = reply.ConflictIndex
	firstLogIdx := rf.getFirstLog().Index
	if reply.ConflictTerm != -1 {
		first := rf.getFirstLog().Index
		size := args.PrevLogIndex - first // inclusive length
		offset := sort.Search(size, func(i int) bool {
			return rf.logs[i].Term >= reply.ConflictTerm
		})

		if idx := first + offset; idx <= args.PrevLogIndex &&
			rf.logs[idx-first].Term == reply.ConflictTerm {
			rf.nextIndex[peer] = idx + 1 // §5.3 rule
		} else {
			rf.nextIndex[peer] = reply.ConflictIndex
		}
	}

	lastLogIdx := rf.getLatestLog().Index + 1
	if rf.nextIndex[peer] < firstLogIdx {
		rf.nextIndex[peer] = firstLogIdx
	}
	if rf.nextIndex[peer] > lastLogIdx {
		rf.nextIndex[peer] = lastLogIdx
	}

}

func (rf *Raft) doSendAppendEntry(peer int) {
	rf.mu.RLock()
	if rf.state != Leader {
		rf.mu.RUnlock()
		return
	}

	prevLogIdx := rf.nextIndex[peer] - 1

	// send leader's snapshot to follower,
	// if its log is behind
	if prevLogIdx < rf.getFirstLog().Index {
		args := rf.makeInstallSnapShotArgs()

		rf.mu.RUnlock()
		reply := &InstallSnapShotReply{}
		if rf.sendInstallSnapShot(peer, args, reply) {
			rf.mu.Lock()
			if rf.state == Leader && rf.currentTerm == args.Term {
				if reply.Term > rf.currentTerm {
					rf.updateTermAndStepDown(reply.Term)
				} else {
					rf.updateLogIndices(peer, args.LastIncludedIndex)
				}
			}
			rf.mu.Unlock()
		}
	} else {
		args := rf.genAppendEntryArgs(prevLogIdx)

		rf.mu.RUnlock()
		reply := &AppendEntryReply{}

		if !rf.sendAppendEntry(peer, args, reply) {
			DPrintf("[Node %v] sendAppendEntry failed. peer %v, args %v, reply %v", rf.me, peer, args, reply)
			return
		}
		rf.mu.Lock()
		defer rf.mu.Unlock()
		if !rf.isValidLeader(args.Term) {
			return
		}
		// make sure that the leader's term is the latest
		if rf.isValidLeader(args.Term) {
			// handle failure
			if !reply.Success {
				if reply.Term > rf.currentTerm {
					// discovered higher term → step down
					rf.updateTermAndStepDown(reply.Term)
					return
				}

				rf.rollBackToConflictTerm(peer, args, reply)
				// signal the per‑peer replicator to try again promptly
				rf.replicateCond[peer].Signal()
			} else {
				// follower accepted: advance matchIndex/nextIndex
				rf.acceptNewLogEntries(peer, args)
			}
		} else {
			DPrintf("[Node %v] not the latest term, ignore. current term %v, current state %v, args term %v",
				rf.me, rf.currentTerm, rf.state, args.Term)
		}
	}

}

// ---------------------------------------------------------------------------------------------------------------------

func (rf *Raft) getLatestLog() LogEntry {
	return rf.logs[len(rf.logs)-1]
}

func (rf *Raft) getFirstLog() LogEntry {
	return rf.logs[0]
}

func (rf *Raft) isLogUpdateToDate(index, term int) bool {
	lastLog := rf.getLatestLog()
	return (term > lastLog.Term) || (term == lastLog.Term && index >= lastLog.Index)
}

// boundary check
// Helper methods for code optimization

func (rf *Raft) isValidLeader(term int) bool {
	return rf.state == Leader && rf.currentTerm == term
}

func (rf *Raft) isValidCandidate(term int) bool {
	return rf.state == Candidate && rf.currentTerm == term
}

func (rf *Raft) updateTermAndStepDown(newTerm int) {
	if newTerm > rf.currentTerm {
		rf.currentTerm, rf.voteFor = newTerm, -1
		rf.ChangeState(Follower)
		rf.persistState()
	}
}

func (rf *Raft) isValidLogIndex(index int) bool {
	firstLogIdx := rf.getFirstLog().Index
	logArrayIndex := index - firstLogIdx
	return logArrayIndex >= 0 && logArrayIndex < len(rf.logs)
}

func (rf *Raft) updateLogIndices(peer, matchIdx int) {
	rf.matchIndex[peer] = matchIdx
	rf.nextIndex[peer] = matchIdx + 1
}

func (rf *Raft) resetElectionTimer() {
	rf.electionTimer.Reset(GeneratingElectionTimeout())
}

// 3C
func (rf *Raft) persistState() {
	rf.persist(nil)
}

func (rf *Raft) rejectVote() (int, bool) {
	return rf.currentTerm, false
}

func (rf *Raft) rejectAppendEntry() (int, bool) {
	return rf.currentTerm, false
}

func (rf *Raft) isLogMatched(index, term int) bool {
	if index < rf.getFirstLog().Index || index > rf.getLatestLog().Index {
		return false
	}
	if !rf.isValidLogIndex(index) {
		return false
	}
	logArrayIndex := index - rf.getFirstLog().Index
	return term == rf.logs[logArrayIndex].Term
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	// Your code here (3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state != Leader {
		return -1, -1, false
	}

	// Leader need to append the newest log to their local first
	newLatestLogIdx := rf.getLatestLog().Index + 1

	newLogEntry := LogEntry{
		Term:    rf.currentTerm,
		Index:   newLatestLogIdx,
		Command: command,
	}

	rf.logs = append(rf.logs, newLogEntry)
	// 3C
	rf.persistState()
	// update log index info
	rf.updateLogIndices(rf.me, newLatestLogIdx)

	DPrintf("[Node %v] start to append a new log entry, log entry %v", rf.me, newLogEntry)

	// notifing the newest log to its followers
	rf.BroadcastAppendEntryMsg(false)

	return newLatestLogIdx, rf.currentTerm, true
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Wake any goroutines blocked on condition variables so they can exit.
	rf.mu.Lock()
	if rf.applyCond != nil {
		rf.applyCond.Broadcast()
	}
	rf.mu.Unlock()
	for peer := range rf.replicateCond {
		if rf.replicateCond[peer] == nil {
			continue
		}
		rf.replicateCond[peer].Broadcast()
	}
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (3A)
		// Check if a leader election should be started.
		// Check if a leader election should be started.
		select {
		// received election timeout(follower)
		case <-rf.electionTimer.C:
			rf.mu.Lock()
			rf.ChangeState(Candidate)
			rf.currentTerm++
			rf.persistState()
			// lauch a new leader election
			rf.LaunchElection()
			rf.resetElectionTimer()
			rf.mu.Unlock()
		// received heartbeat timeout(leader)
		case <-rf.heartbeatTimer.C:
			rf.mu.Lock()
			if rf.state == Leader {
				// send the hearbeat message periodically
				rf.BroadcastAppendEntryMsg(true)
				rf.heartbeatTimer.Reset(GeneratingHearbeatMsgTimeout())
			}
			rf.mu.Unlock()
		}
	}
}

// 3B using applier to save new log created entry into local
// This is the ONLY goroutine that sends to applyCh, ensuring strict ordering
// of all messages (both log entries and snapshots) delivered to the application.
func (rf *Raft) applier() {
	for rf.killed() == false {
		rf.mu.Lock()

		// Wait for work: either new committed entries or a pending snapshot.
		// We wake up when:
		// 1. commitIndex > lastApplied (new committed entries to apply)
		// 2. pendingSnapshot != nil (snapshot received via InstallSnapShot RPC)
		// 3. killed() becomes true (shutting down)
		for rf.commitIndex <= rf.lastApplied && rf.pendingSnapshot == nil && !rf.killed() {
			rf.applyCond.Wait()
		}

		// Check if we're shutting down
		if rf.killed() {
			rf.mu.Unlock()
			return
		}

		// Priority 1: Handle pending snapshot FIRST before applying log entries.
		// This ensures snapshots are delivered in the correct order relative to log entries.
		// The snapshot was staged by InstallSnapShot RPC handler to avoid blocking while holding rf.mu.
		if rf.pendingSnapshot != nil {
			// Make a copy of the snapshot message
			msg := *rf.pendingSnapshot
			// Clear the pending snapshot so we don't send it again
			rf.pendingSnapshot = nil
			// Release lock BEFORE sending to applyCh to avoid blocking while holding lock
			rf.mu.Unlock()
			// Send snapshot to application (may block, but that's okay since we're not holding lock)
			rf.applyCh <- msg
			// Go back to wait for next work
			continue
		}

		// Priority 2: Apply committed log entries to the state machine.
		firstLogIdx := rf.getFirstLog().Index

		// Check if a snapshot has compacted our log while we were waiting.
		// If lastApplied is behind the first log entry, fast-forward to the snapshot point.
		if rf.lastApplied < firstLogIdx {
			// A snapshot has compacted the log; fast-forward to the snapshot.
			rf.lastApplied = firstLogIdx
			rf.mu.Unlock()
			continue
		}

		// Calculate the next entry to apply
		nextIndex := rf.lastApplied + 1

		// Double-check there's actually work to do
		if nextIndex > rf.commitIndex {
			rf.mu.Unlock()
			continue
		}

		// Convert log index to array index (accounting for log compaction)
		logArrayIndex := nextIndex - firstLogIdx

		// Defensive check: ensure the index is valid.
		// This can happen if a concurrent snapshot compacted the log between our checks.
		if logArrayIndex < 0 || logArrayIndex >= len(rf.logs) {
			// Defensive: if a concurrent snapshot compacted the log, retry.
			rf.lastApplied = firstLogIdx
			rf.mu.Unlock()
			continue
		}

		// Get the log entry to apply
		entry := rf.logs[logArrayIndex]
		// Update lastApplied BEFORE releasing lock to maintain invariant
		rf.lastApplied = nextIndex
		// Release lock BEFORE sending to applyCh to avoid blocking while holding lock
		rf.mu.Unlock()

		// Send the committed log entry to application (may block, but okay without lock)
		rf.applyCh <- raftapi.ApplyMsg{
			CommandValid: true,
			Command:      entry.Command,
			CommandIndex: entry.Index,
		}
	}
}

func (rf *Raft) shouleReplicateLog(peer int) bool {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	// check whether peer's latest log index is behind with leaders'
	return (rf.state == Leader) && (rf.matchIndex[peer] < rf.getLatestLog().Index)
}

// 3B
// send log to other peers
func (rf *Raft) replicator(peer int) {
	rf.replicateCond[peer].L.Lock()
	defer rf.replicateCond[peer].L.Unlock()

	for rf.killed() == false {
		for !rf.shouleReplicateLog(peer) && !rf.killed() {
			rf.replicateCond[peer].Wait()
		}
		if rf.killed() {
			return
		}

		// send new log to other peers
		rf.doSendAppendEntry(peer)
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *tester.Persister, applyCh chan raftapi.ApplyMsg) raftapi.Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (3A, 3B, 3C).
	rf.currentTerm = 0
	rf.voteFor = -1
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.dead = 0

	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))

	// a server become the follower by default when it start up
	rf.state = Follower

	rf.electionTimer = time.NewTimer(GeneratingElectionTimeout())
	rf.heartbeatTimer = time.NewTimer(GeneratingHearbeatMsgTimeout())

	rf.applyCh = applyCh

	// 3B
	// using mutex + condition variable to protect the critical section
	// from other go routine that using the raft receiver
	rf.logs = make([]LogEntry, 1)
	rf.replicateCond = make([]*sync.Cond, len(peers))
	rf.applyCond = sync.NewCond(&rf.mu)

	// init the next array and commit log index for peers
	for peer := range peers {
		rf.updateLogIndices(peer, 0)
		rf.nextIndex[peer] = rf.getLatestLog().Index + 1
		if peer != rf.me {
			rf.replicateCond[peer] = sync.NewCond(&sync.Mutex{})
			// start replictor to replica log to peers
			go rf.replicator(peer)
		}
	}
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()
	// apply the new log entry to its local state machine
	go rf.applier()
	return rf
}
