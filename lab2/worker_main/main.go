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

	prabbitmq "lab2/rabbitmq"
	"lab2/shared"
	ptask "lab2/task"
	pworker "lab2/worker"

	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

type ServerContext struct {
	RabbitMQ           *prabbitmq.RabbitMQManager
	CoordinatorAddress string
	RabbitMqConnStr    string
	ExchangeName       string
	MyWorkerId         shared.WorkerId
	Config             Config
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

var worker_context ServerContext

func parse_configs() error {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(file, &worker_context.Config); err != nil {
		return err
	}

	return nil
}

func register_worker() error {
	var buf bytes.Buffer
	wstatus := pworker.WorkerStatusResponse{Status: worker_context.Status, Port: worker_context.Config.Port}

	for attempt := 1; attempt <= worker_context.Config.MaxRetries; attempt++ {
		if err := json.NewEncoder(&buf).Encode(wstatus); err != nil {
			return err
		}

		resp, err := http.Post(
			"http://"+worker_context.CoordinatorAddress+"/internal/api/worker/register",
			"application/json",
			&buf)
		if err != nil {
			return err
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			if err := json.NewDecoder(resp.Body).Decode(&worker_context.MyWorkerId); err != nil {
				log.Println("failed to register worker:", resp.Status)
			} else {
				return nil
			}
		} else {
			log.Println("failed to register worker:", resp.Status)
		}
		if attempt < worker_context.Config.MaxRetries {
			log.Println("try again after delay")
			time.Sleep(worker_context.Config.RetryConnectDelay)
		}
	}

	return errors.New("failed to register worker")
}

func consumeTasks() {
consumeLoop:
	for {
		ch, err := worker_context.RabbitMQ.GetChannel()
		if err != nil {
			log.Printf("Failed to get channel, retrying: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Create a channel to detect connection closures
		notifyClose := make(chan *amqp.Error)
		ch.NotifyClose(notifyClose)

		q, err := ch.QueueDeclare(
			"worker_queue_"+strconv.FormatUint(uint64(worker_context.MyWorkerId), 10),
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.Printf("Failed to declare queue, retrying: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		err = ch.Qos(
			worker_context.Config.MaxParallelTasks,
			0,
			false,
		)
		if err != nil {
			log.Printf("Failed to declare qos, retrying: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		err = ch.QueueBind(
			q.Name,
			q.Name,
			worker_context.ExchangeName,
			false,
			nil,
		)
		if err != nil {
			log.Printf("Failed to bind queue, retrying: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		messages, err := ch.Consume(
			q.Name,
			"",
			false,
			false,
			false,
			false,
			nil)
		if err != nil {
			log.Printf("Failed to define consumer, retrying: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for {
			select {
			case err := <-notifyClose:
				if err != nil {
					log.Printf("RabbitMQ channel/connection closed: %v", err)
				}
				// Break out of the consume loop to restart everything
				goto consumeLoop

			case message, ok := <-messages:
				if !ok {
					// Channel closed
					log.Println("Message channel closed, reconnecting...")
					goto consumeLoop
				}

				tag, ok := message.Headers["tag"]
				if !ok {
					log.Printf("Message missing 'tag' header")
					message.Nack(false, false)
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
					message.Nack(false, false)
					continue
				}

				switch tagStr {
				case "submit":
					go func() {
						err := SubmitTask(&worker_context, &task)
						if err != nil {
							log.Printf("Failed to submit task: %v", err)
							message.Nack(false, true)
							return
						}
						message.Ack(false)
					}()
				case "kill":
					go func() {
						err := KillTask(&worker_context, &task)
						if err != nil {
							log.Printf("Failed to kill task: %v", err)
							message.Nack(false, false)
							return
						}
						message.Ack(false)
					}()
				default:
					log.Printf("Unknown message tag: %s", tagStr)
					message.Nack(false, false)
				}
			}
		}
	}
}

func main() {
	log.SetPrefix("[Server]: ")

	/* parse configs */
	if err := parse_configs(); err != nil {
		log.Println("error while parsing config file:", err)
		return
	}
	log.Println("configs were parsed sucessfully")

	worker_context.CoordinatorAddress = os.Getenv("COORDINATOR_ADDR")
	worker_context.RabbitMqConnStr = os.Getenv("RABBITMQ_URL")
	worker_context.ExchangeName = os.Getenv("EXCHANGE_NAME")

	worker_context.Status = pworker.ALIVE
	if err := register_worker(); err != nil {
		log.Fatalf("failed to register worker after retries: %s\n", err)
	}
	log.Printf("register worker sucessfully, my id = %d", worker_context.MyWorkerId)

	/* Connect to RabbitMq */
	worker_context.RabbitMQ = prabbitmq.NewRabbitMQManager(worker_context.RabbitMqConnStr, worker_context.ExchangeName)
	err := worker_context.RabbitMQ.Connect()
	if err != nil {
		log.Fatalf("cannot connecgt to RabbitMq: %s\n", err)
	}

	http.HandleFunc("/internal/api/worker/status", GetWorkerStatusHandler(&worker_context))
	http.HandleFunc("/internal/api/worker/crack", SubmitTaskHandler(&worker_context))
	http.HandleFunc("/internal/api/worker/kill", KillTaskHandler(&worker_context))
	http.HandleFunc("/internal/api/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alive"))
	})

	log.Println("all handlers were set up")

	go consumeTasks()

	go func() {
		log.Println("server is listening on port", worker_context.Config.Port)
		if err := http.ListenAndServe(":"+worker_context.Config.Port, nil); err != nil {
			log.Println("error while starting server:", err)
			return
		}
	}()

	/* to keep the main goroutine alive */
	select {}
}
