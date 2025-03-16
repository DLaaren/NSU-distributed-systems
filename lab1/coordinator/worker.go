package coordinator

import (
	"lab1/shared"
	"time"
)

type Worker struct {
	Id      shared.WorkerId
	Address string              `json:"address"`
	Status  shared.WorkerStatus `json:"status"`
	LastHB  time.Time           `json:"lastHb"`
}
