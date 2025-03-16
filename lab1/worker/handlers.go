package worker

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"lab1/shared"
)

func GetWorkerStatusHandler(worker *WorkerContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		worker.rwmu.RLock()
		response := shared.WorkerStatusResponse{
			Status: worker.Status,
		}
		worker.rwmu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func registerTask(worker *WorkerContext, task shared.WorkerTask) {
	worker.rwmu.Lock()
	worker.Tasks[task.Id] = &task
	worker.rwmu.Unlock()
}

func SubmitTaskHandler(worker *WorkerContext, coordinatorAddress string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()

		requestId := queryParams.Get("requestId")
		if requestId == "" {
			http.Error(w, "missing requestId parameter", http.StatusBadRequest)
			return
		}

		value, err := strconv.ParseUint(requestId, 10, 32)
		if err != nil {
			log.Fatal(err)
		}

		var task shared.WorkerTask
		task.Id = shared.TaskId(value)
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		task.CancelFunc = cancel
		registerTask(worker, task)

		go func() {
			select {
			case <-ctx.Done():
				worker.rwmu.Lock()
				task.Status = shared.KILLED
				task.Result = ""
				worker.rwmu.Unlock()
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

				for input := start; strings.Compare(end, input) >= 0; incrementString(input) {
					computedHash := md5.Sum([]byte(input))
					if hex.EncodeToString(computedHash[:]) == task.Hash {
						worker.rwmu.Lock()
						task.Status = shared.DONE_SUCCESS
						task.Result = input
						worker.rwmu.Unlock()
					}
				}

				worker.rwmu.Lock()
				task.Status = shared.DONE_FAILURE
				task.Result = ""
				worker.rwmu.Unlock()

				sendAnswer(task, coordinatorAddress)
			}
		}()
	}
}

func sendAnswer(task shared.WorkerTask, coordinatorAddress string) {
	response := shared.TaskResultResponse{
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
}

func KillTaskHandler(worker *WorkerContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()

		taskIdStr := queryParams.Get("requestId")
		if taskIdStr == "" {
			http.Error(w, "missing requestId parameter", http.StatusBadRequest)
			return
		}

		taskId, err := strconv.ParseUint(taskIdStr, 10, 32)
		if err != nil {
			log.Fatal(err)
		}

		worker.rwmu.Lock()
		task, exists := worker.Tasks[shared.TaskId(taskId)]
		if !exists {
			http.Error(w, "task with id = "+strconv.FormatUint(uint64(taskId), 10)+"not found", http.StatusNotFound)
			worker.rwmu.Unlock()
			return
		}

		worker.Tasks[shared.TaskId(taskId)].Status = shared.KILLED
		worker.rwmu.Unlock()

		task.CancelFunc()

		// Respond to the client
		w.WriteHeader(http.StatusOK)
	}
}
