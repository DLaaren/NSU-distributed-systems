package shared

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
