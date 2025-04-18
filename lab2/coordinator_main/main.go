package main

import (
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"

	"lab2/coordinator"
	"lab2/database"
)

type ServerContext struct {
	Port            string
	Coordinator     *pcoordinator.Coordinator
	DbConnStr       string
	RabbitMqConnStr string
	ExchangeName    string
}

type Config struct {
	Port           string        `yaml:"port"`
	HeartbeatDelay time.Duration `yaml:"heartbeat_delay"`
	DeadDelay      time.Duration `yaml:"dead_delay"`
	TaskTimeout    time.Duration `yaml:"task_timeout"`
	TaskRetries    int           `yaml:"task_retries"`
}

var context ServerContext
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

func main() {
	log.SetPrefix("[Coordinator]: ")

	/* parse configs */
	if err := parse_configs(); err != nil {
		log.Println("error while parsing config file:", err)
		return
	}
	log.Println("configs were parsed sucessfully")

	context.Port = config.Port
	// "host=postgres port=5432 user=coordinator_user password=coordinator_password dbname=coordinator_db sslmode=disable"
	context.DbConnStr = "host=" + os.Getenv("DB_HOST") +
		" port=" + os.Getenv("DB_PORT") +
		" user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" sslmode=disable"
	context.RabbitMqConnStr = os.Getenv("RABBITMQ_URL")
	context.ExchangeName = os.Getenv("EXCHANGE_NAME")

	/* Connect to database*/
	db, err := database.Initdb(context.DbConnStr)
	if err != nil {
		log.Fatalf("cannot connect to database: %s\n", err)
	}

	/* Connect to rabbitmq */
	rabbitmq, err := amqp.Dial(context.RabbitMqConnStr)
	if err != nil {
		log.Fatalf("cannot connect to RabbitMQ server: %s\n", err)
	}

	/* Create coordinator */
	context.Coordinator, err = pcoordinator.NewCoordinator(db, rabbitmq, context.ExchangeName)
	if err != nil {
		log.Fatalf("cannot create coordinator: %s\n", err)
	}
	context.Coordinator.HeartbeatDelay = config.HeartbeatDelay
	context.Coordinator.DeadDelay = config.DeadDelay
	context.Coordinator.TaskTimeout = config.TaskTimeout
	context.Coordinator.TaskRetries = config.TaskRetries

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
