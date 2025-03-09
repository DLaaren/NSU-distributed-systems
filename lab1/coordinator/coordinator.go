package coordinator

import (
	"github.com/google/uuid"
	"lab1/worker"
	"log"
)

// default queues sizes

type Coordinator struct {
	UserRequests  []UserRequest
	Workers       []worker.Worker
	RequestsQueue chan UserRequest
	TasksQueue    chan worker.WorkerTask
	TaskTracker   map[UserRequestId]worker.WorkerTask
	FailureChan   chan string
}

// // put it in main.c
// // Start listening server to get users' requests
// func StartServer() {

// }

/* Init Coordinator instance */
func NewCoordinator() *Coordinator {
	log.SetPrefix("[Coordintor]: ")
	log.Println("coordinator was created")
	log.SetPrefix("[Server]: ")
	return &Coordinator{
		UserRequests:  make([]UserRequest, 0),
		Workers:       make([]worker.Worker, 0),
		RequestsQueue: make(chan UserRequest, 0),
		TasksQueue:    make(chan worker.WorkerTask, 0),
		TaskTracker:   make(map[UserRequestId]worker.WorkerTask),
		FailureChan:   make(chan string),
	}
}

func (c *Coordinator) Status(request *UserRequest) {

}

// creates task and map it to workers
func (c *Coordinator) Crack(request *UserRequest) UserRequestId {
	id := uuid.New()
	return UserRequestId(id.ID())
}

func (c *Coordinator) TaskLaunch() {

}

func (c *Coordinator) TaskStatuc() {

}

func (c *Coordinator) TaskKill() {

}

func (c *Coordinator) Map() {

}

func (c *Coordinator) Reduce() {

}
