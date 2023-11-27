package mr

type WorkerStatus int
type WorkerType int
type IntermidiateKeyMapping map[string]string
type uuid_type string

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
	Id         string
	Status     WorkerStatus // the current worker status
	Type       WorkerType
	FileName   string
	FileNames  []string
	NReduce    int
	IsAlive    bool
	IsOccupied bool
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

func (wd *WorkerDetail) IsAvailable() bool {
	return !wd.IsOccupied
}
