package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
	// "lab1/worker"
)

type Config struct {
	Port               string `yaml:"port"`
	CoordinatorAddress string `yaml:"coordinator_address"`
}

type ServerContext struct {
	Port               string
	CoordinatorAddress string
	// Worker *worker.Worker
	// rwmu        sync.RWMutex
}

var context ServerContext

func getWorkerStatus(w http.ResponseWriter, r *http.Request) {

}

func parse_configs() (*Config, error) {
	var config Config

	file, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func connect_to_coordinator() error {
	retryDelay := 5 * time.Second
	maxRetries := 2

	requestBody := struct {
		Address string `json:"address"`
	}{"127.0.0.1" + context.Port}

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
	config, err := parse_configs()
	if err != nil {
		log.Println("error while parsing config file:", err)
		return
	}
	log.Println("configs were parsed sucessfully")

	/* define server context */
	context = ServerContext{
		Port:               config.Port,
		CoordinatorAddress: config.CoordinatorAddress,
		// Worker:
	}
	log.Println("server context was created")

	http.HandleFunc("/api/worker/status", getWorkerStatus)

	if err := connect_to_coordinator(); err != nil {
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
