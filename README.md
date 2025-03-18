# MIT 6.5840 Distributed Systems

### updating ... ...

- [x] **[Lab1: Map Reduce](https://pdos.csail.mit.edu/6.824/labs/lab-mr.html)** : In this lab you'll build a MapReduce system. You'll implement a worker process that calls application Map and Reduce functions and handles reading and writing files, and a coordinator process that hands out tasks to workers and copes with failed workers. You'll be building something similar to the [MapReduce paper](http://research.google.com/archive/mapreduce-osdi04.pdf). (Note: this lab uses "coordinator" instead of the paper's "master".)
- [x] **[Lab 2: Key/Value Server](https://pdos.csail.mit.edu/6.824/labs/lab-mr.html)** :In this lab you will build a key/value server for a single machine that ensures that each operation is executed exactly once despite network failures and that the operations are [linearizable](https://pdos.csail.mit.edu/6.824/papers/linearizability-faq.txt). Later labs will replicate a server like this one to handle server crashes.


- [X] **[Lab 2: Key/Value Server](https://pdos.csail.mit.edu/6.824/labs/lab-mr.html)** :In this lab you will build a key/value server for a single machine that ensures that each operation is executed exactly once despite network failures and that the operations are [linearizable](https://pdos.csail.mit.edu/6.824/papers/linearizability-faq.txt). Later labs will replicate a server like this one to handle server crashes.

- [ ] **[Lab 3: Key/Value Server](https://pdos.csail.mit.edu/6.824/labs/lab-mr.html)** :This is the first in a series of labs in which you'll build a fault-tolerant key/value storage system. In this lab you'll implement Raft, a replicated state machine protocol. In the next lab you'll build a key/value service on top of Raft. Then you will “shard” your service over multiple replicated state machines for higher performance.

  In this lab you'll implement Raft as a Go object type with associated methods, meant to be used as a module in a larger service. A set of Raft instances talk to each other with RPC to maintain replicated logs. Your Raft interface will support an indefinite sequence of numbered commands, also called log entries. The entries are numbered with *index numbers*. The log entry with a given index will eventually be committed. At that point, your Raft should send the log entry to the larger service for it to execute.

  You should follow the design in the [extended Raft paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf), with particular attention to Figure 2. You'll implement most of what's in the paper, including saving persistent state and reading it after a node fails and then restarts. You will not implement cluster membership changes (Section 6).

  - [ ] Part A - Leader Election: 
    Implement Raft leader election and heartbeats (`AppendEntries` RPCs with no log entries). The goal for Part 3A is for a single leader to be elected, for the leader to remain the leader if there are no failures, and for a new leader to take over if the old leader fails or if packets to/from the old leader are lost. Run `go test -run 3A `to test your 3A code.
  - [ ] Part B - Log
    Implement the leader and follower code to append new log entries, so that the `go test -run 3B `tests pass.
  - [ ] Part C - Persistence
    Complete the functions `persist()` and `readPersist()` in `raft.go` by adding code to save and restore persistent state. You will need to encode (or "serialize") the state as an array of bytes in order to pass it to the `Persister`. Use the `labgob` encoder; see the comments in `persist()` and `readPersist()`. `labgob` is like Go's `gob` encoder but prints error messages if you try to encode structures with lower-case field names. For now, pass `nil` as the second argument to `persister.Save()`. Insert calls to `persist()` at the points where your implementation changes persistent state. Once you've done this, and if the rest of your implementation is correct, you should pass all of the 3C tests.