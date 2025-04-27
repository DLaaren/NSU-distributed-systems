package prabbitmq

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQManager struct {
	connStr         string
	HBdelay         time.Duration
	ConnTimeout     time.Duration
	ConnRetryDelay  time.Duration
	ExchangeName    string
	conn            *amqp.Connection
	Channel         *amqp.Channel
	notifyConnClose chan *amqp.Error
	mutex           sync.RWMutex
}

func NewRabbitMQManager(connStr string, hbdelay time.Duration, connTimeout time.Duration, connRetryDelay time.Duration, exchangeName string) *RabbitMQManager {
	return &RabbitMQManager{
		connStr:        connStr,
		HBdelay:        hbdelay,
		ConnTimeout:    connTimeout,
		ConnRetryDelay: connRetryDelay,
		ExchangeName:   exchangeName,
	}
}

func (r *RabbitMQManager) ConnectAndMonitor() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	/* close existing connection if any */
	if r.conn != nil {
		r.conn.Close()
	}

	config := amqp.Config{
		Heartbeat: r.HBdelay,
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, r.ConnTimeout)
		},
	}

	conn, err := amqp.DialConfig(r.connStr, config)
	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}

	r.conn = conn
	r.notifyConnClose = r.conn.NotifyClose(make(chan *amqp.Error))

	if err := r.createChannel(); err != nil {
		return fmt.Errorf("failed to create channel: %v", err)
	}

	go r.monitorConnection()

	return nil
}

func (r *RabbitMQManager) createChannel() error {
	/* close existing channel if any */
	if r.Channel != nil {
		r.Channel.Close()
	}

	ch, err := r.conn.Channel()
	if err != nil {
		return err
	}

	r.Channel = ch

	if err := ch.ExchangeDeclare(
		r.ExchangeName,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return fmt.Errorf("failed to declare exchange: %v", err)
	}

	return nil
}

func (r *RabbitMQManager) GetChannel() (*amqp.Channel, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.Channel == nil || r.Channel.IsClosed() {
		return nil, errors.New("no active channel")
	}
	return r.Channel, nil
}

func (r *RabbitMQManager) monitorConnection() {
	err := <-r.notifyConnClose
	if err != nil {
		log.Printf("RabbitMQ connection closed: %v", err)
	}

	/* attempt reconnection */
	for {
		if err := r.ConnectAndMonitor(); err == nil {
			log.Println("Successfully reconnected to RabbitMQ")
			return /* ConnectAndMonitor wil call monitorConnection again */
		}
		time.Sleep(r.ConnRetryDelay)
	}
}
