package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"gopkg.in/yaml.v3"

	"lab1/coordinator"
	"lab1/worker"
)

type Config struct {
	Port string `yaml:"port"`
}

type ServerContext struct {
	Port        string
	Coordinator *coordinator.Coordinator
	WorkersMap  map[string]*worker.Worker
	rwmu        sync.RWMutex
}

var context ServerContext

func registerNewWorker(w http.ResponseWriter, r *http.Request) {
	log.Println("new worker wow")
}

func getRequestStatusHandler(w http.ResponseWriter, r *http.Request) {

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
		Port:        config.Port,
		Coordinator: coordinator.NewCoordinator(),
		WorkersMap:  make(map[string]*worker.Worker),
	}
	log.Println("server context was created")

	/* define handlers */
	http.HandleFunc("/api/worker/register", registerNewWorker)
	http.HandleFunc("/api/hash/status", getRequestStatusHandler)
	http.HandleFunc("/api/hash/crack", submitRequestCrackHandler)
	// http.HandleFunc("/task/launch", )
	// http.HandleFunc("/task/status", )
	// http.HandleFunc("/task/kill", )
	log.Println("all handlers were set up")

	go func() {
		log.Printf("server is listening on port %s\n", context.Port)
		if err := http.ListenAndServe(":"+context.Port, nil); err != nil {
			log.Printf("error while starting server: %s\n", err)
			return
		}
	}()

	/* to keep the main goroutine alive */
	select {}
}
