package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lab2/shared"
	ptask "lab2/task"
	pworker "lab2/worker"

	amqp "github.com/rabbitmq/amqp091-go"
)

func GetWorkerStatusHandler(workerContext *WorkerContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workerContext.RWmutex.RLock()
		defer workerContext.RWmutex.RUnlock()

		response := pworker.WorkerStatusResponse{
			Status: workerContext.Status,
			Port:   workerContext.Config.Port,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func SubmitTaskHandler(workerContext *WorkerContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()

		taskId := queryParams.Get("taskId")
		if taskId == "" {
			http.Error(w, "missing taskId parameter", http.StatusBadRequest)
			return
		}

		value, err := strconv.ParseUint(taskId, 10, 32)
		if err != nil {
			log.Fatal(err)
		}

		var task ptask.Task
		task.Id = shared.TaskId(value)
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)

		go processTask(workerContext, &task)
	}
}

func processTask(workerContext *WorkerContext, task *ptask.Task) {
	context, cancel := context.WithCancel(context.Background())
	task.CancelFunc = cancel

	workerContext.RWmutex.Lock()
	workerContext.Tasks = append(workerContext.Tasks, task)
	workerContext.RWmutex.Unlock()

	log.Printf("Got task with id = %d and range = %s", task.Id, task.InputRange)

	defer cancel()

	select {
	/* if task was canceles */
	case <-context.Done():
		workerContext.RWmutex.Lock()
		task.Status = ptask.KILLED
		task.Result = []string{""}
		workerContext.RWmutex.Unlock()

		SendTaskResultToCoordinator(task, workerContext.Config.CoordinatorAddress)

		return

	default:
		success := crack(task)

		if !success {
			workerContext.RWmutex.Lock()
			task.Status = ptask.DONE_FAILURE
			task.Result = []string{""}
			workerContext.RWmutex.Unlock()
		}

		SendTaskResultToCoordinator(task, workerContext.Config.CoordinatorAddress)
	}
}

func SendTaskResultToCoordinator(task *ptask.Task, coordinatorAddress string) {
	response := ptask.TaskResultResponse{
		Status: task.Status,
		Result: task.Result,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&response); err != nil {
		log.Fatal(err)
	}

	resp, err := http.Post(
		"http://"+coordinatorAddress+"/internal/api/task/result?taskId="+strconv.FormatUint(uint64(task.Id), 10),
		"application/json",
		&buf,
	)
	if err != nil {
		log.Fatal("failed to send task update:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatal("unexpected status code:", resp.StatusCode)
	}

	log.Printf("Send assigned task with id = %d to coordinator", task.Id)
}

func KillTaskHandler(workerContext *WorkerContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()

		taskIdStr := queryParams.Get("taskId")
		if taskIdStr == "" {
			http.Error(w, "missing requestId parameter", http.StatusBadRequest)
			return
		}

		taskId, err := strconv.ParseUint(taskIdStr, 10, 32)
		if err != nil {
			log.Fatal(err)
		}

		var task *ptask.Task

		workerContext.RWmutex.RLock()
		for _, t := range workerContext.Tasks {
			if t.Id == shared.TaskId(taskId) {
				task = t
			}
		}
		workerContext.RWmutex.RUnlock()

		if task == nil {
			http.Error(w, "task with id = "+strconv.FormatUint(uint64(taskId), 10)+"not found", http.StatusNotFound)
			return
		}

		workerContext.RWmutex.RLock()
		task.CancelFunc()
		workerContext.RWmutex.RUnlock()

		w.WriteHeader(http.StatusOK)
	}
}

func crack(task *ptask.Task) bool {
	/* parse task's range */
	start, end, err := func(inputRange string) (string, string, error) {
		parts := strings.Split(inputRange, "-")
		if len(parts) != 2 {
			return "", "", errors.New("invalid task's range")
		}
		return parts[0], parts[1], nil
	}(task.InputRange)

	if err != nil {
		log.Printf("%v", err)
		return false
	}

	incrementString := func(str string) string {
		runes := []rune(str)
		for i := len(runes) - 1; i >= 0; i-- {
			if runes[i] < 'z' {
				runes[i]++
				return string(runes)
			} else {
				runes[i] = 'a'
			}
		}
		return string(runes)
	}

	success := false
	for input := start; strings.Compare(end, input) >= 0; input = incrementString(input) {
		computedHash := md5.Sum([]byte(input))
		// log.Println("input = "+input+"; computed hash = ", computedHash)
		if hex.EncodeToString(computedHash[:]) == task.Hash {
			workerContext.RWmutex.Lock()
			task.Status = ptask.DONE_SUCCESS
			task.Result = append(task.Result, input)
			workerContext.RWmutex.Unlock()
			success = true
		}
		if strings.Compare(end, input) == 0 {
			break
		}
	}

	return success
}

func ConsumeTask(workerContext *WorkerContext, task *ptask.Task) error {
	context, cancel := context.WithCancel(context.Background())
	task.CancelFunc = cancel

	workerContext.RWmutex.Lock()
	workerContext.Tasks = append(workerContext.Tasks, task)
	workerContext.RWmutex.Unlock()

	log.Printf("Got task with id = %d and range = %s", task.Id, task.InputRange)

	select {
	case <-context.Done():
		workerContext.RWmutex.Lock()
		task.Status = ptask.KILLED
		task.Result = []string{""}
		workerContext.RWmutex.Unlock()

		err := SendTaskResult(workerContext, task)

		if err != nil {
			return err
		}
		return nil

	default:
		success := crack(task)

		if !success {
			workerContext.RWmutex.Lock()
			task.Status = ptask.DONE_FAILURE
			task.Result = []string{""}
			workerContext.RWmutex.Unlock()
		}

		err := SendTaskResult(workerContext, task)
		if err != nil {
			return err
		}
	}
	return nil
}

func KillTask(workerContext *WorkerContext, task *ptask.Task) error {
	log.Printf("Got task to kill with id = %d and range = %s", task.Id, task.InputRange)

	found := false
	workerContext.RWmutex.RLock()
	for _, t := range workerContext.Tasks {
		if t.Id == shared.TaskId(task.Id) {
			task = t
			found = true
			break
		}
	}
	workerContext.RWmutex.RUnlock()

	if !found {
		return fmt.Errorf("error: task with id = %d not found", int(task.Id))
	}

	workerContext.RWmutex.RLock()
	task.CancelFunc()
	workerContext.RWmutex.RUnlock()

	return nil
}

func SendTaskResult(workerContext *WorkerContext, task *ptask.Task) error {
	// TODO for test purposes
	time.Sleep(time.Second * 3)

	var task_json bytes.Buffer
	if err := json.NewEncoder(&task_json).Encode(task); err != nil {
		return err
	}

	log.Printf("sending task %d result to coordinator", task.Id)

	for {
		ch, err := workerContext.RabbitMQ.GetChannel()

		if err != nil || ch.IsClosed() {
			return fmt.Errorf("channel is not available")
		}

		err = ch.Publish(
			workerContext.RabbitMQ.ExchangeName,
			"coordinator",
			false,
			false,
			amqp.Publishing{
				DeliveryMode: amqp.Persistent,
				ContentType:  "application/json",
				Priority:     0,
				Body:         task_json.Bytes(),
			},
		)

		if err == nil {
			return nil
		}

		log.Printf("Failed to send task result, retrying: %v", err)
		time.Sleep(workerContext.Config.RabbitConnRetryDelay)
	}
}
