# Lab 3 Raft

## 3A: leader election

Code change please see [here](https://github.com/mwfj/6.5840-Distributed-Systems/pull/6/files)

![leader-election-flowchart](./pics/raft-3a-leader-election.jpeg)



## 3B: Log Implement

Code Change please see:

1. [Main code change](https://github.com/mwfj/6.5840-Distributed-Systems/pull/7/files)
2. [Make code more robust](https://github.com/mwfj/6.5840-Distributed-Systems/pull/8/files)
3. [Optimize with more efficient algorithm](https://github.com/mwfj/6.5840-Distributed-Systems/pull/9)

In this part the mainly change is to add two important components:

- ***Applier***: each server use is to save new log created entries into local
- ***Replicator*：** Leader use replicator to trigger that sending new created log entries to followers via `AppendEntry RPC`
  - From the leader perspective, each follower has its own replicator

**Leader trigger the *replicator* for each follower to send the new create log entry to receiver.**

**Follower receive the new create log entries from Leader and save it into local triggering this process by *applier*.** 

![lab3b-log](https://raw.githubusercontent.com/mwfj/6.5840-Distributed-Systems/78dd51306107b737cc1eda260240910ec0852ed0/src/raft/pics/6-5840-raft-lab-3b.svg)



## Test Result

### Lab3A

```shell
go test --race -run 3A
Test (3A): initial election ...
  ... Passed --   3.0  3  116   34628    0
Test (3A): election after network failure ...
  ... Passed --   4.4  3  238   51401    0
Test (3A): multiple elections ...
  ... Passed --   5.5  7 1104  238375    0
PASS
ok      6.5840/raft     13.887s
```

### Lab3B

```shell
go test --race -run 3B
Test (3B): basic agreement ...
  ... Passed --   0.3  3   16    4694    3
Test (3B): RPC byte count ...
  ... Passed --   0.7  3   48  115042   11
Test (3B): test progressive failure of followers ...
  ... Passed --   4.3  3  218   49730    3
Test (3B): test failure of leaders ...
  ... Passed --   4.4  3  341   81580    3
Test (3B): agreement after follower reconnects ...
  ... Passed --   3.3  3  142   39636    7
Test (3B): no agreement if too many followers disconnect ...
  ... Passed --   3.2  5  374   82001    3
Test (3B): concurrent Start()s ...
  ... Passed --   0.6  3   19    5633    6
Test (3B): rejoin of partitioned leader ...
  ... Passed --   3.6  3  254   65623    4
Test (3B): leader backs up quickly over incorrect follower logs ...
  ... Passed --   9.7  5 1796 1625437  102
Test (3B): RPC counts aren't too high ...
  ... Passed --   2.1  3   80   24622   12
PASS
ok      6.5840/raft     33.218s
```

