package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"lab2/shared"
	ptask "lab2/task"
	pworker "lab2/worker"

	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

type ServerContext struct {
	Port               string
	CoordinatorAddress string
	RabbitMqConnStr    string
	Channel            *amqp.Channel
	ExchangeName       string
	TasksQueue         amqp.Queue
	MyWorkerId         shared.WorkerId
	RetryConnectDelay  time.Duration
	MaxRetries         int
	MaxParallelTasks   int
	Status             pworker.WorkerStatus
	Tasks              []*ptask.Task
	RWmutex            sync.RWMutex
}

type Config struct {
	Port              string        `yaml:"port"`
	RetryConnectDelay time.Duration `yaml:"retry_connect_delay"`
	MaxRetries        int           `yaml:"max_retries"`
	MaxParallelTasks  int           `yaml:"max_parallel_tasks"`
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
	wstatus := pworker.WorkerStatusResponse{Status: server_context.Status, Port: server_context.Port}

	for attempt := 1; attempt <= server_context.MaxRetries; attempt++ {
		if err := json.NewEncoder(&buf).Encode(wstatus); err != nil {
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
			if err := json.NewDecoder(resp.Body).Decode(&server_context.MyWorkerId); err != nil {
				log.Println("failed to register worker:", resp.Status)
			} else {
				return nil
			}
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
	server_context.CoordinatorAddress = os.Getenv("COORDINATOR_ADDR")
	server_context.RabbitMqConnStr = os.Getenv("RABBITMQ_URL")
	server_context.ExchangeName = os.Getenv("EXCHANGE_NAME")
	server_context.RetryConnectDelay = config.RetryConnectDelay
	server_context.MaxRetries = config.MaxRetries
	server_context.MaxParallelTasks = config.MaxParallelTasks

	/* Connect to rabbitmq */
	rabbitmq, err := amqp.Dial(server_context.RabbitMqConnStr)
	if err != nil {
		log.Fatalf("cannot connect to RabbitMQ server: %s\n", err)
	}

	if err := register_worker(); err != nil {
		log.Fatalf("failed to register worker after retries: %s\n", err)
	}
	log.Printf("register worker sucessfully, my id = %d", server_context.MyWorkerId)

	/* init zone */
	server_context.Status = pworker.ALIVE
	channel, err := rabbitmq.Channel()
	if err != nil {
		log.Fatalf("failed to open rabbitmq channel: %s\n", err)
	}

	server_context.Channel = channel

	err = channel.ExchangeDeclare(
		server_context.ExchangeName,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	server_context.TasksQueue, err = channel.QueueDeclare(
		"worker_queue_"+strconv.FormatUint(uint64(server_context.MyWorkerId), 10),
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("failed to declare rabbitmq queue: %v", err)
	}

	err = channel.Qos(
		server_context.MaxParallelTasks,
		0,     // the total size of unack msgs per consumer
		false, // whether th QoS limits applies to all comsumers within the channel or only to this one
	)
	if err != nil {
		log.Fatalf("Failed to set QoS: %v", err)
	}

	/* bind queue to exchange with worker ID as routing key */
	err = channel.QueueBind(
		server_context.TasksQueue.Name,
		server_context.TasksQueue.Name,
		server_context.ExchangeName,
		false,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	messages, err := channel.Consume(
		server_context.TasksQueue.Name,
		"",
		false,
		false,
		false,
		false,
		nil)
	if err != nil {
		log.Fatalf("failed to register a consumer: %v", err)
	}

	http.HandleFunc("/internal/api/worker/status", GetWorkerStatusHandler(&server_context))
	http.HandleFunc("/internal/api/worker/crack", SubmitTaskHandler(&server_context))
	http.HandleFunc("/internal/api/worker/kill", KillTaskHandler(&server_context))
	http.HandleFunc("/internal/api/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alive"))
	})

	log.Println("all handlers were set up")

	go func() {
		for message := range messages {
			tag, ok := message.Headers["tag"]
			if !ok {
				log.Printf("Message missing 'tag' header")
				message.Nack(false, false) // Discard message
				continue
			}

			tagStr, ok := tag.(string)
			if !ok {
				log.Printf("Tag header is not a string")
				message.Nack(false, false)
				continue
			}

			var task ptask.Task
			if err := json.Unmarshal(message.Body, &task); err != nil {
				log.Printf("Failed to decode task: %v", err)
				message.Nack(false, false) // Discard message
				return
			}

			switch tagStr {
			case "submit":
				go func() {
					err := SubmitTask(&server_context, &task)
					if err != nil {
						log.Printf("Failed to submit task: %v", err)
						message.Nack(false, true) // Requeue on temporary failure
						return
					}
					message.Ack(false)
				}()
			case "kill":
				go func() {
					err := KillTask(&server_context, &task)
					if err != nil {
						log.Printf("Failed to kill task: %v", err)
						// Don't requeue kill commands
						message.Nack(false, false)
						return
					}
					message.Ack(false)
				}()
			default:
				log.Printf("Unknown message tag: %s", tagStr)
				message.Nack(false, false) // Discard unknown message types
				return
			}
		}
	}()

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
