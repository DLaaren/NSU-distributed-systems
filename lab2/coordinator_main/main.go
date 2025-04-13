package main

import (
	"log"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"

	"lab2/coordinator"
	"lab2/database"
)

type ServerContext struct {
	Port        string `yaml:"port"`
	Coordinator *pcoordinator.Coordinator
	DbConnStr   string `yaml:"db_conn_str"`
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

	db, err := database.Initdb(context.DbConnStr)
	if err != nil {

	}

	context.Coordinator = pcoordinator.NewCoordinator(db)

	/* define handlers */
	http.HandleFunc("/api/hash/status", GetRequestStatusHandler(context.Coordinator))
	http.HandleFunc("/api/hash/crack", SubmitRequestCrackHandler(context.Coordinator))
	http.HandleFunc("/internal/api/worker/register", RegisterNewWorkerHandler(context.Coordinator))
	http.HandleFunc("/internal/api/task/result", GetTaskResultHandler(context.Coordinator))

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
