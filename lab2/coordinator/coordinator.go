package pcoordinator

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"lab2/database"
	"lab2/request"
	"lab2/shared"
	"lab2/task"
	"lab2/worker"
)

type Coordinator struct {
	db             *sql.DB
	channel        *amqp.Channel
	HeartbeatDelay time.Duration
	DeadDelay      time.Duration
	TaskTimeout    time.Duration
	TaskRetries    int
}

/* Init Coordinator instance */
func NewCoordinator(db *sql.DB, rabbitmq *amqp.Connection) (*Coordinator, error) {
	log.Println("Coordinator was created")

	channel, err := rabbitmq.Channel()
	if err != nil {
		return nil, err
	}

	err = channel.ExchangeDeclare(
		"exchange",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &Coordinator{
		db:      db,
		channel: channel,
	}, nil
}

func (c *Coordinator) PublishTask(worker *pworker.Worker, task_json bytes.Buffer) error {
	return c.channel.Publish(
		"",
		strconv.FormatUint(uint64(worker.Id), 10),
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Priority:     0,
			Body:         task_json.Bytes(),
		},
	)
}

func (c *Coordinator) PublishKillingTask(worker *pworker.Worker, task_json bytes.Buffer) error {
	return c.channel.Publish(
		"",
		strconv.FormatUint(uint64(worker.Id), 10),
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Priority:     9,
			Body:         task_json.Bytes(),
		},
	)
}

func (c *Coordinator) GetUserRequestStatus(requestId shared.UserRequestId) prequest.UserStatusResponse {
	statusResponse, err := database.GetUserRequestStatusById(c.db, requestId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("Request ID %d not found\n", requestId)
			return prequest.UserStatusResponse{
				Status: prequest.ERROR,
				Result: []string{""},
			}
		}

		log.Printf("Error getting status for request ID %d: %v\n", requestId, err)
		return prequest.UserStatusResponse{
			Status: prequest.ERROR,
			Result: []string{""},
		}
	}

	log.Printf("GetUserRequestStatus(%d) called: Status = %s; Result = %s\n",
		requestId,
		statusResponse.Status,
		statusResponse.Result)
	return statusResponse
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

func (c *Coordinator) setRequestError(request *prequest.UserRequest) {
	tasks, _ := database.GetTasksByRequestId(c.db, request.Id)
	for _, task := range tasks {
		worker, _ := database.GetWorkerById(c.db, task.WorkerId)
		c.taskKill(worker, &task)
	}
	database.UpdateTaskStatusAndResultByRequestId(c.db, request.Id, ptask.KILLED, []string{""})
	database.UpdateRequestStatusAndResult(c.db, request.Id, prequest.ERROR, []string{""})
}

func (c *Coordinator) setRequestTimeout(request *prequest.UserRequest) {
	tasks, _ := database.GetTasksByRequestId(c.db, request.Id)
	for _, task := range tasks {
		worker, _ := database.GetWorkerById(c.db, task.WorkerId)
		c.taskKill(worker, &task)
	}
	database.UpdateTaskStatusAndResultByRequestId(c.db, request.Id, ptask.KILLED, []string{""})
	database.UpdateRequestStatusAndResult(c.db, request.Id, prequest.TIMEOUT_ERROR, []string{""})
}

func (c *Coordinator) assignTasks(request *prequest.UserRequest) {
get_workers:
	workers, err := database.GetAllAliveWorkers(c.db)
	if err != nil {
		c.setRequestError(request)
		log.Printf("ERROR cannot get alive workers\n")
		return
	}
	numWorkers := len(workers)
	if numWorkers == 0 {
		log.Printf("WARNING no alive workers\n")
		time.Sleep(c.DeadDelay)
		goto get_workers
	}
	crack_len := request.MaxLength
	chunks := splitRange(strings.Repeat("a", int(crack_len)), strings.Repeat("z", int(crack_len)), numWorkers)

	ctx, cancel := context.WithTimeout(context.Background(), c.TaskTimeout)
	defer cancel()

	go func(c *Coordinator, request *prequest.UserRequest) {
		task := ptask.Task{
			RequestId: request.Id,
			Hash:      request.Hash,
			MaxLength: request.MaxLength,
			Status:    ptask.IN_PROGRESS,
		}

		retry := 0
		for i, chunk := range chunks {
			task.InputRange = chunk

			id, err := database.AddTask(c.db, &task)
			if err != nil {
				c.setRequestError(request)
				log.Printf("ERROR while saving task\n")
				return
			}
			task.Id = id

		retryTaskLaunch:
			select {
			case <-ctx.Done():
				return
			default:
				worker := workers[(i+retry)%numWorkers]

				launched, err := c.taskLaunch(worker, &task)
				if !launched || err != nil {
					log.Printf("ERROR cannot assign task: %v\n", err)
					if retry >= c.TaskRetries {
						c.setRequestError(request)
						return
					}
					retry++
					goto retryTaskLaunch
				}
				task.WorkerId = worker.Id
				err = database.UpdateTaskWorkerId(c.db, &task)
				if err != nil {
					c.setRequestError(request)
					log.Printf("ERROR while assigning task")
				}
			}
		}
	}(c, request)

	/* Wait for the context to be done (timeout or cancellation) */
	<-ctx.Done()
	if ctx.Err() == context.DeadlineExceeded {
		c.setRequestTimeout(request)

		log.Printf("timeout while trying to assign tasks for user request with id = %d\n", request.Id)
	}
}

/* Creates task and map it to workers */
func (c *Coordinator) Crack(request *prequest.UserRequest) (shared.UserRequestId, error) {
	id, err := database.AddUserRequest(c.db, request)
	if err != nil {
		return shared.UserRequestId(0), err
	}
	request.Id = id

	go c.assignTasks(request)

	log.Printf("Crack(user_request) called: UserRequestId = %d; Error = %v\n", id, err)

	return request.Id, err
}

/* Register worker */
func (c *Coordinator) RegisterWorker(worker *pworker.Worker) error {
	_, err := database.GetWorkerByAddress(c.db, worker.Address)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var id shared.WorkerId
	if err != nil && errors.Is(err, sql.ErrNoRows) { /* Such worker isn't found then register this new one */
		id, err = database.AddWorker(c.db, worker)
		log.Printf("Got new worker\n")
	} else { /* This worker is existing then update it */
		id, err = database.UpdateWorker(c.db, worker)
		log.Printf("Got known worker\n")
	}

	log.Printf("RegisterWorker(worker) called: Id = %d; Address = %s; Status = %s; Error = %v;\n", id, worker.Address, worker.Status, err)

	return err
}

func (c *Coordinator) UpdateWorkerStatus(worker *pworker.Worker) error {
	err := database.UpdateWorkerStatus(c.db, worker)

	log.Printf("UpdateWorkerStatus(worker) called: Id = %d; Address = %s; Status = %s; Error = %v\n", worker.Id, worker.Address, worker.Status, err)

	return err
}

func (c *Coordinator) UpdateWorkerStatusAndLastHB(worker *pworker.Worker) error {
	_, err := database.UpdateWorker(c.db, worker)

	// log.Printf("UpdateWorkerStatusAndLastHB(worker, time) called: Id = %d; Address = %s Error = %v\n", id, worker.Address, err)

	return err
}

func (c *Coordinator) DeleteWorker(worker *pworker.Worker) error {
	err := database.DeleteWorker(c.db, worker)

	log.Printf("DeleteWorker(worker) called: Id = %d; Address = %s; Status = %s; Error = %v\n", worker.Id, worker.Address, worker.Status, err)

	return err
}

/* Send heartbeat to workers
 * If worker is DEAD for a long time then delete it
 */
func (c *Coordinator) CheckWorkers() {
	ticker := time.NewTicker(c.HeartbeatDelay)

	for range ticker.C {
		workers, err := database.GetAllWorkers(c.db)
		if err != nil {
			continue
		}

		for _, worker := range workers {
			go func() {
				c.sendHB(worker)

				if worker.Status == pworker.DEAD && time.Since(worker.LastHB) >= c.DeadDelay {
					err = c.DeleteWorker(worker)
				} else {
					err = c.UpdateWorkerStatusAndLastHB(worker)
				}
				if err != nil {
					return
				}
			}()
		}
	}
}

func (c *Coordinator) sendHB(worker *pworker.Worker) {
	resp, err := http.Get("http://" + worker.Address + "/internal/api/worker/heartbeat")
	if err != nil || resp.StatusCode != http.StatusOK {
		worker.Status = pworker.DEAD
		return
	}
	defer resp.Body.Close()

	worker.Status = pworker.ALIVE
	worker.LastHB = time.Now()
}

func (c *Coordinator) UpdateTask(task *ptask.Task) error {
	log.Printf("Got task with id = %d and status = %s; result = %s", task.Id, task.Status, task.Result)

	err := database.UpdateTaskStatusAndResult(c.db, task)
	if err != nil {
		return err
	}

	task, err = database.GetTask(c.db, task.Id)
	if err != nil {
		return err
	}

	/* check if all tasks are done */
	complete, err := database.CountCompleteTasks(c.db, task.RequestId)
	if err != nil {
		return err
	}

	if complete {
		err = c.finalizeUserRequest(task.RequestId)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Coordinator) finalizeUserRequest(requestId shared.UserRequestId) error {
	results, err := database.GetTaskResultsByRequestId(c.db, requestId)
	if err != nil {
		return err
	}

	var requestResult []string

	for _, result := range results {
		if result.Status == ptask.DONE_SUCCESS {
			requestResult = append(requestResult, result.Result...)
		}
	}

	if len(requestResult) == 0 {
		requestResult = append(requestResult, "")
	}

	err = database.UpdateRequestStatusAndResult(c.db, requestId, prequest.READY, requestResult)
	if err != nil {
		return err
	}

	return nil
}

func (c *Coordinator) taskLaunch(worker *pworker.Worker, task *ptask.Task) (bool, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(task); err != nil {
		return false, err
	}

	c.PublishTask(worker, buf)

	// resp, err := http.Post(
	// 	"http://"+worker.Address+"/internal/api/worker/crack?taskId="+strconv.FormatUint(uint64(task.Id), 10),
	// 	"application/json",
	// 	&buf)
	// if err != nil {
	// 	return false, err
	// }

	// defer resp.Body.Close()

	// if resp.StatusCode != http.StatusOK {
	// 	log.Printf("ERROR non OK")
	// 	return false, nil
	// }
	log.Printf("Successfully assigned task with id = %d from request with id = %d to worker %d with address = %s", task.Id, task.RequestId, worker.Id, worker.Address)

	return true, nil
}

func (c *Coordinator) taskKill(worker *pworker.Worker, task *ptask.Task) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(task); err != nil {
		return
	}

	c.PublishKillingTask(worker, buf)

	// resp, err := http.Post(
	// 	"http://"+worker.Address+"/internal/api/worker/kill?id="+strconv.FormatUint(uint64(task.Id), 10),
	// 	"application/json",
	// 	&buf)
	// if err != nil {
	// 	return
	// }
	// defer resp.Body.Close()

	// if resp.StatusCode != http.StatusOK {
	// 	return
	// }

	log.Printf("Successfully killed task with id = %d from request with id = %d to worker %d with address = %s", task.Id, task.RequestId, worker.Id, worker.Address)
}
