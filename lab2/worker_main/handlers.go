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
	"lab2/task"
	"lab2/worker"

	amqp "github.com/rabbitmq/amqp091-go"
)

func GetWorkerStatusHandler(sc *ServerContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sc.RWmutex.RLock()
		response := pworker.WorkerStatusResponse{
			Status: sc.Status,
			Port:   sc.Config.Port,
		}
		sc.RWmutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func SubmitTaskHandler(sc *ServerContext) http.HandlerFunc {
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

		ctx, cancel := context.WithCancel(context.Background())
		task.CancelFunc = cancel

		sc.RWmutex.Lock()
		sc.Tasks = append(sc.Tasks, &task)
		sc.RWmutex.Unlock()

		log.Printf("Got task with id = %d and range = %s", task.Id, task.InputRange)

		go func(sc *ServerContext, task *ptask.Task) {
			defer cancel()
			select {
			case <-ctx.Done():
				sc.RWmutex.Lock()
				task.Status = ptask.KILLED
				task.Result = []string{""}
				sc.RWmutex.Unlock()

				SendTaskResultToCoordinator(task, sc.CoordinatorAddress)

				return

			default:
				start, end, err := func(inputRange string) (string, string, error) {
					parts := strings.Split(inputRange, "-")
					if len(parts) != 2 {
						return "", "", errors.New("invalid input range")
					}
					return parts[0], parts[1], nil
				}(task.InputRange)
				if err != nil {
					http.Error(w, "invalid request body", http.StatusBadRequest)
					return
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
						sc.RWmutex.Lock()
						task.Status = ptask.DONE_SUCCESS
						task.Result = append(task.Result, input)
						sc.RWmutex.Unlock()
						success = true
					}
					if strings.Compare(end, input) == 0 {
						break
					}
				}

				if !success {
					sc.RWmutex.Lock()
					task.Status = ptask.DONE_FAILURE
					task.Result = []string{""}
					sc.RWmutex.Unlock()
				}

				SendTaskResultToCoordinator(task, sc.CoordinatorAddress)
			}
		}(sc, &task)

		w.WriteHeader(http.StatusOK)
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

func KillTaskHandler(sc *ServerContext) http.HandlerFunc {
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

		sc.RWmutex.RLock()
		for _, t := range sc.Tasks {
			if t.Id == shared.TaskId(taskId) {
				task = t
			}
		}
		sc.RWmutex.RUnlock()

		if task == nil {
			http.Error(w, "task with id = "+strconv.FormatUint(uint64(taskId), 10)+"not found", http.StatusNotFound)
			return
		}

		sc.RWmutex.RLock()
		task.CancelFunc()
		sc.RWmutex.RUnlock()

		w.WriteHeader(http.StatusOK)
	}
}

func SubmitTask(sc *ServerContext, task *ptask.Task) error {
	ctx, cancel := context.WithCancel(context.Background())
	task.CancelFunc = cancel

	sc.RWmutex.Lock()
	sc.Tasks = append(sc.Tasks, task)
	sc.RWmutex.Unlock()

	log.Printf("Got task with id = %d and range = %s", task.Id, task.InputRange)

	select {
	case <-ctx.Done():
		sc.RWmutex.Lock()
		task.Status = ptask.KILLED
		task.Result = []string{""}
		sc.RWmutex.Unlock()

		err := SendTaskResult(sc, task)

		if err != nil {
			return err
		}
		return nil

	default:
		sc.RWmutex.RLock()
		input := task.InputRange
		sc.RWmutex.RUnlock()

		start, end, err := func(inputRange string) (string, string, error) {
			parts := strings.Split(inputRange, "-")
			if len(parts) != 2 {
				return "", "", errors.New("invalid input range")
			}
			return parts[0], parts[1], nil
		}(input)
		if err != nil {
			return err
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
				sc.RWmutex.Lock()
				task.Status = ptask.DONE_SUCCESS
				task.Result = append(task.Result, input)
				sc.RWmutex.Unlock()
				success = true
			}
			if strings.Compare(end, input) == 0 {
				break
			}
		}

		if !success {
			sc.RWmutex.Lock()
			task.Status = ptask.DONE_FAILURE
			task.Result = []string{""}
			sc.RWmutex.Unlock()
		}

		err = SendTaskResult(sc, task)
		if err != nil {
			return err
		}
	}
	return nil
}

func KillTask(sc *ServerContext, task *ptask.Task) error {
	log.Printf("Got task to kill with id = %d and range = %s", task.Id, task.InputRange)

	found := false
	sc.RWmutex.RLock()
	for _, t := range sc.Tasks {
		if t.Id == shared.TaskId(task.Id) {
			task = t
			found = true
			break
		}
	}
	sc.RWmutex.RUnlock()

	if !found {
		return fmt.Errorf("error: task with id = %d not found", int(task.Id))
	}

	sc.RWmutex.RLock()
	task.CancelFunc()
	sc.RWmutex.RUnlock()

	return nil
}

func SendTaskResult(sc *ServerContext, task *ptask.Task) error {
	// TODO for test purposes
	time.Sleep(time.Second * 3)
	var task_json bytes.Buffer
	if err := json.NewEncoder(&task_json).Encode(task); err != nil {
		return err
	}

	log.Printf("sending task %d result to coordinator", task.Id)

	for {
		sc.RWmutex.RLock()
		ch := sc.RabbitMQ.Channel
		sc.RWmutex.RUnlock()

		if ch == nil || ch.IsClosed() {
			return fmt.Errorf("channel is not available")
		}

		err := ch.Publish(
			sc.ExchangeName,
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
		time.Sleep(worker_context.Config.RetryConnectDelay)
	}
}
