package coordinator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"lab1/shared"

	"github.com/google/uuid"
)

type CoordinatorI interface {
	GetUserRequestStatus(requestId shared.UserRequestId) UserStatusResponse
	Crack(request *UserRequest) shared.UserRequestId
	RegisterWorker(worker *Worker)
	CheckWorkers()
	UpdateTask(task *shared.WorkerTask)
}

type Coordinator struct {
	UserRequests        map[shared.UserRequestId]*UserRequest
	UserRequestsToTasks map[shared.UserRequestId][]*shared.WorkerTask
	Workers             map[shared.WorkerId]*Worker
	AddressesToWorkers  map[string]*Worker
	WorkersTasks        map[shared.TaskId]*shared.WorkerTask

	rwmu sync.RWMutex
}

/* Init Coordinator instance */
func NewCoordinator() *Coordinator {
	log.SetPrefix("[Coordintor]: ")
	log.Println("coordinator was created")
	log.SetPrefix("[Server]: ")
	return &Coordinator{
		UserRequests:        make(map[shared.UserRequestId]*UserRequest, 0),
		UserRequestsToTasks: make(map[shared.UserRequestId][]*shared.WorkerTask, 0),
		Workers:             make(map[shared.WorkerId]*Worker, 0),
		AddressesToWorkers:  make(map[string]*Worker, 0),
		WorkersTasks:        make(map[shared.TaskId]*shared.WorkerTask, 0),
	}
}

func (c *Coordinator) GetUserRequestStatus(requestId shared.UserRequestId) UserStatusResponse {
	c.rwmu.RLock()
	defer c.rwmu.RUnlock()

	userRequest, found := c.UserRequests[requestId]

	var response UserStatusResponse

	if !found {
		response = UserStatusResponse{
			Status: ERROR,
			Result: "",
		}
	} else {
		response = UserStatusResponse{
			Status: userRequest.Status,
			Result: userRequest.Result,
		}
	}

	log.SetPrefix("[Coordintor]: ")
	log.Println("GetUserRequestStatus(" + strconv.FormatUint(uint64(requestId), 10) + ") called; Status = " + string(response.Status) + "; Result = " + response.Result)
	log.SetPrefix("[Server]: ")

	return response
}

/* Trearing each string as a base-26 number */
func stringToInt(s string) int32 {
	var result int32
	for _, r := range s {
		result = result*26 + int32(r-'a')
	}
	return result
}

func intToString(n int32, length int) string {
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
	chunkSize := (endVal - startVal) / int32(numWorkers)

	var chunks []string
	for i := 0; i < numWorkers; i++ {
		chunkStart := intToString(startVal+int32(i)*chunkSize, len(start))
		chunkEnd := intToString(startVal+int32(i+1)*chunkSize-1, len(start))
		if i == numWorkers-1 {
			/* Ensure the last chunk includes the end value */
			chunkEnd = end
		}
		chunks = append(chunks, fmt.Sprintf("%s-%s", chunkStart, chunkEnd))
	}

	return chunks
}

func (c *Coordinator) assignTasks(request UserRequest) {
	task := shared.WorkerTask{
		RequestId: request.Id,
		Hash:      request.Hash,
		MaxLength: request.MaxLength,
		Status:    shared.IN_PROGRESS,
	}

	c.rwmu.Lock()
	numWorkers := len(c.Workers)
	c.UserRequests[request.Id].TasksScheduled = numWorkers
	c.rwmu.Unlock()

	crack_len := request.MaxLength
	chunks := splitRange(strings.Repeat("a", int(crack_len)), strings.Repeat("z", int(crack_len)), numWorkers)

	getNextWorker := func(c *Coordinator, numWorkers *int, i int) *Worker {
		c.rwmu.RLock()
		*numWorkers = len(c.Workers)
		workerIndex := shared.WorkerId(i % *numWorkers)
		worker := c.Workers[workerIndex]
		c.rwmu.RUnlock()

		return worker
	}

	retry := 0
	timeout := 1 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go func() {
		for i, chunk := range chunks {
			task.Id = shared.TaskId(uuid.New().ID())
			task.InputRange = chunk
		retrytaskLaunch:
			select {
			case <-ctx.Done():
				return
			default:
				worker := getNextWorker(c, &numWorkers, i+retry)

				res, err := c.TaskLaunch(worker, &task)
				if !res || err != nil {
					retry++
					goto retrytaskLaunch
				}
			}
		}
	}()

	/* Wait for the context to be done (timeout or cancellation) */
	<-ctx.Done()
	if ctx.Err() == context.DeadlineExceeded {
		c.deleteRequestAndTasks(request)

		log.SetPrefix("[Coordinator]: ")
		log.Println("timeout while trying to assign tasks for user request with id =" + strconv.FormatUint(uint64(request.Id), 10))
		log.SetPrefix("[Server]: ")
	}
}

func (c *Coordinator) deleteRequestAndTasks(request UserRequest) {
	c.rwmu.Lock()
	c.UserRequests[request.Id].Status = TIMEOUT_ERROR
	for _, task := range c.UserRequestsToTasks[request.Id] {
		task.Status = shared.KILLED
	}
	c.rwmu.Unlock()

	c.rwmu.RLock()
	for _, task := range c.UserRequestsToTasks[request.Id] {
		err := c.TaskKill(task)
		if err != nil {
			task.Status = shared.UNKNOWN
		}
	}
	c.rwmu.RUnlock()
}

/* Creates task and map it to workers */
func (c *Coordinator) Crack(request *UserRequest) shared.UserRequestId {
	request.Id = shared.UserRequestId(uuid.New().ID())
	request.Status = PROCESSING
	request.TasksDone = 0
	request.TasksScheduled = 0

	c.rwmu.Lock()
	c.UserRequests[request.Id] = request
	c.rwmu.Unlock()

	go c.assignTasks(*request)

	log.SetPrefix("[Coordintor]: ")
	log.Println("successfully assigned tasks for user request with id = " + strconv.FormatUint(uint64(request.Id), 10))
	log.SetPrefix("[Server]: ")

	return request.Id
}

/* Register worker */
func (c *Coordinator) RegisterWorker(worker *Worker) {
	c.rwmu.Lock()
	defer c.rwmu.Unlock()

	_, found := c.AddressesToWorkers[worker.Address]
	/* If we don't have record about worker with such address then register it as total new to us */
	if !found {
		worker.Id = shared.WorkerId(uuid.New().ID())
	}

	c.Workers[worker.Id] = worker
	c.AddressesToWorkers[worker.Address] = worker

	log.SetPrefix("[Coordintor]: ")
	log.Println("registered worker with id = " + strconv.FormatUint(uint64(worker.Id), 10) + " and address = " + worker.Address)
	log.SetPrefix("[Server]: ")
}

func (c *Coordinator) DeleteWorker(worker *Worker) {
	c.rwmu.Lock()
	defer c.rwmu.Unlock()

	delete(c.Workers, worker.Id)
	delete(c.AddressesToWorkers, worker.Address)

	log.SetPrefix("[Coordintor]: ")
	log.Println("deleted worker with id = " + strconv.FormatUint(uint64(worker.Id), 10) + " and address = " + worker.Address)
	log.SetPrefix("[Server]: ")
}

/* Send heartbeat to workers
 * If worker is DEAD for a long time then delete it
 */
func (c *Coordinator) CheckWorkers() {
	heartbeatDelay := 5 * time.Second
	ticker := time.NewTicker(heartbeatDelay)
	deadDelay := 1 * time.Minute

	for range ticker.C {
		c.rwmu.RLock()
		workers := make([]*Worker, 0, len(c.Workers))
		for _, w := range c.Workers {
			workers = append(workers, w)
		}
		c.rwmu.RUnlock()

		for _, w := range workers {
			response, err := http.Get("http://" + w.Address + "/internal/api/worker/heartbeat")

			if err != nil || response.StatusCode != http.StatusOK {
				if w.Status == shared.DEAD && time.Since(w.LastHB) >= deadDelay {
					c.DeleteWorker(w)
				} else {
					c.rwmu.Lock()
					w.Status = shared.DEAD
					c.rwmu.Unlock()
				}
			} else if err == nil && response.StatusCode == http.StatusOK {
				c.rwmu.Lock()
				w.LastHB = time.Now()
				w.Status = c.getWorkerStatus(w.Address)
				c.rwmu.Unlock()
			}
		}
	}
}

/* Get worker status */
func (c *Coordinator) getWorkerStatus(address string) shared.WorkerStatus {
	resp, err := http.Get("http://" + address + "/internal/api/worker/status")
	if err != nil || resp.StatusCode != http.StatusOK {
		return shared.DEAD
	}
	defer resp.Body.Close()

	var statusResponse shared.WorkerStatusResponse

	if err := json.NewDecoder(resp.Body).Decode(&statusResponse); err != nil {
		return shared.DEAD
	}

	return statusResponse.Status
}

func (c *Coordinator) TaskLaunch(worker *Worker, task *shared.WorkerTask) (bool, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(task); err != nil {
		return false, err
	}

	resp, err := http.Post(
		"http://"+worker.Address+"/internal/api/worker/crack?id="+strconv.FormatUint(uint64(task.Id), 10),
		"application/json",
		&buf)
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	c.rwmu.Lock()
	task.WorkerId = worker.Id
	c.WorkersTasks[task.Id] = task
	c.UserRequestsToTasks[task.RequestId] = append(c.UserRequestsToTasks[task.RequestId], task)
	c.rwmu.Unlock()

	log.SetPrefix("[Coordintor]: ")
	log.Println("successfully assigned tasks with id = " + strconv.FormatUint(uint64(task.Id), 10) + " to worker with address = " + worker.Address)
	log.SetPrefix("[Server]: ")

	return true, nil
}

func (c *Coordinator) TaskStatus(task *shared.WorkerTask) shared.TaskStatus {
	c.rwmu.RLock()
	worker := c.Workers[task.WorkerId]
	c.rwmu.RUnlock()

	resp, err := http.Get("http://" + worker.Address + "/internal/api/worker/crack?id=" + strconv.FormatUint(uint64(task.Id), 10))
	if err != nil {
		return shared.UNKNOWN
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return shared.UNKNOWN
	}

	var statusResponse shared.TaskStatusResponse

	if err := json.NewDecoder(resp.Body).Decode(&statusResponse); err != nil {
		return shared.UNKNOWN
	}

	c.rwmu.Lock()
	task.Status = statusResponse.Status
	c.rwmu.Unlock()

	log.SetPrefix("[Coordintor]: ")
	log.Println("status of task with id = " + strconv.FormatUint(uint64(task.Id), 10) + " is " + string(task.Status))
	log.SetPrefix("[Server]: ")

	return task.Status
}

func (c *Coordinator) TaskKill(task *shared.WorkerTask) error {
	c.rwmu.RLock()
	worker, exists := c.Workers[task.WorkerId]
	c.rwmu.RUnlock()
	if !exists {
		return fmt.Errorf("worker with ID %d not found", task.WorkerId)
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(task); err != nil {
		return err
	}

	resp, err := http.Post(
		"http://"+worker.Address+"/internal/api/worker/crack?id="+strconv.FormatUint(uint64(task.Id), 10),
		"application/json",
		&buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	c.rwmu.Lock()
	task.Status = shared.KILLED
	c.rwmu.Unlock()

	return nil
}

func (c *Coordinator) UpdateTask(task *shared.WorkerTask) {
	c.rwmu.Lock()
	request, found := c.UserRequests[task.RequestId]
	if !found {
		return
	}

	c.WorkersTasks[task.Id] = task
	request.TasksDone += 1
	c.rwmu.Unlock()

	if request.TasksDone == request.TasksScheduled {
		c.rwmu.Lock()
		request.Status = READY
		for _, t := range c.UserRequestsToTasks[request.Id] {
			if t.Result != "" {
				request.Result = t.Result
			}
		}
		c.rwmu.Unlock()
	}
}
