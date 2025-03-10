package coordinator

import (
	"lab1/worker"
	"log"
	"sync"

	"github.com/google/uuid"
)

// default queues sizes

type Coordinator struct {
	UserRequests map[UserRequestId]*UserRequest
	Workers      map[string]*worker.Worker

	rwmu sync.RWMutex
}

/* Init Coordinator instance */
func NewCoordinator() *Coordinator {
	log.SetPrefix("[Coordintor]: ")
	log.Println("coordinator was created")
	log.SetPrefix("[Server]: ")
	return &Coordinator{
		UserRequests: make(map[UserRequestId]*UserRequest, 0),
		Workers:      make(map[string]*worker.Worker, 0),
	}
}

func (c *Coordinator) RegisterWorker(worker *worker.Worker) {
	c.rwmu.Lock()
	defer c.rwmu.Unlock()

	c.Workers[worker.Address] = worker
}

// creates task and map it to workers
func (c *Coordinator) Crack(request *UserRequest) UserRequestId {
	request.RequestId = UserRequestId(uuid.New().ID())
	request.Status = IN_PROGRESS

	c.rwmu.Lock()
	defer c.rwmu.Unlock()

	c.UserRequests[request.RequestId] = request

	go func() {

	}()

	return request.RequestId
}

func (c *Coordinator) UserRequestStatus(requestId UserRequestId) UserStatusResponse {
	c.rwmu.RLock()
	defer c.rwmu.RUnlock()

	userRequest := c.UserRequests[requestId]

	return UserStatusResponse{
		Status: userRequest.Status,
		Result: userRequest.Result,
	}
}

func (c *Coordinator) TaskLaunch() {

}

func (c *Coordinator) TaskStatus() {

}

func (c *Coordinator) TaskKill() {

}

func (c *Coordinator) Map() {

}

func (c *Coordinator) Reduce() {

}
