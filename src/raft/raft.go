package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 3D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 3D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

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
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (3A, 3B, 3C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// 3A
	currentTerm int // latest term server has seen
	voteFor     int // candidatedId that received vote in current state
	commitIndex int // index of hightest log entry know to be committed
	lastApplied int // index of hightest log entry appiled to state machine

	state ServerState // the current server state in the election

	nextIndex  []int // index of the next log entry to send to that server
	matchIndex []int // index of highest log entry applied to state machine

	electionTimer  *time.Timer // the timer for election timeout, for follower
	heartbeatTimer *time.Timer // the timer for heartbeat timeout, for leader

	applyCh chan ApplyMsg // channel to send

	// 3B new add
	logs          []LogEntry
	applyCond     *sync.Cond   // the condition variable for applyCh
	replicateCond []*sync.Cond // the condition variable for notifing all of peers to send log

}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	// Your code here (3A).
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.currentTerm, rf.state == Leader
}

func (rf *Raft) IsFollower() (int, bool) {

	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.currentTerm, rf.state == Follower
}

func (rf *Raft) IsCandidate() (int, bool) {

	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.currentTerm, rf.state == Candidate
}

func (rf *Raft) ChangeState(newstate ServerState) {
	// do nothing when the state is not changed
	if rf.state == newstate {
		return
	}
	rf.state = newstate
	switch newstate {
	case Follower:
		// follower election timout only
		rf.electionTimer.Reset(GeneratingElectionTimeout())
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
		rf.matchIndex[rf.me] = last // but the leader has every entry itself
	// do nothing for candidate
	default:
	}
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (3C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (3C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (3D).

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

// Initialed by candidated during election §5.2
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).
	// 3A
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// §5.1 reply false if term < current Term
	// §5.2 If votedFor is null or not the candidateId,
	///     then reject the vote
	if (args.Term < rf.currentTerm) ||
		(args.Term == rf.currentTerm && rf.voteFor != -1 && rf.voteFor != args.CandidateId) {
		reply.Term, reply.VoteGranted = rf.currentTerm, false
		return
	}

	// if the current term that this server hold is not the latest
	// revert itself to follower
	if args.Term > rf.currentTerm {
		rf.ChangeState(Follower)
		rf.currentTerm, rf.voteFor = args.Term, -1
	}

	// 3B
	// §5.4: If candidate's log is not up-to-date, reject the vote request
	if !rf.isLogUpdateToDate(args.LastLogIndex, args.LastLogTerm) {
		reply.Term, reply.VoteGranted = rf.currentTerm, false
		return
	}

	// vote for the first election request that has potential become leader
	// first come first serve
	rf.voteFor = args.CandidateId
	rf.electionTimer.Reset(GeneratingElectionTimeout())

	reply.Term, reply.VoteGranted = rf.currentTerm, true
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
	entries := make([]LogEntry, len(rf.logs[prevLogIndex-firstLogIndex+1:]))
	copy(entries, rf.logs[prevLogIndex-firstLogIndex+1:])
	args := &AppendEntryArgs{
		Term:         rf.currentTerm,
		LeaderId:     rf.me,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  rf.logs[prevLogIndex-firstLogIndex].Term,
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
		reply.Term, reply.Success = rf.currentTerm, false
		return
	}

	// find out the current term is not the latest, then update it
	if args.Term > rf.currentTerm {
		rf.currentTerm, rf.voteFor = args.Term, -1
	}

	// change state to follower
	if rf.state != Follower {
		rf.ChangeState(Follower)
	}
	// here make the test case happy, reset election timer again
	// otherwise, warning throwed when test case check the term
	rf.electionTimer.Reset(GeneratingElectionTimeout())

	// 3B
	// §5.3: reply false if log doesn't contain an entry at prevLogIndex
	//       whose term matches preLogTerm
	if args.PrevLogIndex < rf.getFirstLog().Index {
		reply.Term, reply.Success = rf.currentTerm, false
		return
	}

	firstLogIdx := rf.getFirstLog().Index

	// §5.3: If an existing entry conflict with a new one
	//       (same index but different terms),
	//       delete the existing entry and all that follow it
	if !rf.isLogMatched(args.PrevLogIndex, args.PrevLogTerm) {
		firstLogIdx := rf.getFirstLog().Index
		lastLogIdx := rf.getLatestLog().Index

		reply.Term, reply.Success = rf.currentTerm, false
		// the current server doesn't have that high index, return its the next index
		if lastLogIdx < args.PrevLogIndex {
			reply.ConflictIndex, reply.ConflictTerm = lastLogIdx+1, -1
		} else {
			conflictTerm := rf.logs[args.PrevLogIndex-firstLogIdx].Term
			reply.ConflictTerm = conflictTerm
			// find the first conflict term in follower's log
			index := args.PrevLogIndex

			for index >= firstLogIdx && rf.logs[index-firstLogIdx].Term == args.PrevLogTerm {
				index--
			}
			reply.ConflictIndex = index + 1
		}

		return
	}

	// append any new entries that not already in the log
	for index, entry := range args.Entries {
		if (entry.Index-firstLogIdx) >= len(rf.logs) || (rf.logs[entry.Index-firstLogIdx].Term != entry.Term) {
			rf.logs = append(rf.logs[:entry.Index-firstLogIdx], args.Entries[index:]...)
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

func (rf *Raft) sendAppendEntry(server int, args *AppendEntryArgs, reply *AppendEntryReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntry", args, reply)
	return ok
}

func (rf *Raft) LaunchElection() {
	// vote for himself first
	rf.voteFor = rf.me
	grantedVotes := 1

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
				if voteArgs.Term == rf.currentTerm && rf.state == Candidate {
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
						rf.ChangeState(Follower)
						rf.currentTerm, rf.voteFor = voteReply.Term, -1
					}
				}
			}
		}(peer)
	}
}

/**
 * For the heartbeat message, we will send the AppendEntry RPC immdiatly
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
	rf.matchIndex[peer] = args.PrevLogIndex + len(args.Entries)
	rf.nextIndex[peer] = rf.matchIndex[peer] + 1

	matchLen := len(rf.matchIndex)
	sortedIdxes := make([]int, matchLen)
	copy(sortedIdxes, rf.matchIndex)
	// Get the current highest commit index
	sort.Ints(sortedIdxes)

	newCommitIdx := sortedIdxes[matchLen-(matchLen/2+1)]

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
		found := false
		// find the right index that this matched conflict term
		for i := args.PrevLogIndex - 1; i >= firstLogIdx; i-- {
			if rf.logs[i-firstLogIdx].Term == reply.ConflictTerm {
				rf.nextIndex[peer] = i + 1
				found = true
				break
			}
		}
		// If no entry with ConflictTerm exists in our log,
		// keep ConflictIndex as provided by the follower.
		if !found {
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
	args := rf.genAppendEntryArgs(prevLogIdx)

	rf.mu.RUnlock()
	reply := &AppendEntryReply{}

	if !rf.sendAppendEntry(peer, args, reply) {
		DPrintf("[Node %v] sendAppendEntry failed. peer %v, args %v, reply %v", rf.me, peer, args, reply)
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// make sure that the leader's term is the latest
	if (args.Term == rf.currentTerm) && (rf.state == Leader) {
		// handle failure
		if !reply.Success {
			if reply.Term > rf.currentTerm {
				// discovered higher term → step down
				rf.ChangeState(Follower)
				rf.currentTerm, rf.voteFor = reply.Term, -1
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

func (rf *Raft) isLogMatched(index, term int) bool {
	return (index <= rf.getLatestLog().Index) && (term == rf.logs[index-rf.getFirstLog().Index].Term)
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

	// update log index info
	rf.matchIndex[rf.me], rf.nextIndex[rf.me] = newLatestLogIdx, newLatestLogIdx+1

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
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (3A)
		// Check if a leader election should be started.
		select {
		// received election timeout(follower)
		case <-rf.electionTimer.C:
			rf.mu.Lock()
			rf.ChangeState(Candidate)
			rf.currentTerm++
			// lauch a new leader election
			rf.LaunchElection()
			rf.electionTimer.Reset(GeneratingElectionTimeout())
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

// 3B using appiler to save new log created entry into local
func (rf *Raft) applier() {
	for rf.killed() == false {
		rf.mu.Lock()
		// check whehter its commit index is the latest
		for rf.commitIndex <= rf.lastApplied {
			// wait the commit index update to the latest
			rf.applyCond.Wait()
		}

		// apply the latest commit log to state machine
		firstLogIdx, commitLogIdx, lastApplied := rf.getFirstLog().Index, rf.commitIndex, rf.lastApplied
		newEntryies := make([]LogEntry, commitLogIdx-lastApplied)
		copy(newEntryies, rf.logs[(lastApplied-firstLogIdx+1):(commitLogIdx-firstLogIdx+1)])
		rf.mu.Unlock()

		// send each newly log entries into apply channel
		for _, newEntry := range newEntryies {
			rf.applyCh <- ApplyMsg{
				CommandValid: true,
				Command:      newEntry.Command,
				CommandIndex: newEntry.Index,
			}
		}

		// update last applied
		rf.mu.Lock()
		DPrintf("[Node %v] applies new log entry from the index %v to %v in term %v", rf.me, rf.lastApplied+1, commitLogIdx, rf.currentTerm)
		rf.lastApplied = commitLogIdx
		rf.mu.Unlock()
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
		for !rf.shouleReplicateLog(peer) {
			rf.replicateCond[peer].Wait()
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
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (3A, 3B, 3C).
	// 3A
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

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	// 3B
	// using mutex + condition variable to protect the critical section
	// from other go routine that using the raft receiver
	rf.logs = make([]LogEntry, 1)
	rf.replicateCond = make([]*sync.Cond, len(peers))
	rf.applyCond = sync.NewCond(&rf.mu)

	// init the next array and commit log index for peers
	for peer := range peers {
		rf.matchIndex[peer], rf.nextIndex[peer] = 0, rf.getLatestLog().Index+1
		if peer != rf.me {
			rf.replicateCond[peer] = sync.NewCond(&sync.Mutex{})
			// start replictor to replica log to peers
			go rf.replicator(peer)
		}
	}

	// apply the new log entry to its local state machine
	go rf.applier()
	return rf
}
