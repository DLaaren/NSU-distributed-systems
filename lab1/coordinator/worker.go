package coordinator

import (
	"sync"
	"time"
)

type WorkerStatus int

const (
	IDLE WorkerStatus = iota
	CRACKING
	DONE
	DEAD
)

type Worker struct {
	Address string       `json:"address"`
	Status  WorkerStatus `json:"status"`
	LastHB  time.Time    `json:"lastHb"`
	rwmu    sync.RWMutex
}

type WorkerStatusResponse struct {
	Status WorkerStatus `json:"status"`
}
