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
	Id        int
	Status    WorkerStatus // the current worker status
	Type      WorkerType
	FileName  string
	FileNames []string
	NReduce   int
	IsAlive   bool
}
