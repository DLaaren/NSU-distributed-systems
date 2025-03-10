package worker

import (
	"log"
)

type WorkerStatus int

const (
	IDLE WorkerStatus = iota
	PROCESSING
	DONE
	DEAD
)

// type WorkerTask struct {
// 	id int
// }

type Worker struct {
	Address string       `json:"address"`
	Status  WorkerStatus `json:"status"`
}

/* Init Worker instance */
func NewWorker(address string) *Worker {
	log.SetPrefix("[Worker]: ")
	log.Println("worker was created")
	log.SetPrefix("[Server]: ")
	return &Worker{
		Address: address,
		Status:  IDLE,
	}
}
