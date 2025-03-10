package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"lab1/coordinator"
	"lab1/shared"
)

type ServerContext struct {
	Port        string `yaml:"port"`
	Coordinator *coordinator.Coordinator
}

var context ServerContext

func registerNewWorkerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var worker coordinator.Worker
	var workerStatus coordinator.WorkerStatusResponse
	err := json.NewDecoder(r.Body).Decode(&workerStatus)
	if err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	worker.Address = r.RemoteAddr
	worker.LastHB = time.Now()
	worker.Status = workerStatus.Status
	context.Coordinator.RegisterWorker(&worker)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("worker registered successfully"))

	log.Println("new worker with address", worker.Address, "was registered")
}

func getRequestStatusHandler(w http.ResponseWriter, r *http.Request) {
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

	response := context.Coordinator.UserRequestStatus(shared.Id(value))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func submitRequestCrackHandler(w http.ResponseWriter, r *http.Request) {
	var userRequest coordinator.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	requestId := context.Coordinator.Crack(&userRequest)

	response := coordinator.UserResponse{
		RequestId: requestId,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
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

func main() {
	log.SetPrefix("[Server]: ")

	/* parse configs */
	if err := parse_configs(); err != nil {
		log.Println("error while parsing config file:", err)
		return
	}
	log.Println("configs were parsed sucessfully")

	context.Coordinator = coordinator.NewCoordinator()

	/* define handlers */
	http.HandleFunc("/api/worker/register", registerNewWorkerHandler)
	http.HandleFunc("/api/hash/status", getRequestStatusHandler)
	http.HandleFunc("/api/hash/crack", submitRequestCrackHandler)

	log.Println("all handlers were set up")

	go func() {
		log.Printf("server is listening on port %s\n", context.Port)
		if err := http.ListenAndServe(":"+context.Port, nil); err != nil {
			log.Printf("error while starting server: %s\n", err)
			return
		}
	}()

	go func() {
		context.Coordinator.CheckWorkers()
	}()

	/* to keep the main goroutine alive */
	select {}
}
