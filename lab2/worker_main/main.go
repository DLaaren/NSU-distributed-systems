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

type WorkerContext struct {
	RabbitMQ   *prabbitmq.RabbitMQManager
	Config     Config
	MyWorkerId shared.WorkerId
	Status     pworker.WorkerStatus
	Tasks      []*ptask.Task
	RWmutex    sync.RWMutex
}

type Config struct {
	Port                 string        `yaml:"port"`
	RetryConnectDelay    time.Duration `yaml:"retry_connect_delay"`
	MaxRetries           int           `yaml:"max_retries"`
	MaxParallelTasks     int           `yaml:"max_parallel_tasks"`
	RabbitHBDelay        time.Duration `yaml:"rabbit_hb_delay"`
	RabbitConnTimeout    time.Duration `yaml:"rabbit_conn_timeout"`
	RabbitConnRetryDelay time.Duration `yaml:"rabbit_conn_retry_delay"`
	RabbitMqConnStr      string
	ExchangeName         string
	CoordinatorAddress   string
}

var workerContext WorkerContext

func main() {
	log.SetPrefix("[Worker]: ")

	/* parse configs */
	if err := parse_configs(); err != nil {
		log.Printf("error while parsing config file: %v", err)
		return
	}
	log.Printf("configs were parsed sucessfully\n")

	/* get configs from envs */
	configGetEnv()

	/* register worker sending coordinator a request and get my worker id */
	workerContext.Status = pworker.ALIVE
	if err := register_worker(); err != nil {
		log.Fatalf("failed to register worker after retries: %s", err)
	}
	log.Printf("register worker sucessfully, my id = %d", workerContext.MyWorkerId)

	/* Connect to RabbitMq */
	err := connectWorkerToRabbit()
	if err != nil {
		log.Fatalf("failed to connect to RabbitMq: %s", err)
	}
	log.Printf("connect to RabbitMq\n")

	registerHttpHandlers()
	log.Println("all handlers were set up")

	/* start consumer waiting for tasks from queue */
	go consumeTasks()

	/* listen on port for coordinator's requests */
	go listenOnPort()

	/* to keep the main goroutine alive */
	select {}
}

func configGetEnv() {
	workerContext.Config.CoordinatorAddress = os.Getenv("COORDINATOR_ADDR")
	workerContext.Config.RabbitMqConnStr = os.Getenv("RABBITMQ_URL")
	workerContext.Config.ExchangeName = os.Getenv("EXCHANGE_NAME")
}

func parse_configs() error {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(file, &workerContext.Config); err != nil {
		return err
	}

	return nil
}

func register_worker() error {
	var buf bytes.Buffer
	wstatus := pworker.WorkerStatusResponse{Status: workerContext.Status, Port: workerContext.Config.Port}

	for attempt := 1; attempt <= workerContext.Config.MaxRetries; attempt++ {
		if err := json.NewEncoder(&buf).Encode(wstatus); err != nil {
			return err
		}

		resp, err := http.Post(
			"http://"+workerContext.Config.CoordinatorAddress+"/internal/api/worker/register",
			"application/json",
			&buf)
		if err != nil {
			return err
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			if err := json.NewDecoder(resp.Body).Decode(&workerContext.MyWorkerId); err != nil {
				log.Println("failed to register worker:", resp.Status)
			} else {
				return nil
			}
		} else {
			log.Println("failed to register worker:", resp.Status)
		}
		if attempt < workerContext.Config.MaxRetries {
			log.Println("try again after delay")
			time.Sleep(workerContext.Config.RetryConnectDelay)
		}
	}

	return errors.New("failed to register worker")
}

func connectWorkerToRabbit() error {
	workerContext.RabbitMQ =
		prabbitmq.NewRabbitMQManager(
			workerContext.Config.RabbitMqConnStr,
			workerContext.Config.RabbitHBDelay,
			workerContext.Config.RabbitConnTimeout,
			workerContext.Config.RetryConnectDelay,
			workerContext.Config.ExchangeName)

	return workerContext.RabbitMQ.ConnectAndMonitor()
}

func registerHttpHandlers() {
	http.HandleFunc("/internal/api/worker/status", GetWorkerStatusHandler(&workerContext))
	http.HandleFunc("/internal/api/worker/crack", SubmitTaskHandler(&workerContext))
	http.HandleFunc("/internal/api/worker/kill", KillTaskHandler(&workerContext))
	http.HandleFunc("/internal/api/worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alive"))
	})
}

func consumeTasks() {
retryConn:
	for {
		ch, err := workerContext.RabbitMQ.GetChannel()
		if err != nil {
			log.Printf("Failed to get channel, retrying after delay: %v", err)
			time.Sleep(workerContext.Config.RabbitConnRetryDelay)
			continue
		}

		/* Create a channel to detect connection closures for consumer */
		notifyCloseConsumer := ch.NotifyClose(make(chan *amqp.Error))

		q, err := ch.QueueDeclare(
			"worker_queue_"+strconv.FormatUint(uint64(workerContext.MyWorkerId), 10),
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.Printf("Failed to declare queue, retrying after delay: %v", err)
			time.Sleep(workerContext.Config.RabbitConnRetryDelay)
			continue
		}

		err = ch.Qos(
			workerContext.Config.MaxParallelTasks,
			0,
			false,
		)
		if err != nil {
			log.Printf("Failed to declare qos, retrying after delay: %v", err)
			time.Sleep(workerContext.Config.RabbitConnRetryDelay)
			continue
		}

		err = ch.QueueBind(
			q.Name,
			q.Name,
			workerContext.RabbitMQ.ExchangeName,
			false,
			nil,
		)
		if err != nil {
			log.Printf("Failed to bind queue, retrying after delay: %v", err)
			time.Sleep(workerContext.Config.RabbitConnRetryDelay)
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
			log.Printf("Failed to define consumer, retrying after delay: %v", err)
			time.Sleep(workerContext.Config.RabbitConnRetryDelay)
			continue
		}

		for {
			select {
			case err := <-notifyCloseConsumer:
				log.Printf("RabbitMQ channel/connection closed: %v", err)
				time.Sleep(workerContext.Config.RabbitConnRetryDelay)
				/* connection is detected, start over again */
				goto retryConn

			case message, ok := <-messages:
				if !ok {
					/* channel closed */
					log.Printf("RabbitMQ channel/connection closed")
					time.Sleep(workerContext.Config.RabbitConnRetryDelay)
					/* connection is detected, start over again */
					goto retryConn
				}

				tag, ok := getMsgTag(&message)
				if !ok {
					time.Sleep(workerContext.Config.RabbitConnRetryDelay)
					/* connection is detected, start over again */
					goto retryConn
				}

				/* parse task from msg */
				var task ptask.Task
				if err := json.Unmarshal(message.Body, &task); err != nil {
					log.Printf("Failed to decode task: %v", err)
					message.Nack(false, false)
					continue
				}

				switch tag {
				case "submit":
					go func() {
						err := ConsumeTask(&workerContext, &task)
						if err != nil {
							log.Printf("Failed to submit task: %v", err)
							message.Nack(false, true)
							return
						}
						message.Ack(false)
					}()

				case "kill":
					go func() {
						err := KillTask(&workerContext, &task)
						if err != nil {
							log.Printf("Failed to kill task: %v", err)
							message.Nack(false, false)
							return
						}
						message.Ack(false)
					}()

				default:
					log.Printf("Unknown message tag: %s", tag)
					message.Nack(false, false)
				}
			}
		}
	}
}

func getMsgTag(message *amqp.Delivery) (string, bool) {
	tag, ok := message.Headers["tag"]
	if !ok {
		log.Printf("Message missing 'tag' header")
		message.Nack(false, false)
		return "", ok
	}

	tagStr, ok := tag.(string)
	if !ok {
		log.Printf("Tag header is not a string")
		message.Nack(false, false)
		return "", ok
	}

	return tagStr, true
}

func listenOnPort() {
	log.Println("server is listening on port", workerContext.Config.Port)
	if err := http.ListenAndServe(":"+workerContext.Config.Port, nil); err != nil {
		log.Println("error while starting server:", err)
		return
	}
}
