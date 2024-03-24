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
	None = iota
	Map
	Reduce
)

type WorkerDetail struct {
	Id        int          // task id
	Status    WorkerStatus // the current worker status
	Type      WorkerType
	FileName  string
	FileNames []string
	NReduce   int
}

func (wd *WorkerDetail) IsCompleted() bool {
	return wd.Status == Completed
}

func (wd *WorkerDetail) IsIdle() bool {
	return wd.Status == Idle
}

func (wd *WorkerDetail) IsInProcess() bool {
	return wd.Status == InProcess
}

func (wd *WorkerDetail) ChangeStatusToCompleted() {
	wd.Status = Completed
}

func (wd *WorkerDetail) ChangeStatusToIdle() {
	wd.Status = Idle
}

func (wd *WorkerDetail) ChangeStatusToInProcess() {
	wd.Status = InProcess
}

func (wd *WorkerDetail) IsMapWorker() bool {
	return wd.Type == Map
}

func (wd *WorkerDetail) IsReduceWorker() bool {
	return wd.Type == Reduce
}
