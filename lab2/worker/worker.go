package pworker

import (
	"time"

	"lab2/shared"
)

type WorkerStatus string

const (
	IDLE     WorkerStatus = "IDLE"
	CRACKING WorkerStatus = "CRACKING"
	DONE     WorkerStatus = "DONE"
	DEAD     WorkerStatus = "DEAD"
)

type WorkerStatusResponse struct {
	Status WorkerStatus `json:"status"`
}

/* This interface is used only for worker-side */
type WorkerI interface {
}

/* This struct is used only for coordinator-side */
type Worker struct {
	Id      shared.WorkerId `json:"-"`
	Address string          `json:"address"`
	Status  WorkerStatus    `json:"status"`
	LastHB  time.Time       `json:"-"`
}
