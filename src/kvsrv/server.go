package kvsrv

/**
 * 1. data stored in the kay value paired map
 * 2. append operation appends arg to key's value and returns the old value
 * 	  append to a non-existent key should act as if the existing value were a zero-length string.
 * 3. get operation will return empty string if there is no key found in map
 * 4. put operation installs or replaces the value for a particular key in the map
 *
 * Your server must arrange that application calls to Clerk Get/Put/Append methods be linearizable.
 */
import (
	"log"
	"sync"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

/**
 * Using duplicate table to make sure server only process one request from client
 */
type dupTab struct {
	seq   int
	value string
}

type KVServer struct {
	mu sync.Mutex

	// Your definitions here.
	cache     map[string]string // client ID -> value
	clientMap map[int64]*dupTab // each client id will map the corresponding duptab
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Always get the cache data if this client has not store data in cache
	if _, ok := kv.clientMap[args.Id]; !ok {
		reply.Value = kv.cache[args.Key]
		return
	}

	// The sequence id is not new, return the previous value
	cliDupTab := kv.clientMap[args.Id]
	if cliDupTab.seq == args.Seq {
		reply.Value = cliDupTab.value
		return
	}

	// Update clientMap otherwise
	cliDupTab.seq = args.Seq
	cliDupTab.value = kv.cache[args.Key]
	// Reply the latest data in cache
	reply.Value = kv.cache[args.Key]
	//Log here

}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Duplicate detection
	// If this client has never send request to server,
	// just store the key value pair
	if kv.clientMap[args.Id] == nil {
		kv.cache[args.Key] = args.Value
		return
	}

	dupTab := kv.clientMap[args.Id]

	// If seq exist,
	// it represent the current request has been proceed, ignore
	if dupTab.seq == args.Seq {
		return
	}

	// Otherwise, update the duptable
	dupTab.seq = args.Seq
	dupTab.value = ""
	kv.cache[args.Key] = args.Value

}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Duplication detection
	// If this client has never send request to server before,
	// create new dup tab
	if kv.clientMap[args.Id] == nil {
		kv.clientMap[args.Id] = &dupTab{seq: -1, value: ""}
	}

	dupTab := kv.clientMap[args.Id]

	if dupTab.seq == args.Seq {
		reply.Value = dupTab.value
		return
	}

	dupTab.seq = args.Seq
	// Get the old data in cache
	dupTab.value = kv.cache[args.Key]
	reply.Value = dupTab.value
	// Append data
	kv.cache[args.Key] = kv.cache[args.Key] + args.Value

}

func StartKVServer() *KVServer {
	kv := new(KVServer)

	// You may need initialization code here.
	kv.cache = make(map[string]string)
	kv.clientMap = make(map[int64]*dupTab)

	return kv
}
