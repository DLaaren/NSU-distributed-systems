package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"lab1/coordinator"
	"lab1/shared"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type ServerContext struct {
	Port               string `yaml:"port"`
	CoordinatorAddress string `yaml:"coordinator_address"`
	WorkerStatus       coordinator.WorkerStatus
	Tasks              map[shared.Id]*shared.WorkerTask
	rwmu               sync.RWMutex
}

var context ServerContext

func getWorkerStatusHandler(w http.ResponseWriter, r *http.Request) {
	response := coordinator.WorkerStatusResponse{
		Status: context.WorkerStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func submitTaskHandler(w http.ResponseWriter, r *http.Request) {
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
	task.Id = shared.Id(value)
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	context.rwmu.Lock()
	context.Tasks[task.Id] = &task
	context.rwmu.Unlock()

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
			context.rwmu.Lock()
			task.Status = shared.DONE_SUCCESS
			task.Result = input
			context.rwmu.Unlock()
		}
	}

	context.rwmu.Lock()
	task.Status = shared.DONE_FAILURE
	task.Result = ""
	context.rwmu.Unlock()
}

func killTaskHandler(w http.ResponseWriter, r *http.Request) {
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

}

func parse_configs() error {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(file, &context); err != nil {
		return err
	}

	return nil
}

func register_worker() error {
	retryDelay := 5 * time.Second
	maxRetries := 2

	requestBody := context.WorkerStatus

	var buf bytes.Buffer

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := json.NewEncoder(&buf).Encode(requestBody); err != nil {
			return err
		}

		resp, err := http.Post(
			"http://"+context.CoordinatorAddress+"/api/worker/register",
			"aplication/json",
			&buf)

		if err != nil {
			return err
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		} else {
			log.Println("failed to register worker:", resp.Status)
		}
		if attempt < maxRetries {
			log.Println("try again after delay")
			time.Sleep(retryDelay)
		}
	}

	return errors.New("failed to register worker")
}

func main() {
	log.SetPrefix("[Server]: ")

	/* parse configs */
	if err := parse_configs(); err != nil {
		log.Println("error while parsing config file:", err)
		return
	}
	log.Println("configs were parsed sucessfully")

	context.WorkerStatus = coordinator.IDLE
	context.Tasks = make(map[shared.Id]*shared.WorkerTask, 0)

	http.HandleFunc("/internal/api/worker/status", getWorkerStatusHandler)
	http.HandleFunc("/internal/api/worker/crack", submitTaskHandler)
	http.HandleFunc("/internal/api/worker/kill", killTaskHandler)
	http.HandleFunc("/internal/api/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alive"))
	})

	log.Println("all handlers were set up")

	if err := register_worker(); err != nil {
		log.Println("failed to register worker after retries:", err)
		return
	}
	log.Println("register worker sucessfully")

	go func() {
		log.Println("server is listening on port", context.Port)
		if err := http.ListenAndServe(":"+context.Port, nil); err != nil {
			log.Println("error while starting server:", err)
			return
		}
	}()

	/* to keep the main goroutine alive */
	select {}
}
