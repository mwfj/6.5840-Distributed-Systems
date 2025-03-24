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

	"os"
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

	nextIndex []int // index of the next log entry to send to that server
	matchIdex []int // index of highest log entry applied to state machine

	electionTimer  *time.Timer // the timer for election timeout, for follower
	heartbeatTimer *time.Timer // the timer for heartbeat timeout, for leader

	applyCh chan ApplyMsg // channel to send

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
	// LastLogIndex int // index of candidate's last log entry
	// LastLogTerm  int // term of candidate's last log entry

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

	PrevLogTerm int   // term of prevLogIndex Entry
	Entries     []int // log entry to store

	LeaderCommit int // leader's commitIndex

}

type AppendEntryReply struct {
	// Your data here (3A).
	// current term, for leader to update itself true
	// if follower contained entry matching prevLogIndex and prevLogTerm
	Term int
	// true if follower contained entry matching prevLogIndex and prevLogTerm
	Success bool // candidate requesting vote
}

// Initialed by leaders to replicate log entries and to provide a form of heartbeat,
// where heartbeat message is an AppendEntry that carry no log entry
func (rf *Raft) AppendEntry(args *AppendEntryArgs, reply *AppendEntryReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

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
	rf.ChangeState(Follower)
	// here make the test case happy, reset election timer again
	// otherwise, warning throwed when test case check the term
	rf.electionTimer.Reset(GeneratingElectionTimeout())

	reply.Term, reply.Success = rf.currentTerm, true
}

func (rf *Raft) sendAppendEntry(server int, args *AppendEntryArgs, reply *AppendEntryReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntry", args, reply)
	return ok
}

func (rf *Raft) LaunchElection() {
	// vote for himself
	rf.voteFor = rf.me
	grantedVotes := 1

	voteArgs := &RequestVoteArgs{
		Term:        rf.currentTerm,
		CandidateId: rf.me,
	}

	DPrintf("[%v] %v launch a new leader election with requestvote args %v", os.Getpid(), rf.me, voteArgs)

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
				DPrintf("[%v] %v after got reply for sendRequestVote from node %v, vote reply %v", os.Getpid(), rf.me, peer, voteReply)
				if voteArgs.Term == rf.currentTerm && rf.state == Candidate {
					if voteReply.VoteGranted {
						grantedVotes++
						// claim as leader if this server received vote from most of peers
						if grantedVotes > len(rf.peers)/2 {
							DPrintf("[%v] node %v claim as leader with the number of votes %v", os.Getpid(), rf.me, grantedVotes)
							rf.ChangeState(Leader)
							// send heart beat message
							rf.BroadcastHeartBeatMsg()
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

func (rf *Raft) BroadcastHeartBeatMsg() {
	for peer := range rf.peers {
		if peer == rf.me {
			continue
		}

		// send heartbeat message
		// to the rest of peers
		go func(peer int) {
			rf.mu.RLock()
			if rf.state != Leader {
				rf.mu.RUnlock()
				return
			}

			heartbeatArgs := &AppendEntryArgs{
				Term:     rf.currentTerm,
				LeaderId: rf.me,
			}

			rf.mu.RUnlock()
			heartbeatReply := &AppendEntryReply{}

			if rf.sendAppendEntry(peer, heartbeatArgs, heartbeatReply) {
				rf.mu.Lock()
				if heartbeatArgs.Term == rf.currentTerm && rf.state == Leader {
					// handle failure
					if !heartbeatReply.Success {
						if heartbeatReply.Term > rf.currentTerm {
							rf.ChangeState(Follower)
							rf.currentTerm, rf.voteFor = heartbeatReply.Term, -1
						}
					}
				}
				rf.mu.Unlock()
			}

		}(peer)
	}
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
	index := -1
	term := -1
	isLeader := true

	// Your code here (3B).

	return index, term, isLeader
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
				rf.BroadcastHeartBeatMsg()
				rf.heartbeatTimer.Reset(GeneratingHearbeatMsgTimeout())
			}
			rf.mu.Unlock()
		}
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
	rf.matchIdex = make([]int, len(peers))

	// a server become the follower by default when it start up
	rf.state = Follower

	rf.electionTimer = time.NewTimer(GeneratingElectionTimeout())
	rf.heartbeatTimer = time.NewTimer(GeneratingHearbeatMsgTimeout())

	rf.applyCh = applyCh

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
