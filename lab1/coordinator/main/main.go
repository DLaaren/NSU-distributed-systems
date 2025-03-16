package main

import (
	"log"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"

	"lab1/coordinator"
)

type ServerContext struct {
	Port        string `yaml:"port"`
	Coordinator coordinator.CoordinatorI
}

var context ServerContext

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
	http.HandleFunc("/api/hash/status", coordinator.GetRequestStatusHandler(context.Coordinator))
	http.HandleFunc("/api/hash/crack", coordinator.SubmitRequestCrackHandler(context.Coordinator))
	http.HandleFunc("/internal/api/worker/register", coordinator.RegisterNewWorkerHandler(context.Coordinator))
	http.HandleFunc("/internal/api/task/result", coordinator.GetTaskResultHandler(context.Coordinator))

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
