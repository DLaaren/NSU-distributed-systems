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

type ServerContext struct {
	Coordinator *pcoordinator.Coordinator
	Config      Config
}

type Config struct {
	Port                 string        `yaml:"port"`
	HeartbeatDelay       time.Duration `yaml:"heartbeat_delay"`
	DeadDelay            time.Duration `yaml:"dead_delay"`
	TaskTimeout          time.Duration `yaml:"task_timeout"`
	TaskRetries          int           `yaml:"task_retries"`
	RabbitHBDelay        time.Duration `yaml:"rabbit_hb_delay"`
	RabbitConnTimeout    time.Duration `yaml:"rabbit_conn_timeout"`
	RabbitConnRetryDelay time.Duration `yaml:"rabbit_conn_retry_delay"`
	DbConnStr            string
	RabbitMqConnStr      string
	ExchangeName         string
}

var serverContext ServerContext

func main() {
	log.SetPrefix("[Coordinator]: ")

	/* parse configs */
	if err := parse_configs(); err != nil {
		log.Println("error while parsing config file:", err)
		return
	}
	log.Println("configs were parsed sucessfully")

	/* get configs from envs */
	configGetEnv()

	/* Connect to database*/
	db, err := database.Initdb(serverContext.Config.DbConnStr)
	if err != nil {
		log.Fatalf("cannot connect to database with connStr \"%s\": %s\n",
			serverContext.Config.DbConnStr, err)
	}

	/* connect to RabbitMq */
	rabbitmq, err := connectCoordinatorToRabbit()
	if err != nil {
		log.Fatalf("failed to connect to RabbitMq: %s", err)
	}
	log.Printf("connect to RabbitMq\n")

	/* Create coordinator */
	serverContext.Coordinator =
		pcoordinator.NewCoordinator(db,
			rabbitmq,
			serverContext.Config.HeartbeatDelay,
			serverContext.Config.DeadDelay,
			serverContext.Config.TaskTimeout,
			serverContext.Config.TaskRetries)

	/* recover after crash */
	err = serverContext.Coordinator.RecoverAfterCrash()
	if err != nil {
		log.Fatalf("cannot recover after crash: %s\n", err)
	}
	log.Printf("successfully recover after crash\n")

	/* define handlers */
	registerHttpHandlers()
	log.Println("all handlers were set up")

	/* listen on port for coordinator's requests */
	go listenOnPort()

	/* star coordinator work */
	serverContext.Coordinator.Start()

	/* to keep the main goroutine alive */
	select {}
}

func parse_configs() error {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(file, &serverContext.Config); err != nil {
		return err
	}

	return nil
}

func configGetEnv() {
	// "host=postgres port=5432 user=coordinator_user password=coordinator_password dbname=coordinator_db sslmode=disable"
	serverContext.Config.DbConnStr = "host=" + os.Getenv("DB_HOST") +
		" port=" + os.Getenv("DB_PORT") +
		" user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" sslmode=disable"
	serverContext.Config.RabbitMqConnStr = os.Getenv("RABBITMQ_URL")
	serverContext.Config.ExchangeName = os.Getenv("EXCHANGE_NAME")
}

func connectCoordinatorToRabbit() (*prabbitmq.RabbitMQManager, error) {
	RabbitMQ :=
		prabbitmq.NewRabbitMQManager(
			serverContext.Config.RabbitMqConnStr,
			serverContext.Config.RabbitHBDelay,
			serverContext.Config.RabbitConnTimeout,
			serverContext.Config.RabbitConnRetryDelay,
			serverContext.Config.ExchangeName)
	return RabbitMQ, RabbitMQ.ConnectAndMonitor()
}

func registerHttpHandlers() {
	http.HandleFunc("/api/hash/status", GetRequestStatusHandler(serverContext.Coordinator))
	http.HandleFunc("/api/hash/crack", SubmitRequestCrackHandler(serverContext.Coordinator))
	http.HandleFunc("/internal/api/worker/register", RegisterNewWorkerHandler(serverContext.Coordinator))
	http.HandleFunc("/internal/api/task/result", GetTaskResultHandler(serverContext.Coordinator))
}

func listenOnPort() {
	log.Printf("server is listening on port %s\n", serverContext.Config.Port)
	if err := http.ListenAndServe(":"+serverContext.Config.Port, nil); err != nil {
		log.Printf("error while starting server: %s\n", err)
		return
	}
}
