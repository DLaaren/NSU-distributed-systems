package worker

import (
	"lab1/shared"
	"sync"
)

type WorkerContext struct {
	Status shared.WorkerStatus
	Tasks  map[shared.TaskId]*shared.WorkerTask

	rwmu sync.RWMutex
}
