package kvraft

import (
	"sync/atomic"
	"time"

	"6.5840/kvsrv1/rpc"
	kvtest "6.5840/kvtest1"
	tester "6.5840/tester1"
)

type Clerk struct {
	clnt    *tester.Clnt
	servers []string
	// You will have to modify this struct.
	clientId  int64
	seqNum    int64
	leaderIdx int // record the previous leader index, 0 is default
}

var globalClientId int64

func MakeClerk(clnt *tester.Clnt, servers []string) kvtest.IKVClerk {
	ck := &Clerk{
		clnt:      clnt,
		servers:   servers,
		// Use a monotonically increasing ID to avoid collisions when many
		// clerks are created concurrently.
		clientId:  atomic.AddInt64(&globalClientId, 1),
		seqNum:    0,
		leaderIdx: 0,
	}
	// You'll have to add code here.
	return ck
}

// Get fetches the current value and version for a key.  It returns
// ErrNoKey if the key does not exist. It keeps trying forever in the
// face of all other errors.
//
// You can send an RPC to server i with code like this:
// ok := ck.clnt.Call(ck.servers[i], "KVServer.Get", &args, &reply)
//
// The types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. Additionally, reply must be passed as a pointer.
func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {

	// You will have to modify this function.
	ck.seqNum++
	var reply rpc.GetReply
	args := rpc.GetArgs{
		Key:      key,
		SeqNum:   ck.seqNum,
		ClientId: ck.clientId,
	}

	for {
		reply = rpc.GetReply{}
		// Try previous leader first
		ok := ck.clnt.Call(ck.servers[ck.leaderIdx], "KVServer.Get", &args, &reply)
		if ok && (reply.Err == rpc.OK || reply.Err == rpc.ErrNoKey) {
			return reply.Value, reply.Version, reply.Err
		}

		// Find and update new leader from the rest of servers
		for idx, srv := range ck.servers {
			if idx == ck.leaderIdx {
				continue
			}
			ok = ck.clnt.Call(srv, "KVServer.Get", &args, &reply)
			if ok && (reply.Err == rpc.OK || reply.Err == rpc.ErrNoKey) {
				ck.leaderIdx = idx
				return reply.Value, reply.Version, reply.Err
			}
		}

		// All servers failed or returned ErrWrongLeader, wait before retrying
		time.Sleep(10 * time.Millisecond)
	}
}

// Put updates key with value only if the version in the
// request matches the version of the key at the server.  If the
// versions numbers don't match, the server should return
// ErrVersion.  If Put receives an ErrVersion on its first RPC, Put
// should return ErrVersion, since the Put was definitely not
// performed at the server. If the server returns ErrVersion on a
// resend RPC, then Put must return ErrMaybe to the application, since
// its earlier RPC might have been processed by the server successfully
// but the response was lost, and the the Clerk doesn't know if
// the Put was performed or not.
//
// You can send an RPC to server i with code like this:
// ok := ck.clnt.Call(ck.servers[i], "KVServer.Put", &args, &reply)
//
// The types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. Additionally, reply must be passed as a pointer.
func (ck *Clerk) Put(key string, value string, version rpc.Tversion) rpc.Err {
	// You will have to modify this function.
	ck.seqNum++

	var reply rpc.PutReply

	args := rpc.PutArgs{
		Key:      key,
		Value:    value,
		Version:  version,
		ClientId: ck.clientId,
		SeqNum:   ck.seqNum,
	}

	// If we've had any RPC failure (ok==false), a previous attempt may have been
	// processed by the server even though we didn't see a reply.
	retried := false

	for {
		reply = rpc.PutReply{}

		// Try previous leader first
		ok := ck.clnt.Call(ck.servers[ck.leaderIdx], "KVServer.Put", &args, &reply)

		if !ok {
			retried = true
		} else if reply.Err != rpc.ErrWrongLeader {
			if reply.Err == rpc.ErrVersion && retried {
				return rpc.ErrMaybe
			}
			return reply.Err
		}

		for idx, srv := range ck.servers {
			if idx == ck.leaderIdx {
				continue
			}

			ok = ck.clnt.Call(srv, "KVServer.Put", &args, &reply)
			if !ok {
				retried = true
				continue
			}

			if reply.Err != rpc.ErrWrongLeader {
				ck.leaderIdx = idx
				if reply.Err == rpc.ErrVersion && retried {
					return rpc.ErrMaybe
				}
				return reply.Err
			}
		}

		// All servers failed or returned ErrWrongLeader, wait before retrying
		time.Sleep(10 * time.Millisecond)
	}
}
