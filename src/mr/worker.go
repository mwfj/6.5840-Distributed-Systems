package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"sort"
	"strconv"
	"time"
)

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	// log.Println("[woker]: worker已被创建")
	// Your worker implementation here.
	//不断请求任务
	for {
		task, ok := CallForGetTask()
		if !ok {

			return
		}

		if task.Type == Map {

			filenames := MapWork(task, mapf)

			//通知coordinator
			args := &Args{Task: *task, IntermidiateFiles: filenames}
			ok := CallForFinishedTask(args)
			if !ok {

				return
			}
		} else if task.Type == Reduce {

			filename := ReduceWork(task, reducef)

			args := &Args{Task: *task, OutPutFileName: filename}
			ok := CallForFinishedTask(args)
			if !ok {

				return
			}
		} else {
			time.Sleep(1 * time.Second)
			continue
		}
	}
	// uncomment to send the Example RPC to the coordinator.
	// CallExample()
}

// 处理map任务
func MapWork(task *WorkerDetail, mapFunc func(string, string) []KeyValue) (filenames []string) {

	filename := task.FileName
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", filename)
	}
	file.Close()

	kva := mapFunc(filename, string(content))

	//创建存取每个文件的encoder
	encodersMap := make(map[int]*json.Encoder)
	for i := 0; i < task.NReduce; i++ {

		name := "mr-" + strconv.Itoa(task.Id) + "-" + strconv.Itoa(i)
		file, err := os.Create(name)
		//将文件名加入到产生的文件列表
		filenames = append(filenames, name)

		if err != nil {
			log.Fatalf("create file: %s error: %s", file.Name(), err.Error())
		}
		encodersMap[i] = json.NewEncoder(file)
	}

	for _, kv := range kva {

		i := ihash(kv.Key) % task.NReduce

		encoder := encodersMap[i]

		err = encoder.Encode(&kv)
		if err != nil {
			log.Fatalf("encode error: %s", err.Error())
		}
	}
	return filenames
}

// 处理reduce任务
func ReduceWork(task *WorkerDetail, reduceFunc func(string, []string) string) (filename string) {

	filenames := task.FileNames

	intermediates := []KeyValue{}
	for _, filename := range filenames {
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("open file: %s error: %s", filename, err.Error())
		}
		decoder := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := decoder.Decode(&kv); err != nil {
				break
			}
			intermediates = append(intermediates, kv)
		}

	}

	tempFile, err := ioutil.TempFile("", "mr-temp*")
	if err != nil {
		log.Fatalf("crete temp file error: %s", err.Error())
	}
	defer tempFile.Close()

	// sort.Sort(ByKey(intermediates))
	sort.Slice(intermediates, func(i, j int) bool {
		return intermediates[i].Key < intermediates[j].Key
	})

	i := 0
	for i < len(intermediates) {
		j := i + 1
		for j < len(intermediates) && intermediates[j].Key == intermediates[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediates[k].Value)
		}
		output := reduceFunc(intermediates[i].Key, values)
		// this is the correct format for each line of Reduce output.
		fmt.Fprintf(tempFile, "%v %v\n", intermediates[i].Key, output)
		i = j
	}

	filename = "mr-out-" + strconv.Itoa(task.Id)
	err = os.Rename(tempFile.Name(), filename)
	if err != nil {
		log.Fatalf("rename temp file error: %s", err.Error())
	}
	return filename
}

func CallForGetTask() (*WorkerDetail, bool) {
	args := &Args{}
	reply := &Reply{}
	ok := call("Coordinator.GetTask", args, reply)
	return &reply.Task, ok
}

func CallForFinishedTask(args *Args) bool {
	reply := &Reply{}
	ok := call("Coordinator.FinishedTask", args, reply)
	return ok
}

// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}
	// fill in the argument(s).
	args.X = 99
	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
