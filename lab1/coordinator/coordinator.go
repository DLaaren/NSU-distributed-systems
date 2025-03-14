package coordinator

import (
	"bytes"
	"container/list"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"lab1/shared"

	"github.com/google/uuid"
)

type Coordinator struct {
	UserRequests map[shared.Id]*UserRequest
	Workers      map[string]*Worker
	WorkersTasks map[shared.Id]*shared.WorkerTask

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

/* Trearing each string as a base-26 number */
func stringToInt(s string) int64 {
	var result int64
	for _, r := range s {
		result = result*26 + int64(r-'a')
	}
	return result
}

func intToString(n int64, length int) string {
	var result string
	for i := 0; i < length; i++ {
		result = string('a'+n%26) + result
		n /= 26
	}
	return result
}

func splitRange(start, end string, numWorkers int) []string {
	startVal := stringToInt(start)
	endVal := stringToInt(end)
	chunkSize := (endVal - startVal) / int64(numWorkers)

	var chunks []string
	for i := 0; i < numWorkers; i++ {
		chunkStart := intToString(startVal+int64(i)*chunkSize, len(start))
		chunkEnd := intToString(startVal+int64(i+1)*chunkSize-1, len(start))
		if i == numWorkers-1 {
			/* Ensure the last chunk includes the end value */
			chunkEnd = end
		}
		chunks = append(chunks, fmt.Sprintf("%s-%s", chunkStart, chunkEnd))
	}

	return chunks
}

/* Creates task and map it to workers */
func (c *Coordinator) Crack(request *UserRequest) shared.Id {
	request.Id = shared.Id(uuid.New().ID())
	request.Status = PROCESSING

	c.rwmu.Lock()
	c.UserRequests[request.Id] = request
	c.rwmu.Unlock()

	go func(request *UserRequest) {
		task := shared.WorkerTask{
			Id:        shared.Id(uuid.New().ID()),
			RequestId: request.Id,
			Hash:      request.Hash,
			MaxLength: request.MaxLength,
			Status:    shared.IN_PROGRESS,
		}

		c.rwmu.RLock()
		num_workers := len(c.Workers)
		c.rwmu.RUnlock()

		crack_len := request.MaxLength
		chunks := splitRange(strings.Repeat("a", int(crack_len)), strings.Repeat("z", int(crack_len)), num_workers)

		c.rwmu.RLock()
		defer c.rwmu.RUnlock()
		i := 0
		for {
			for _, worker := range c.Workers {
				task.InputRange = chunks[i]
				res, err := c.TaskLaunch(worker, &task)
				if res == true && err == nil {
					i++
				}
				if err != nil {
					request.Status = ERROR
					break
				}
				if i == len(chunks) {
					break
				}
			}
		}
	}(request)

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

func (c *Coordinator) TaskLaunch(worker *Worker, task *shared.WorkerTask) (bool, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(task); err != nil {
		return false, err
	}

	// Send the HTTP POST request
	resp, err := http.Post(
		"http://"+worker.Address+"/internal/api/worker/crack?id=%"+string(task.Id),
		"application/json",
		&buf)
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	c.WorkersTasks[task.Id] = task

	return true, nil
}

func (c *Coordinator) TaskStatus() {

}

func (c *Coordinator) TaskKill() {

}
