package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
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
 * 7.  coordiantor notify reducer worker via RPC call
 * 8.  when a reducer worker get notified, it use RPC to read the buffer data
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
	mapsTaskList           map[int]*WorkerDetail // map	 worker uuid => worker detail
	reduceTaskList         map[int]*WorkerDetail // reduce worker uuid => worker detail
	reduceChan             chan *WorkerDetail
	mapChan                chan *WorkerDetail
	isAllMapWorkerFinished bool
	mu                     sync.Mutex
	interFiles             map[int][]string
}

// Your code here -- RPC handlers for the worker to call.
/**
 * In effect, the method must look schematically like
 * 		func (t *T) MethodName(argType T1, replyType *T2) error
 */
func (c *Coordinator) GetTask(args *Args, reply *Reply) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isAllMapWorkerFinished {

		if len(c.mapChan) != 0 {

			task := <-c.mapChan

			task.ChangeStatusToInProcess()

			reply.Task = *task

			go c.CheckFinished(task.Id, task.Type)
		}
		return nil
	}

	if len(c.reduceChan) != 0 {

		task := <-c.reduceChan

		task.ChangeStatusToInProcess()
		//将中间文件放入到task中
		task.FileNames = c.interFiles[task.Id]
		//放置返回值中
		reply.Task = *task

		go c.CheckFinished(task.Id, task.Type)
	}
	return nil
}

func (c *Coordinator) FinishedTask(args *Args, reply *Reply) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	taskArg := args.Task
	var task *WorkerDetail

	if taskArg.Type == Map {
		task = c.mapsTaskList[taskArg.Id]
	} else {
		task = c.reduceTaskList[taskArg.Id]
	}

	if task.IsInProcess() {

		task.ChangeStatusToCompleted()

		if task.Type == Map {
			filenames := args.IntermidiateFiles

			for _, filename := range filenames {
				split := strings.Split(filename, "-")
				reduceId, _ := strconv.Atoi(split[2])
				c.interFiles[reduceId] = append(c.interFiles[reduceId], filename)
			}

			return nil
		}

	}

	return nil
}

// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	go func() {
		for {
			log.Println("go routine running...")
			time.Sleep(1000)
		}
	}()
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

func (c *Coordinator) CheckFinished(taskId int, taskType WorkerType) {

	time.Sleep(10 * time.Second)

	c.mu.Lock()
	defer c.mu.Unlock()

	if taskType == Map {
		t := c.mapsTaskList[taskId]
		if t.IsCompleted() {

			return
		}

		t.ChangeStatusToIdle()
		c.mapChan <- t

	} else {
		t := c.reduceTaskList[taskId]
		if t.IsCompleted() {

			return
		}

		t.ChangeStatusToIdle()
		c.reduceChan <- t

	}
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {

	c.mu.Lock()
	defer c.mu.Unlock()
	mapAllFinished := true
	reduceAllFinished := true
	// Your code here.

	for _, task := range c.mapsTaskList {
		if !task.IsCompleted() {
			mapAllFinished = false
		}
	}

	c.isAllMapWorkerFinished = mapAllFinished

	if !mapAllFinished {
		return false
	}

	for _, task := range c.reduceTaskList {
		if !task.IsCompleted() {

			reduceAllFinished = false
		}
	}

	return reduceAllFinished
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// NReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {

	c := Coordinator{
		mapsTaskList:           make(map[int]*WorkerDetail),
		reduceTaskList:         make(map[int]*WorkerDetail),
		mapChan:                make(chan *WorkerDetail, 10000),
		reduceChan:             make(chan *WorkerDetail, nReduce),
		isAllMapWorkerFinished: false,
		interFiles:             make(map[int][]string),
	}

	// Your code here.

	for i, file := range files {

		t := &WorkerDetail{
			Id:       i,
			Type:     Map,
			FileName: file,
			Status:   Idle,
			NReduce:  nReduce,
		}

		c.mapsTaskList[i] = t

		c.mapChan <- t
	}

	for i := 0; i < nReduce; i++ {

		t := &WorkerDetail{
			Id:        i,
			Type:      Reduce,
			Status:    Idle,
			FileNames: make([]string, 0),
			NReduce:   nReduce,
		}

		c.reduceTaskList[i] = t

		c.reduceChan <- t
	}

	c.server()
	return &c
}
