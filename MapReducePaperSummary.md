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

The main technical approach introduced in this paper is **Map Reduce Programming Model**, where the computation takes a set of *input key/value pairs*, and produces a set of *output key/value* pairs.

- Map, written by the user, takes an input pair and produces a set of *intermediate* key/value pairs. The MapReduce library group together all intermediate value associate with the same intermediate key and passes them to the Reduce function.
- Reduce, also written by the user, accepts an intermediate key and a set of values for that key. It merge together these values to form a possibly smaller set of values. Typically just zero or one output value is produced per Reduce function. The intermediate value are supplied to the user's reduce function via an iterator. This allow us to handle lists of values that are too large fit in memory.

Specifically:

1. The MapReduce library in the user program first splits the input files into M pieces of typically 16 megabytes to 64 megabytes(MB).
   It then starts up many copies of the program on a cluster of machines.
   - One of copies programs is special - the master
   - The rest are workers that assigned work by master
   - There are ***M*** map worker and ***R*** reduce worker assigned

![Execution Overview](/home/mwfj/Documents/6.5840-Distributed-Systems/pics/mapreduce_folwchart.png)



## How is the work evaluated? Are the evaluation method concrete? Does the evaluation cover all the aspects of consideration? Do the evaluation results support the claims?



## What are impacts of this work?

