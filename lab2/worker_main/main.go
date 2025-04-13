package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"lab2/task"
	"lab2/worker"

	"gopkg.in/yaml.v3"
)

type ServerContext struct {
	Port               string
	CoordinatorAddress string
	RetryConnectDelay  time.Duration
	MaxRetries         int
	Status             pworker.WorkerStatus
	Tasks              []ptask.Task
	RWmutex            sync.RWMutex
}

type Config struct {
	Port               string        `yaml:"port"`
	CoordinatorAddress string        `yaml:"coordinator_address"`
	RetryConnectDelay  time.Duration `yaml:"retry_connect_delay"`
	MaxRetries         int           `yaml:"max_retries"`
}

var server_context ServerContext
var config Config

func parse_configs() error {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(file, &config); err != nil {
		return err
	}

	return nil
}

func register_worker() error {
	var buf bytes.Buffer

	for attempt := 1; attempt <= server_context.MaxRetries; attempt++ {
		if err := json.NewEncoder(&buf).Encode(server_context.Status); err != nil {
			return err
		}

		resp, err := http.Post(
			"http://"+server_context.CoordinatorAddress+"/internal/api/worker/register",
			"application/json",
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
		if attempt < server_context.MaxRetries {
			log.Println("try again after delay")
			time.Sleep(server_context.RetryConnectDelay)
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

	server_context.Port = config.Port
	server_context.CoordinatorAddress = config.CoordinatorAddress
	server_context.RetryConnectDelay = config.RetryConnectDelay
	server_context.MaxRetries = config.MaxRetries

	http.HandleFunc("/internal/api/worker/status", GetWorkerStatusHandler(&server_context))
	http.HandleFunc("/internal/api/worker/crack", SubmitTaskHandler(&server_context))
	http.HandleFunc("/internal/api/worker/kill", KillTaskHandler(&server_context))
	http.HandleFunc("/internal/api/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alive"))
	})

	log.Println("all handlers were set up")

	server_context.RWmutex.Lock()
	server_context.Status = pworker.IDLE
	server_context.RWmutex.Unlock()
	//check if we died and then awaken and there is some tasks -> context.Status = CRACKING

	if err := register_worker(); err != nil {
		log.Println("failed to register worker after retries:", err)
		return
	}
	log.Println("register worker sucessfully")

	go func() {
		server_context.RWmutex.RLock()
		port := server_context.Port
		server_context.RWmutex.RUnlock()

		log.Println("server is listening on port", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Println("error while starting server:", err)
			return
		}
	}()

	/* to keep the main goroutine alive */
	select {}
}
