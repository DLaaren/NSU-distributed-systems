package coordinator

import (
	"lab1/shared"
	"time"
)

type WorkerStatus string

const (
	IDLE     WorkerStatus = "IDLE"
	CRACKING WorkerStatus = "CRACKING"
	DONE     WorkerStatus = "DONE"
	DEAD     WorkerStatus = "DEAD"
)

type Worker struct {
	Id      shared.WorkerId
	Address string       `json:"address"`
	Status  WorkerStatus `json:"status"`
	LastHB  time.Time    `json:"lastHb"`
}

type WorkerStatusResponse struct {
	Status WorkerStatus `json:"status"`
}
