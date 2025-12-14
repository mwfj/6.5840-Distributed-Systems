# MIT 6.5840 Distributed Systems

### updating ... ...

- [x] **[Lab1: Map Reduce](https://pdos.csail.mit.edu/6.824/labs/lab-mr.html)** : In this lab you'll build a MapReduce system. You'll implement a worker process that calls application Map and Reduce functions and handles reading and writing files, and a coordinator process that hands out tasks to workers and copes with failed workers. You'll be building something similar to the [MapReduce paper](http://research.google.com/archive/mapreduce-osdi04.pdf). (Note: this lab uses "coordinator" instead of the paper's "master".)

  - [**MapReduce Paper Summary**](https://github.com/mwfj/6.5840-Distributed-Systems/blob/master/paper_summary/MapReducePaperSummary.md)

  - [**Lab1 main code change**](https://github.com/mwfj/6.5840-Distributed-Systems/pull/3/files) 

  - [Fix dial error issue when running the test](https://github.com/mwfj/6.5840-Distributed-Systems/pull/12)

- [x] **[Lab 2: Key/Value Server](http://nil.csail.mit.edu/6.5840/2024/labs/lab-kvsrv.html)** :In this lab you will build a key/value server for a single machine that ensures that each operation is executed exactly once despite network failures and that the operations are [linearizable](https://pdos.csail.mit.edu/6.824/papers/linearizability-faq.txt). Later labs will replicate a server like this one to handle server crashes.

  - [**Lab2 code change**](https://github.com/mwfj/6.5840-Distributed-Systems/pull/4/files)
  - [Lab2 Get method enhancement](https://github.com/mwfj/6.5840-Distributed-Systems/pull/14)

- [x] **[Lab 3: Raft](http://nil.csail.mit.edu/6.5840/2024/labs/lab-raft.html)** :This is the first in a series of labs in which you'll build a fault-tolerant key/value storage system. In this lab you'll implement Raft, a replicated state machine protocol. In the next lab you'll build a key/value service on top of Raft. Then you will “shard” your service over multiple replicated state machines for higher performance.

  In this lab you'll implement Raft as a Go object type with associated methods, meant to be used as a module in a larger service. A set of Raft instances talk to each other with RPC to maintain replicated logs. Your Raft interface will support an indefinite sequence of numbered commands, also called log entries. The entries are numbered with *index numbers*. The log entry with a given index will eventually be committed. At that point, your Raft should send the log entry to the larger service for it to execute.

  You should follow the design in the [extended Raft paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf), with particular attention to Figure 2. You'll implement most of what's in the paper, including saving persistent state and reading it after a node fails and then restarts. You will not implement cluster membership changes (Section 6).

  - **Code change in Lab 3 please refer to [here](./src/raft1/README.md)**

  - [X] Part A - Leader Election: 
    Implement Raft leader election and heartbeats (`AppendEntries` RPCs with no log entries). The goal for Part 3A is for a single leader to be elected, for the leader to remain the leader if there are no failures, and for a new leader to take over if the old leader fails or if packets to/from the old leader are lost. Run `go test -run 3A `to test your 3A code.
    
  - [x] Part B - Log
    Implement the leader and follower code to append new log entries, so that the `go test -run 3B `tests pass.
    
  - [x] Part C - Persistence
    Complete the functions `persist()` and `readPersist()` in `raft.go` by adding code to save and restore persistent state. You will need to encode (or "serialize") the state as an array of bytes in order to pass it to the `Persister`. Use the `labgob` encoder; see the comments in `persist()` and `readPersist()`. `labgob` is like Go's `gob` encoder but prints error messages if you try to encode structures with lower-case field names. For now, pass `nil` as the second argument to `persister.Save()`. Insert calls to `persist()` at the points where your implementation changes persistent state. Once you've done this, and if the rest of your implementation is correct, you should pass all of the 3C tests.
    
  - [x] Part D: log compaction 

    As things stand now, a rebooting server replays the complete Raft log in order to restore its state. However, it's not practical for a long-running service to remember the complete Raft log forever. Instead, you'll modify Raft to cooperate with services that persistently store a "snapshot" of their state from time to time, at which point Raft discards log entries that precede the snapshot. The result is a smaller amount of persistent data and faster restart. However, it's now possible for a follower to fall so far behind that the leader has discarded the log entries it needs to catch up; the leader must then send a snapshot plus the log starting at the time of the snapshot. Section 7 of the  [extended Raft paper](http://nil.csail.mit.edu/6.5840/2024/papers/raft-extended.pdf) outlines the scheme; you will have to design the details.

    Your Raft must provide the following function that the service can call with a serialized snapshot of its state:

    ```
    Snapshot(index int, snapshot []byte)
    ```

- [ ] [6.5840 Lab 4: Fault-tolerant Key/Value Service](https://pdos.csail.mit.edu/6.824/labs/lab-kvraft1.html): In this lab you will build a fault-tolerant key/value storage service using your Raft library from [Lab 3](https://pdos.csail.mit.edu/6.824/labs/lab-raft1.html). To clients, the service looks similar to the server of [Lab 2](https://pdos.csail.mit.edu/6.824/labs/lab-kvsrv1.html). However, instead of a single server, the service consists of a set of servers that use Raft to help them maintain identical databases. Your key/value service should continue to process client requests as long as a majority of the servers are alive and can communicate, in spite of other failures or network partitions. After Lab 4, you will have implemented all parts (Clerk, Service, and Raft) shown in the [diagram of Raft interactions](https://pdos.csail.mit.edu/6.824/figs/kvraft.pdf).

  Clients will interact with your key/value service through a Clerk, as in Lab 2. A Clerk implements the `Put` and `Get` methods with the same semantics as Lab 2: Puts are at-most-once and the Puts/Gets must form a linearizable history.

  Providing linearizability is relatively easy for a single server. It is harder if the service is replicated, since all servers must choose the same execution order for concurrent requests, must avoid replying to clients using state that isn't up to date, and must recover their state after a failure in a way that preserves all acknowledged client updates.

  This lab has three parts. In part A, you will implement a replicated-state machine package, `rsm`, using your raft implementation; `rsm` is agnostic of the requests that it replicates. In part B, you will implement a replicated key/value service using `rsm`, but without using snapshots. In part C, you will use your snapshot implementation from Lab 3D, which will allow Raft to discard old log entries. Please submit each part by the respective deadline.

  You should review the [extended Raft paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf), in particular Section 7 (but not 8). For a wider perspective, have a look at Chubby, Paxos Made Live, Spanner, Zookeeper, Harp, Viewstamped Replication, and [Bolosky et al.](http://static.usenix.org/event/nsdi11/tech/full_papers/Bolosky.pdf) 

  - [x] Part A: replicated state machine (RSM): In the common situation of a client/server service using Raft for replication, the service interacts with Raft in two ways: the service leader submits client operations by calling `raft.Start()`, and all service replicas receive committed operations via Raft's `applyCh`, which they execute. On the leader, these two activities interact. At any given time, some server goroutines are handling client requests, have called `raft.Start()`, and each is waiting for its operation to commit and to find out what the result of executing the operation is. And as committed operations appear on the `applyCh`, each needs to be executed by the service, and the results need to be handed to the goroutine that called `raft.Start()` so that it can return the result to the client.

    The `rsm` package encapsulates the above interaction. It sits as a layer between the service (e.g. a key/value database) and Raft. In `rsm/rsm.go` you will need to implement a "reader" goroutine that reads the `applyCh`, and a `rsm.Submit()` function that calls `raft.Start()` for a client operation and then waits for the reader goroutine to hand it the result of executing that operation.

    The service that is using `rsm` appears to the `rsm` reader goroutine as a `StateMachine` object providing a `DoOp()` method. The reader goroutine should hand each committed operation to `DoOp()`; `DoOp()`'s return value should be given to the corresponding `rsm.Submit()` call for it to return. `DoOp()`'s argument and return value have type `any`; the actual values should have the same types as the argument and return values that the service passes to `rsm.Submit()`, respectively.

    The service should pass each client operation to `rsm.Submit()`. To help the reader goroutine match `applyCh` messages with waiting calls to `rsm.Submit()`, `Submit()` should wrap each client operation in an `Op` structure along with a unique identifier. `Submit()` should then wait until the operation has committed and been executed, and return the result of execution (the value returned by `DoOp()`). If `raft.Start()` indicates that the current peer is not the Raft leader, `Submit()` should return an `rpc.ErrWrongLeader` error. `Submit()` should detect and handle the situation in which leadership changed just after it called `raft.Start()`, causing the operation to be lost (never committed).

    For Part A, the `rsm` tester acts as the service, submitting operations that it interprets as increments on a state consisting of a single integer. In Part B you'll use `rsm` as part of a key/value service that implements `StateMachine` (and `DoOp()`), and calls `rsm.Submit()`.

    If all goes well, the sequence of events for a client request is:

    - The client sends a request to the service leader.
    - The service leader calls `rsm.Submit()` with the request.
    - `rsm.Submit()` calls `raft.Start()` with the request, and then waits.
    - Raft commits the request and sends it on all peers' `applyCh`s.
    - The `rsm` reader goroutine on each peer reads the request from the `applyCh` and passes it to the service's `DoOp()`.
    - On the leader, the `rsm` reader goroutine hands the `DoOp()` return value to the `Submit()` goroutine that originally submitted the request, and `Submit()` returns that value.

    Your servers should not directly communicate; they should only interact with each other through Raft.

    - **[Lab4A Code Change](https://github.com/mwfj/6.5840-Distributed-Systems/pull/13)**

  - [x] Part B: Key/value service without snapshots: Now you will use the `rsm` package to replicate a key/value server. Each of the servers ("kvservers") will have an associated rsm/Raft peer. Clerks send `Put()` and `Get()` RPCs to the kvserver whose associated Raft is the leader. The kvserver code submits the Put/Get operation to `rsm`, which replicates it using Raft and invokes your server's `DoOp` at each peer, which should apply the operations to the peer's key/value database; the intent is for the servers to maintain identical replicas of the key/value database.

    A `Clerk` sometimes doesn't know which kvserver is the Raft leader. If the `Clerk` sends an RPC to the wrong kvserver, or if it cannot reach the kvserver, the `Clerk` should re-try by sending to a different kvserver. If the key/value service commits the operation to its Raft log (and hence applies the operation to the key/value state machine), the leader reports the result to the `Clerk` by responding to its RPC. If the operation failed to commit (for example, if the leader was replaced), the server reports an error, and the `Clerk` retries with a different server.

    Your kvservers should not directly communicate; they should only interact with each other through Raft.

    - [Lab 4B code change](https://github.com/mwfj/6.5840-Distributed-Systems/pull/15)

  - [ ] Part C: Key/value service with snapshots:  As things stand now, your key/value server doesn't call your Raft library's `Snapshot()` method, so a rebooting server has to replay the complete persisted Raft log in order to restore its state. Now you'll modify kvserver and `rsm` to cooperate with Raft to save log space and reduce restart time, using Raft's `Snapshot()` from Lab 3D.
  
    The tester passes `maxraftstate` to your `StartKVServer()`, which passes it to `rsm`. `maxraftstate` indicates the maximum allowed size of your persistent Raft state in bytes (including the log, but not including snapshots). You should compare `maxraftstate` to `rf.PersistBytes()`. Whenever your `rsm` detects that the Raft state size is approaching this threshold, it should save a snapshot by calling Raft's `Snapshot`. `rsm` can create this snapshot by calling the `Snapshot` method of the `StateMachine` interface to obtain a snapshot of the kvserver. If `maxraftstate` is -1, you do not have to snapshot. The `maxraftstate` limit applies to the GOB-encoded bytes your Raft passes as the first argument to `persister.Save()`.
  
    You can find the source for the `persister` object in `tester1/persister.go`.
