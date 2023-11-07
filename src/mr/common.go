package mr

type WorkerStatus int
type WorkerType int
type IntermidiateKeyMapping map[string]string

// Worker states enmu
const (
	Idle WorkerStatus = iota
	InProcess
	Completed
	Error
)

const (
	Map WorkerType = iota
	Reduce
)

type WorkerDetail struct {
	// task id
	Id string
	// the current worker status
	Status  WorkerStatus
	Type    WorkerType
	NReduce int
	IsAlive bool
}
