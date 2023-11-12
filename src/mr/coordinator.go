package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
)

/**
 * 1.  parition map input data into a set of M splits, where M is the number of map workers
 * 2.  start up the M map workers(one of them is coordinator)
 * 3.  coordinator picks idle workers to do the map job
 * 4.  the map worker do:
 *              a. read the split input
 *              b. parse key/valye pairs out of the input data
 *              c. passes each pair to the user-defined Map function
 *              d. produce the intermidate key/value pairs
 *              e. buffered in memory
 * 5.  periodly the buffered pair are written to the local disk
 * 6.  map worker partition the intermidate keys in to N splits,
 *        where N is the number of reducer workers(specify by users).
 *     And send it to the coordinate
 * 7.  coordiantor notify reducer worker via RPC
 * 8.  when a reducer worker get notified, it user RPC to read the buffer data
 * 9.  the reducer worker sort the buffer data by the intermidiate keys
 *     so that all the occurrence of the same key are grouped together
 * 10. the reducer worker iterate over the sorted intermidiate data,
 *     for each itermidiate key, reducer worker pass the key and cooresponding value to the Reduce function
 * 11. the output of the Reduce function is appended to a final output file for this reduce partition
 * 12. when all map and reduce function completed, coordinator wake the user program up.
 * 13. After successful completion, the output of the mapreduce execution is available in the R output files
 *     (one reduce task per file)
 *     Typically, users do not need to combine these R output files into one file:
 *          - they often pass these files as input to another mapreduce call,
 *          - or use them from another distributed application that is able to deal with the input
 *            that is partitioned into multiple files
 */

/**
 * What coordinator need to do?
 * 1.  parition map input data into a set of M splits, where M is the number of map workers
 * 2.  start up the M map workers(one of them is coordinator)
 * 3.  coordinatro picks idle workers to do the map job
 * 4.  coordiante notify reducer worker via RPC
 * 5.  when all map and reduce function completed, coordinator wake the user program up.
 */

type Coordinator struct {
	// Your definitions here.

}

// Your code here -- RPC handlers for the worker to call.

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	ret := false

	// Your code here.

	return ret
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}

	// Your code here.

	c.server()
	return &c
}
