package prabbitmq

import (
	"context"
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
	ExchangeName    string
	conn            *amqp.Connection
	Channel         *amqp.Channel
	notifyConnClose chan *amqp.Error
	mutex           sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
}

func NewRabbitMQManager(connStr string, exchangeName string) *RabbitMQManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &RabbitMQManager{
		connStr:      connStr,
		ExchangeName: exchangeName,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (r *RabbitMQManager) Connect() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Close existing connection if any
	if r.conn != nil {
		r.conn.Close()
	}

	config := amqp.Config{
		Heartbeat: 10 * time.Second,
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, time.Minute)
		},
	}

	conn, err := amqp.DialConfig(r.connStr, config)
	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}

	r.conn = conn
	r.notifyConnClose = r.conn.NotifyClose(make(chan *amqp.Error))

	// Create initial channel
	if err := r.createChannel(); err != nil {
		return fmt.Errorf("failed to create channel: %v", err)
	}

	// Start connection monitor
	go r.monitorConnection()

	return nil
}

func (r *RabbitMQManager) createChannel() error {
	// Close existing channel if any
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

	select {
	case err := <-r.notifyConnClose:
		if err != nil {
			log.Printf("RabbitMQ connection closed: %v", err)
		}

		// Attempt reconnection
		for {
			select {
			case <-r.ctx.Done():
				return
			default:
				if err := r.Connect(); err == nil {
					log.Println("Successfully reconnected to RabbitMQ")
					return
				}
				time.Sleep(time.Second * 20)
			}
		}
	case <-r.ctx.Done():
		return
	}
}
