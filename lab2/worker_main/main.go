package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	ExchangeName       string
	Channel            *amqp.Channel
	Messages           <-chan amqp.Delivery
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

var worker_context ServerContext
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
	wstatus := pworker.WorkerStatusResponse{Status: worker_context.Status, Port: worker_context.Port}

	for attempt := 1; attempt <= worker_context.MaxRetries; attempt++ {
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
		if attempt < worker_context.MaxRetries {
			log.Println("try again after delay")
			time.Sleep(worker_context.RetryConnectDelay)
		}
	}

	return errors.New("failed to register worker")
}

func connectToRabbit(worker_context *ServerContext) error {
	worker_context.RWmutex.RLock()
	connstr := worker_context.RabbitMqConnStr
	exchangeName := worker_context.ExchangeName
	workerId := worker_context.MyWorkerId
	maxParallelTasks := worker_context.MaxParallelTasks
	worker_context.RWmutex.RUnlock()

	rabbitmq, err := amqp.Dial(connstr)
	if err != nil {
		return fmt.Errorf("cannot connect to RabbitMQ server: %s", err)
	}

	channel, err := rabbitmq.Channel()
	if err != nil {
		return fmt.Errorf("failed to open rabbitmq channel: %s", err)
	}

	err = channel.ExchangeDeclare(
		exchangeName,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	tasksQueue, err := channel.QueueDeclare(
		"worker_queue_"+strconv.FormatUint(uint64(workerId), 10),
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare rabbitmq queue: %v", err)
	}

	err = channel.Qos(
		maxParallelTasks,
		0,     // the total size of unack msgs per consumer
		false, // whether th QoS limits applies to all comsumers within the channel or only to this one
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %v", err)
	}

	/* bind queue to exchange with worker ID as routing key */
	err = channel.QueueBind(
		tasksQueue.Name,
		tasksQueue.Name,
		exchangeName,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	messages, err := channel.Consume(
		tasksQueue.Name,
		"",
		false,
		false,
		false,
		false,
		nil)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %v", err)
	}

	worker_context.RWmutex.Lock()
	worker_context.Channel = channel
	worker_context.Messages = messages
	worker_context.RWmutex.Unlock()

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

	worker_context.Port = config.Port
	worker_context.CoordinatorAddress = os.Getenv("COORDINATOR_ADDR")
	worker_context.RabbitMqConnStr = os.Getenv("RABBITMQ_URL")
	worker_context.ExchangeName = os.Getenv("EXCHANGE_NAME")
	worker_context.RetryConnectDelay = config.RetryConnectDelay
	worker_context.MaxRetries = config.MaxRetries
	worker_context.MaxParallelTasks = config.MaxParallelTasks

	worker_context.Status = pworker.ALIVE
	if err := register_worker(); err != nil {
		log.Fatalf("failed to register worker after retries: %s\n", err)
	}
	log.Printf("register worker sucessfully, my id = %d", worker_context.MyWorkerId)

	err := connectToRabbit(&worker_context)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %s\n", err)
	}

	http.HandleFunc("/internal/api/worker/status", GetWorkerStatusHandler(&worker_context))
	http.HandleFunc("/internal/api/worker/crack", SubmitTaskHandler(&worker_context))
	http.HandleFunc("/internal/api/worker/kill", KillTaskHandler(&worker_context))
	http.HandleFunc("/internal/api/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alive"))
	})

	log.Println("all handlers were set up")

	/* Constantly check if RabbitMQ is alive */
	go func() {
		for {
			time.Sleep(worker_context.RetryConnectDelay)

			conn, err := amqp.Dial(worker_context.RabbitMqConnStr)
			if err != nil {
				err = connectToRabbit(&worker_context)
				if err != nil {
					continue
				}
			}

			ch, err := conn.Channel()
			if err != nil {
				err = connectToRabbit(&worker_context)
				if err != nil {
					continue
				}
			}

			conn.Close()
			ch.Close()
		}
	}()

	go func() {
		for {
			worker_context.RWmutex.RLock()
			messages := worker_context.Messages
			worker_context.RWmutex.RUnlock()

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
					continue
				}

				switch tagStr {
				case "submit":
					go func() {
						err := SubmitTask(&worker_context, &task)
						if err != nil {
							isConnErr := checkConnectionError(err)
							if isConnErr {
								log.Printf("Waiting for reconnection to RabbitMq")
								message.Nack(false, true)
								return
							}

							log.Printf("Failed to submit task: %v", err)
							message.Nack(false, true) // Requeue on temporary failure
							return
						}
						message.Ack(false)
					}()
				case "kill":
					go func() {
						err := KillTask(&worker_context, &task)
						if err != nil {
							isConnErr := checkConnectionError(err)
							if isConnErr {
								log.Printf("Waiting for reconnection to RabbitMq")
								message.Nack(false, true)
								return
							}

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
					continue
				}
			}
		}
	}()

	go func() {
		worker_context.RWmutex.RLock()
		port := worker_context.Port
		worker_context.RWmutex.RUnlock()

		log.Println("server is listening on port", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Println("error while starting server:", err)
			return
		}
	}()

	/* to keep the main goroutine alive */
	select {}
}

func checkConnectionError(err error) bool {
	if err == nil {
		return false
	}

	if amqpErr, ok := err.(*amqp.Error); ok {
		if amqpErr.Code == amqp.ChannelError {
			log.Printf("RabbitMQ channel/connection error (code %d): %v", amqpErr.Code, amqpErr.Reason)
			return true
		}
	}

	return false
}
