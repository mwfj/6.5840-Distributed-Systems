package kvsrv

import (
	"math/rand"
	"time"

	"6.5840/kvsrv1/rpc"
	kvtest "6.5840/kvtest1"
	tester "6.5840/tester1"
)

type Clerk struct {
	clnt     *tester.Clnt
	server   string
	clientId int64
	seqNum   int64
}

func MakeClerk(clnt *tester.Clnt, server string) kvtest.IKVClerk {
	ck := &Clerk{
		clnt:     clnt,
		server:   server,
		clientId: rand.New(rand.NewSource(time.Now().UnixNano())).Int63(),
		seqNum:   0,
	}
	// You may add code here.
	return ck
}

// Get fetches the current value and version for a key.  It returns
// ErrNoKey if the key does not exist. It keeps trying forever in the
// face of all other errors.
//
// You can send an RPC with code like this:
// ok := ck.clnt.Call(ck.server, "KVServer.Get", &args, &reply)
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
		ClientId: ck.clientId,
		SeqNum:   ck.seqNum,
	}
	ok := false

	for !ok {
		reply = rpc.GetReply{}
		ok = ck.clnt.Call(ck.server, "KVServer.Get", &args, &reply)
	}
	return reply.Value, reply.Version, reply.Err
}

// Put updates key with value only if the version in the
// request matches the version of the key at the server.  If the
// versions numbers don't match, the server should return
// ErrVersion.  If Put receives an ErrVersion on its first RPC, Put
// should return ErrVersion, since the Put was definitely not
// performed at the server. If the server returns ErrVersion on a
// resend RPC, then Put must return ErrMaybe to the application, since
// its earlier RPC might have been processed by the server successfully
// but the response was lost, and the Clerk doesn't know if
// the Put was performed or not.
//
// You can send an RPC with code like this:
// ok := ck.clnt.Call(ck.server, "KVServer.Put", &args, &reply)
//
// The types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. Additionally, reply must be passed as a pointer.
func (ck *Clerk) Put(key, value string, version rpc.Tversion) rpc.Err {
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
	ok := false
	firstAttempt := true

	for !ok {
		reply = rpc.PutReply{}
		ok = ck.clnt.Call(ck.server, "KVServer.Put", &args, &reply)
		if reply.Err == rpc.ErrVersion {
			if !firstAttempt {
				reply.Err = rpc.ErrMaybe
			}
		}
		firstAttempt = false
	}

	return reply.Err
}
