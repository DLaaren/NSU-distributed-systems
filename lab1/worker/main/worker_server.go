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

	"lab1/worker"
)

type ServerContext struct {
	Port               string `yaml:"port"`
	CoordinatorAddress string `yaml:"coordinator_address"`
	Worker             *worker.Worker
}

var context ServerContext

func getWorkerStatus(w http.ResponseWriter, r *http.Request) {

}

func submitTaskHandler(w http.ResponseWriter, r *http.Request) {

}

func killTaskHandler(w http.ResponseWriter, r *http.Request) {

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

	requestBody := context.Worker

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

	context.Worker = worker.NewWorker("localhost:" + context.Port)

	http.HandleFunc("/internal/api/worker/status", getWorkerStatus)
	http.HandleFunc("/internal/api/worker/crack", submitTaskHandler)
	http.HandleFunc("/internal/api/worker/kill", killTaskHandler)

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
