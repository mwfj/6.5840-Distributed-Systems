package kvraft

import (
	"bytes"
	"sync"
	"sync/atomic"

	"6.5840/kvraft1/rsm"
	"6.5840/kvsrv1/rpc"
	"6.5840/labgob"
	"6.5840/labrpc"
	tester "6.5840/tester1"
)

type KVPair struct {
	Value   string
	Version rpc.Tversion
}

// Record client operation results for deduplication
// Maps seqNum -> cached reply for exactly-once semantics
type dupTab struct {
	Replies map[int64]RaftReplyMsg // seqNum -> reply
	MaxSeqNum int64  // highest seqNum we've seen (for garbage collection)
}

type KVServer struct {
	me   int
	dead int32 // set by Kill()
	rsm  *rsm.RSM

	// Your definitions here.
	mu    sync.Mutex
	cache map[string]*KVPair // key => (value, version)
	// Each client will map the Duplicate Table to record the last client get operation
	clientMap map[int64]*dupTab
}

const (
	GetMethod = "Get"
	PutMethod = "Put"
)

type RaftReqMsg struct {
	Command  string
	Key      string
	Value    string // Empty string for Get method
	SeqNum   int64
	ClientId int64
	Version  rpc.Tversion // Only Put method use this
}

type RaftReplyMsg struct {
	Command string
	Value   string // Not used in Put method
	Version rpc.Tversion
	Err     rpc.Err
}

// To type-cast req to the right type, take a look at Go's type switches or type
// assertions below:
//
// https://go.dev/tour/methods/16
// https://go.dev/tour/methods/15
func (kv *KVServer) DoOp(req any) any {
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// RSM passes RaftReqMsg directly (not wrapped in Op)
	// Handle both pointer and value types
	var raftReq *RaftReqMsg

	if ptr, ok := req.(*RaftReqMsg); ok {
		raftReq = ptr
	} else if val, ok := req.(RaftReqMsg); ok {
		raftReq = &val
	} else {
		// Should never happen; return a safe error so the client retries.
		return RaftReplyMsg{Err: rpc.ErrWrongLeader}
	}

	// Deduplication: Check if we've already processed this exact request (ClientId, SeqNum pair)
	if dup, ok := kv.clientMap[raftReq.ClientId]; ok {
		if dup.Replies != nil {
			if reply, exists := dup.Replies[raftReq.SeqNum]; exists {
				// Exact duplicate - return cached result for exactly-once semantics
				return reply
			}
		}
	}

	replyMsg := &RaftReplyMsg{
		Command: raftReq.Command,
		Value:   "",
		Version: raftReq.Version,
	}
	switch raftReq.Command {
	case GetMethod:
		if kvPair, exists := kv.cache[raftReq.Key]; exists {
			replyMsg.Value = kvPair.Value
			replyMsg.Version = kvPair.Version
			replyMsg.Err = rpc.OK
		} else {
			replyMsg.Err = rpc.ErrNoKey
		}

	case PutMethod:
		if kvPair, exist := kv.cache[raftReq.Key]; exist {
			if kvPair.Version == raftReq.Version {

				kvPair.Version++
				kvPair.Value = raftReq.Value
				kv.cache[raftReq.Key] = kvPair

				replyMsg.Version = kvPair.Version
				replyMsg.Err = rpc.OK
			} else {
				// This log is for debug, will print a lot when you run the unit test
				// fmt.Printf("Version mismatched, Put failed. Old version: %v - Incoming Verion: %v", kvPair.Version, raftReq.Version)
				replyMsg.Err = rpc.ErrVersion
			}
		} else {
			if raftReq.Version == 0 {
				kv.cache[raftReq.Key] = &KVPair{
					Value:   raftReq.Value,
					Version: 1,
				}
				replyMsg.Version = 1
				replyMsg.Err = rpc.OK
			} else {
				// In this lab, a missing key behaves like version 0, so any
				// non-zero version is a version mismatch.
				replyMsg.Err = rpc.ErrVersion
			}
		}

	default:
		return RaftReplyMsg{Err: rpc.ErrWrongLeader}
	}

	// Cache the result for this specific (ClientId, SeqNum) pair
	if dup, ok := kv.clientMap[raftReq.ClientId]; ok {
		// Ensure Replies map is initialized (safety check for snapshot restore)
		if dup.Replies == nil {
			dup.Replies = make(map[int64]RaftReplyMsg)
		}
		dup.Replies[raftReq.SeqNum] = *replyMsg
		if raftReq.SeqNum > dup.MaxSeqNum {
			dup.MaxSeqNum = raftReq.SeqNum
			// Garbage collect old entries to prevent unbounded growth
			// Keep only recent entries (within 100 of max)
			for seqNum := range dup.Replies {
				if seqNum < dup.MaxSeqNum-100 {
					delete(dup.Replies, seqNum)
				}
			}
		}
	} else {
		kv.clientMap[raftReq.ClientId] = &dupTab{
			Replies:   map[int64]RaftReplyMsg{raftReq.SeqNum: *replyMsg},
			MaxSeqNum: raftReq.SeqNum,
		}
	}

	return *replyMsg
}

func (kv *KVServer) Snapshot() []byte {
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Create buffer and encode
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)

	// Encode data
	e.Encode(kv.cache)

	// Don't snapshot the duplicate table at all - it will be rebuilt after restore
	// Snapshotting it causes issues when operations are replayed after restore
	emptyDupTab := make(map[int64]*dupTab)
	e.Encode(emptyDupTab)

	return w.Bytes()
}

func (kv *KVServer) Restore(snapshot []byte) {
	// Your code here
	if len(snapshot) == 0 {
		return
	}

	// Create Buffer and Decoder
	r := bytes.NewBuffer(snapshot)
	d := labgob.NewDecoder(r)

	var newCache map[string]*KVPair
	var newDupTab map[int64]*dupTab

	// Decode snapshot (can be slow, so do it without holding lock)
	if d.Decode(&newCache) != nil || d.Decode(&newDupTab) != nil {
		panic("Failed to decode snapshot")
	}

	// Now acquire lock and atomically update both cache and clientMap
	// This ensures DoOp sees consistent state
	kv.mu.Lock()
	kv.cache = newCache
	kv.clientMap = newDupTab
	kv.mu.Unlock()
}

func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	// Your code here. Use kv.rsm.Submit() to submit args
	// You can use go's type casts to turn the any return value
	// of Submit() into a GetReply: rep.(rpc.GetReply)

	if kv.rsm == nil || kv.killed() {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	// Only leader can reply (fast fail optimization)
	_, isLeader := kv.rsm.Raft().GetState()
	if !isLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	raftMsg := RaftReqMsg{
		Command:  GetMethod,
		Key:      args.Key,
		Value:    "",
		SeqNum:   args.SeqNum,
		ClientId: args.ClientId,
		Version:  0,
	}
	err, result := kv.rsm.Submit(raftMsg)

	if err != rpc.OK {
		reply.Err = err
		return
	}

	resonse, ok := result.(RaftReplyMsg)

	if !ok {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	if resonse.Command != GetMethod {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	reply.Value = resonse.Value
	reply.Version = resonse.Version
	reply.Err = resonse.Err
}

func (kv *KVServer) Put(args *rpc.PutArgs, reply *rpc.PutReply) {
	// Your code here. Use kv.rsm.Submit() to submit args
	// You can use go's type casts to turn the any return value
	// of Submit() into a PutReply: rep.(rpc.PutReply)

	if kv.rsm == nil || kv.killed() {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	// Only leader can reply (fast fail optimization)
	_, isLeader := kv.rsm.Raft().GetState()
	if !isLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	raftMsg := RaftReqMsg{
		Command:  PutMethod,
		Key:      args.Key,
		Value:    args.Value,
		SeqNum:   args.SeqNum,
		ClientId: args.ClientId,
		Version:  args.Version,
	}
	err, result := kv.rsm.Submit(raftMsg)

	if err != rpc.OK {
		reply.Err = err
		return
	}

	resonse, ok := result.(RaftReplyMsg)

	if !ok {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	if resonse.Command != PutMethod {
		reply.Err = rpc.ErrWrongLeader
		return
	}
	reply.Err = resonse.Err
}

// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	if kv.rsm != nil {
		kv.rsm.Kill()
	}
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// StartKVServer() and MakeRSM() must return quickly, so they should
// start goroutines for any long-running work.
func StartKVServer(servers []*labrpc.ClientEnd, gid tester.Tgid, me int, persister *tester.Persister, maxraftstate int) []tester.IService {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(rsm.Op{})
	labgob.Register(rpc.PutArgs{})
	labgob.Register(rpc.GetArgs{})
	labgob.Register(RaftReqMsg{})
	labgob.Register(RaftReplyMsg{})

	kv := &KVServer{me: me}

	// You may need initialization code here.
	kv.cache = make(map[string]*KVPair)
	kv.clientMap = make(map[int64]*dupTab)
	atomic.StoreInt32(&kv.dead, 0)

	// MakeRSM creates the Raft instance and handles all apply logic
	kv.rsm = rsm.MakeRSM(servers, me, persister, maxraftstate, kv)

	return []tester.IService{kv, kv.rsm.Raft()}
}
