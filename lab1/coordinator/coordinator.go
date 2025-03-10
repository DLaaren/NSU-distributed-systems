package coordinator

import (
	"encoding/json"
	"lab1/shared"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Coordinator struct {
	UserRequests map[shared.Id]*UserRequest
	Workers      map[string]*Worker

	rwmu sync.RWMutex
}

/* Init Coordinator instance */
func NewCoordinator() *Coordinator {
	log.SetPrefix("[Coordintor]: ")
	log.Println("coordinator was created")
	log.SetPrefix("[Server]: ")
	return &Coordinator{
		UserRequests: make(map[shared.Id]*UserRequest, 0),
		Workers:      make(map[string]*Worker, 0),
	}
}

/* Send heartbeat to workers
 * If worker is DEAD for a long time then delete it
 */
func (c *Coordinator) CheckWorkers() {
	heartbeatDelay := 5 * time.Second
	ticker := time.NewTicker(heartbeatDelay)
	deadDelay := 1 * time.Minute

	for range ticker.C {
		for address, w := range c.Workers {
			response, err := http.Get("http://" + w.Address + "/internal/api/worker/heartbeat")
			if err != nil || response.StatusCode != http.StatusOK {
				if w.Status == DEAD && time.Now().Sub(w.LastHB) >= deadDelay {
					c.rwmu.Lock()
					delete(c.Workers, address)
					c.rwmu.Unlock()
					log.SetPrefix("[Coordintor]: ")
					log.Println("worker with address", address, "was deleted")
					log.SetPrefix("[Server]: ")
				} else {
					w.rwmu.Lock()
					w.Status = DEAD
					w.rwmu.Unlock()
				}
			} else if err == nil && response.StatusCode == http.StatusOK {
				w.rwmu.Lock()
				w.LastHB = time.Now()
				w.Status = c.getWorkerStatus(address)
				w.rwmu.Unlock()
			}
		}
	}
}

/* Register new worker */
func (c *Coordinator) RegisterWorker(worker *Worker) {
	c.rwmu.Lock()
	defer c.rwmu.Unlock()

	c.Workers[worker.Address] = worker
}

/* Get worker status */
func (c *Coordinator) getWorkerStatus(address string) WorkerStatus {
	resp, err := http.Get("http://" + address + "/internal/api/worker/status")
	if err != nil || resp.StatusCode != http.StatusOK {
		return DEAD
	}
	defer resp.Body.Close()

	var statusResponse WorkerStatusResponse

	if err := json.NewDecoder(resp.Body).Decode(&statusResponse); err != nil {
		return DEAD
	}

	return statusResponse.Status
}

/* Creates task and map it to workers */
func (c *Coordinator) Crack(request *UserRequest) shared.Id {
	request.Id = shared.Id(uuid.New().ID())
	request.Status = PROCESSING

	c.rwmu.Lock()
	defer c.rwmu.Unlock()

	c.UserRequests[request.Id] = request

	go func() {

	}()

	return request.Id
}

func (c *Coordinator) UserRequestStatus(requestId shared.Id) UserStatusResponse {
	c.rwmu.RLock()
	defer c.rwmu.RUnlock()

	userRequest := c.UserRequests[requestId]

	return UserStatusResponse{
		Status: userRequest.Status,
		Result: userRequest.Result,
	}
}

func (c *Coordinator) TaskLaunch() {

}

func (c *Coordinator) TaskStatus() {

}

func (c *Coordinator) TaskKill() {

}

func (c *Coordinator) Map() {

}

func (c *Coordinator) Reduce() {

}
