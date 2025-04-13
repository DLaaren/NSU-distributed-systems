package pworker

import (
	"time"

	"lab2/shared"
)

type WorkerStatus string

const (
	ALIVE WorkerStatus = "ALIVE"
	DEAD  WorkerStatus = "DEAD"
)

type WorkerStatusResponse struct {
	Port   string       `json:"port"`
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
