package main

import (
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"gopkg.in/yaml.v3"

	pcoordinator "lab2/coordinator"
	"lab2/database"
	prabbitmq "lab2/rabbitmq"
)

type WorkerContext struct {
	Coordinator     *pcoordinator.Coordinator
	DbConnStr       string
	RabbitMqConnStr string
	ExchangeName    string
	Config          Config
}

type Config struct {
	Port           string        `yaml:"port"`
	HeartbeatDelay time.Duration `yaml:"heartbeat_delay"`
	DeadDelay      time.Duration `yaml:"dead_delay"`
	TaskTimeout    time.Duration `yaml:"task_timeout"`
	TaskRetries    int           `yaml:"task_retries"`
}

var context WorkerContext

func parse_configs() error {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(file, &context.Config); err != nil {
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
		log.Fatalf("cannot connect to database with connStr \"%s\": %s\n",
			context.DbConnStr, err)
	}

	/* Connect to RabbitMq */
	rabbitmq := prabbitmq.NewRabbitMQManager(context.RabbitMqConnStr, context.ExchangeName)
	err = rabbitmq.Connect()
	if err != nil {
		log.Fatalf("cannot connecgt to RabbitMq: %s\n", err)
	}

	/* Create coordinator */
	context.Coordinator =
		pcoordinator.NewCoordinator(db,
			rabbitmq,
			context.Config.HeartbeatDelay,
			context.Config.DeadDelay,
			context.Config.TaskTimeout,
			context.Config.TaskRetries)

	err = context.Coordinator.RecoverAfterCrash()
	if err != nil {
		log.Fatalf("cannot recover after crash: %s\n", err)
	}
	log.Printf("successfully recover after crash\n")

	/* define handlers */
	http.HandleFunc("/api/hash/status", GetRequestStatusHandler(context.Coordinator))
	http.HandleFunc("/api/hash/crack", SubmitRequestCrackHandler(context.Coordinator))
	http.HandleFunc("/internal/api/worker/register", RegisterNewWorkerHandler(context.Coordinator))
	http.HandleFunc("/internal/api/task/result", GetTaskResultHandler(context.Coordinator))

	log.Println("all handlers were set up")

	go func() {
		log.Printf("server is listening on port %s\n", context.Config.Port)
		if err := http.ListenAndServe(":"+context.Config.Port, nil); err != nil {
			log.Printf("error while starting server: %s\n", err)
			return
		}
	}()

	context.Coordinator.Start()

	/* to keep the main goroutine alive */
	select {}
}
