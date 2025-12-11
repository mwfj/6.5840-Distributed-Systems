package kvsrv

import (
	"log"
	"sync"

	"6.5840/kvsrv1/rpc"
	"6.5840/labrpc"
	tester "6.5840/tester1"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

// each key is mapping a (value, version) pair
type KVPair struct {
	value   string
	version rpc.Tversion
}

// duplicate table entry for each client
type dupTab struct {
	lastSeqNum int64
	lastReply  rpc.PutReply
}

type KVServer struct {
	mu sync.Mutex

	// Your definitions here.
	cache     map[string]*KVPair       // key -> (value, version)
	clientMap map[int64]*dupTab        // each client id will map the corresponding duptab
}

func MakeKVServer() *KVServer {
	kv := &KVServer{
		cache:     make(map[string]*KVPair),
		clientMap: make(map[int64]*dupTab),
	}
	// Your code here.
	return kv
}

// Get returns the value and version for args.Key, if args.Key
// exists. Otherwise, Get returns ErrNoKey.
func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	// Your code here.
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if pair, ok := kv.cache[args.Key]; ok {
		reply.Value = pair.value
		reply.Version = pair.version
		reply.Err = rpc.OK
	} else {
		reply.Err = rpc.ErrNoKey
	}
}

// Update the value for a key if args.Version matches the version of
// the key on the server. If versions don't match, return ErrVersion.
// If the key doesn't exist, Put installs the value if the
// args.Version is 0, and returns ErrNoKey otherwise.
func (kv *KVServer) Put(args *rpc.PutArgs, reply *rpc.PutReply) {
	// Your code here.
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// check for duplicate request - only if operation would still succeed
	if dup, ok := kv.clientMap[args.ClientId]; ok {
		if args.SeqNum == dup.lastSeqNum && dup.lastReply.Err == rpc.OK {
			// check if this operation would still succeed with current state
			if pair, exists := kv.cache[args.Key]; exists {
				if args.Version == pair.version {
					// operation would still succeed, return cached reply
					*reply = dup.lastReply
					return
				}
			} else if args.Version == 0 {
				// key doesn't exist and version is 0, operation would still succeed
				*reply = dup.lastReply
				return
			}
			// state has changed, process as new operation
		}
	}

	if pair, ok := kv.cache[args.Key]; ok {

		// check the version if exist
		if args.Version == pair.version {
			// version matched, update KVPair
			kv.cache[args.Key] = &KVPair{value: args.Value, version: pair.version + 1}
			reply.Err = rpc.OK
			// cache successful operation
			kv.clientMap[args.ClientId] = &dupTab{
				lastSeqNum: args.SeqNum,
				lastReply:  *reply,
			}
		} else {
			reply.Err = rpc.ErrVersion
		}
	} else {
		if args.Version == 0 {
			// reset version
			kv.cache[args.Key] = &KVPair{value: args.Value, version: 1}
			reply.Err = rpc.OK
			// cache successful operation
			kv.clientMap[args.ClientId] = &dupTab{
				lastSeqNum: args.SeqNum,
				lastReply:  *reply,
			}
		} else {
			reply.Err = rpc.ErrNoKey
		}
	}
}

// You can ignore Kill() for this lab
func (kv *KVServer) Kill() {
}

// You can ignore all arguments; they are for replicated KVservers
func StartKVServer(ends []*labrpc.ClientEnd, gid tester.Tgid, srv int, persister *tester.Persister) []tester.IService {
	kv := MakeKVServer()
	return []tester.IService{kv}
}
