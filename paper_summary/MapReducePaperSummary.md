# [MapReduce](http://nil.csail.mit.edu/6.824/2022/papers/mapreduce.pdf)

## Brief Summary

In this paper, Google introduced a new programming model - MapReduce, which providing an efficient way(regard as scalability, fault tolerance, performance) to do the data processing in the large-scale computation.

In MapReduce programming model, it mainly divided by two steps:

1. Map: process a key/value pair to generate a set of intermediate key/value pairs
2. Reduce: merges all intermediate values associated with the same intermediate key.



## What is the addressed problem and why is it important to address?

### Importance of Addressing the Problem:

1. **Big Data Era**: The amount of data generated and collected has been growing exponentially. Efficient processing of this data is crucial for deriving insights, making decisions, and driving innovations.
2. **System stability:** In big data era, to prevent data lost, a reliability and availability programming model is important.
3. **Scalability:** In the large scale data processing system, how to use thousand of machine working together is important. What's more, when the current machine scale cannot handle the data amount, extending extra machine into the current system smoothly is important as well.
4. **Cost Efficiency**: Efficient distributed processing can significantly reduce the costs associated with data storage and processing.

### The problem need to address

1. Provide an solution both make programmer easy to handle.
2. Provide a programming model that processing large amount data with good scalability and fault tolerant
3. Most of the parallel processing systems have only been implemented on smaller scaler and leave the details of handing machine failure to the programmer.
4. How to make sure the task completed when it encountered the repeatedly failure.



## What has been done? what do previous studies fail to do and why?

MapReduce can be considered a simplification and distillation compared with the some of the previous study. Most significantly, they are providing a fault-tolerant implementation that scales to thousand of processors.

In contrast, most of the parallel processing systems have only been implementation on smaller scales and leave the details machine failure to the programmer.

MapReduce framework is able to partition the problem into a large number of fine-grained tasks. These tasks dynamically scheduled on available workers so that faster workers process more tasks. The restricted programming model also allows us to schedule redundant execution of tasks near the end of job which greatly reduces completion time in the presence of non-uniformities(such as slow or stuck workers).



## What is novelty or main contribution of this work?

The main contribution of this work are a simple and powerful interfaces that enables automatic  parallelization and distribution of large-scale computations, combined with an implementation of this interface that achieves high performance on large clusters of commodity PCs.



## What is the technical method and approach of this work?

### Map Reduce Programming Model

The main technical approach introduced in this paper is **Map Reduce Programming Model**, where the computation takes a set of *input key/value pairs*, and produces a set of *output key/value* pairs.

- Map, written by the user, takes an input pair and produces a set of *intermediate* key/value pairs. The MapReduce library group together all intermediate value associate with the same intermediate key and passes them to the Reduce function.
- Reduce, also written by the user, accepts an intermediate key and a set of values for that key. It merge together these values to form a possibly smaller set of values. Typically just zero or one output value is produced per Reduce function. The intermediate value are supplied to the user's reduce function via an iterator. This allow us to handle lists of values that are too large fit in memory.

Specifically:

1. The MapReduce library in the user program first splits the input files into M pieces of typically 16 megabytes to 64 megabytes(MB).
   It then starts up many copies of the program on a cluster of machines.
   - One of copies programs is special - the master
   - The rest are workers that assigned work by master
   - There are ***M*** map worker and ***R*** reduce worker assigned

2. A worker who is assigned a map task reads the contents of the corresponding input split.
   It parses key/value pairs out of the input data and passes each pair to the user-defined ***Map*** function are buffered in memory

3. Periodically, the buffered pair are written to local disk, partition into R regions by the partitioning function;

   The location of these buffered pairs on the local disk are passed back to master.

4. When a reduce worker is notified by the master about these location, it uses remote procedure calls to read the buffered data from the local disks of the map workers.
   When a reduce worker has read all intermediate keys so that all occurrences of the same keys are grouped together.

5. The reduce worker iterates over the sorted intermediate data and for each unique intermediate key encountered, it passes the key the corresponding set of intermediate values to the user's ***Reduce*** function
   **The output of the Reduce function is appended to a final output file for this reduce partition.**

6. When all of map/reduce tasks finished, the master wake up the user program, and returns back to the user code.

7. After successful completion, the output file of the mapreduce execution is available in the R output files(one per reduce task)

**When map task completes**, the worker sends a message to the master and indicates the names of the R temporary files in the messages. 

- If master receive a completion map task message for an already completed map task, it ignore the message
- Otherwise, it records the name of R files in a master data structure

**When a reduce task completes**, the reduce worker atomically renames its temporary output file to the final output file.

- If the same  reduce task is executes on multiple machines, multiple rename calls will be executed for the same final output file

![Execution Overview](/home/mwfj/Documents/6.5840-Distributed-Systems/paper_summary/pics/mapreduce_folwchart.png)

### Fault Tolerance

#### Worker Failure

For each map task and reduce task, it stores the state(***idle***, ***in-process***, ***completed***). 
For each completed map task, the master stores the locations and sizes of the R intermediate file regions produced by the map task. Update to this location and size information are received as map task are completed.

- The master pings every worker periodically. If no response is received from a worker in a certain amount of time, the master mark the worker as failed.
- Any map tasks completed by the worker are reset back to their initial **idle** state;
- Any map task or reduce task ***in-progress*** on a failed worker is also reset to ***idle*** and become eligible for rescheduling.
- **Completed map tasks** are re-executed on a failure because their output is stored on the local disk(s) of the failed machine and is therefore inaccessible.
  **Completed reduce tasks** do not need to be re-executed since their output is stored in a global file system
- When a maps task is executed first by the worker A and then later executed by worker B(because A failed), all workers executing reduce tasks are notified of the re-execution.
  Any reduce task that has not already read the data from worker A will read the data from worker B.
- If a machine is unreachable, the MapReduce master simply re-executed the work done by the unreachable worker machine, and continued to make forward progress, eventually completing the MapReduce operation.

####  Master Failure

master write periodic checkpoints of the master data structures. If the master task dies, a new copy can be started from the last checkpointed state.

#### Slow workers/Tail Latency

To solve straggler worker issue, when a MapReduce operation is close to completion, the master schedules **backup executions** of the remaining ***in-process*** tasks. The task is marked as completed whenever either the primary or the backup execution completes.

#### Partition Function

A default partitioning function is provided that uses **hashing**

The user can provider their own specific partition function.

#### Combiner Function

MapReduce API allow the user to specify an optional **Combiner** function that does partial merging of data before sent over the network.

The combiner function is executed on each machine that perform a map task. Typically the same code is used to implement both the combiner and the reduce functions. The only difference is how the MapReduce library handles the output of the function.

- The output of reduce function is written to the final output file
- The output of a combiner function is written to an intermediate file that will be sent to a reduce task

#### Skipping Bad Records

Each worker process installs a signal handler that catches segmentation violations and bus errors. Before invoking a user Map or Reduce operation, the MapReduce library stores the sequence number of the argument in a global variable.

If the user code generates a signal, the signal handler sends a "last gasp" UDP packet that contains the sequence number the to MapReduce master.

**When the master has seen more than one of failure on a particular record, it indicates that the record should be skipped** when it is issue the next re-execution of the corresponding Map or Reduce task.



## How is the work evaluated? Are the evaluation method concrete? Does the evaluation cover all the aspects of consideration? Do the evaluation results support the claims?

#### How is the work evaluated?

The MapReduce has been evaluated in the following perspectives:

##### 1. Performance

The paper uses some common applications to test the performance in MapReduce system, such as **Grep**/**Sort**, 10^10 100-byte records,

##### 2. Fault Tolerance

The paper has test some cases that the worker job encountered effect of **Backup Tasks** and **Machines Failures**

##### 3. Scalability

The scalability of MapReduce was tested by running jobs on clusters of different sizes, ranging from a few nodes to thousands of nodes.

##### 4. Cases Studies

In this paper, MapReduce has been test on differenece internal Google Services, including:

- Large-scale machine learning problems
- Cluster problems for the Google News and Froogle products
- Extracting of data used to produce reports of popular queries(***e.g.*** Google Zeitgeist)
- Extraction of properities of web pages for new experiments and products
- Large-scale graph computations

#### Concreteness of the Evaluation Methods:

The evaluation methods used in the paper are concrete and well-defined. The authors provided clear descriptions of the experimental setup, including the hardware configuration, the datasets used, and the parameters varied in the experiments. This allows for reproducibility and provides a solid foundation for the evaluation.

#### The aspects of consideration

In my perspective, the test in this paper has been covered most of the claim they described. However, some aspects like **Security/Energy Saving/Load Balancing** have not been discussed in here and some experimental detail is not fully described.



## What are impacts of this work?

The MapReduce paper has a huge impact on the area of distribution system, where it has become the must read list for most of distribution engineer/researcher.

Right now, the MapReduce model has become a fundamental concept in the field of big data, inspiring a vast body of research on distributed computing, data processing, and parallel algorithms. The work has stimulated research into optimizing various aspects of distributed computing, such as fault tolerance, load balancing, data locality, and resource management.
The citation number for this paper has been more  than 10,000.

In the industry level, Hadoop Ecosystem/Apache Spark and some other cloud service application has been inspired on this paper.
