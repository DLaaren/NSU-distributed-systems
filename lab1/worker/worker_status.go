package worker

type WorkerStatus int

const (
	IDLE WorkerStatus = iota
	PROCESSING
	DONE
	DEAD
)
