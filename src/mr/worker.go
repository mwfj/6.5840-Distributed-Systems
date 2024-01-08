package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/rpc"
	"os"
	"sort"
	"strconv"
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

	// Your worker implementation here.
	for {

	}
	// uncomment to send the Example RPC to the coordinator.
	// CallExample()

}

func ProcessMapWorker(mapTask *WorkerDetail, mapf func(string, string) []KeyValue) (filenames []string) {
	filename := mapTask.FileName
	mapFile, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	defer mapFile.Close()
	content, err := io.ReadAll(mapFile)
	if err != nil {
		log.Fatalf("file to read content from file %v", filename)
	}
	// get all key -> value pairs
	kvpairs := mapf(filename, string(content))

	// reduce number => json file
	encoderMap := make(map[int]*json.Encoder)

	// produce intermidiate file for each reduce job
	for i := 0; i < mapTask.NReduce; i++ {
		// filename format intermediate-map_worker_id-reduce_number
		inter_file_name := "intermediate-" + mapTask.Id + "-" + strconv.Itoa(i)
		inter_file, err := os.Create(inter_file_name)
		if err != nil {
			log.Fatalf("create file" + inter_file_name + "failed, reason: " + err.Error())
		}
		filenames = append(filenames, inter_file_name)
		encoderMap[i] = json.NewEncoder(inter_file)
	}

	for _, kv := range kvpairs {
		i := ihash(kv.Key) % mapTask.NReduce
		encoder := encoderMap[i]
		// write kv into the cooresponding file
		err := encoder.Encode(&kv)

		if err != nil {
			log.Fatalf("encode error %s", err.Error())
		}
	}

	return filenames
}

func ProcessReduceWorker(reduceTask *WorkerDetail, reducef func(string, []string) string) (filename string) {
	// parse the intermidiate files
	filenames := reduceTask.FileNames

	intermidiates := []KeyValue{}

	for _, filename := range filenames {
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open file: %s, error: %s", filename, err.Error())
		}
		decoder := json.NewDecoder(file)
		// fetch all key values at that file
		for {
			var kv KeyValue
			if err := decoder.Decode(&kv); err != nil {
				break
			}
			intermidiates = append(intermidiates, kv)
		}

	}

	// create temperate file
	tmpFile, err := os.CreateTemp("", "mr-tmp*")

	if err != nil {
		log.Fatalf("create tmp file: mr-tmp* failed, error: %s", err.Error())
	}
	defer tmpFile.Close()

	sort.Slice(intermidiates, func(i, j int) bool {
		return intermidiates[i].Key < intermidiates[j].Key
	})

	i := 0

	for i < len(intermidiates) {
		j := i + 1
		if j < len(intermidiates) && (intermidiates[i].Key == intermidiates[j].Key) {
			j++
		}
		values := []string{}

		for k := i; k < j; k++ {
			values = append(values, intermidiates[k].Value)
		}

		output := reducef(intermidiates[i].Key, values)
		// write it into the tmpfile
		fmt.Fprintf(tmpFile, "%v %v\n", intermidiates[i].Key, output)
		i = j
	}

	filename = "mr-out-" + reduceTask.Id
	err = os.Rename(tmpFile.Name(), filename)

	if err != nil {
		log.Fatalf("rename file %s failed, error: %s", tmpFile.Name(), err.Error())
	}

	return filename
}

func CallForTask() (*WorkerDetail, bool) {
	args := &Args{}
	reply := &Reply{}
	ok := call("Coordinator.GetTask", args, reply)
	return &reply.Task, ok
}

func CallForFinishedTask(args *Args) bool {
	reply := &Reply{}
	ok := call("Coordinator.NotifyFinishedTask", args, reply)
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
