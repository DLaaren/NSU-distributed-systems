package test

import (
	"encoding/json"
	"lab1/coordinator"
	"lab1/shared"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewCoordinator(t *testing.T) {
	coord := coordinator.NewCoordinator()
	assert.NotNil(t, coord)
	assert.Empty(t, coord.UserRequests)
	assert.Empty(t, coord.Workers)
}

func TestGetUserRequestStatus(t *testing.T) {
	coord := coordinator.NewCoordinator()
	requestId := shared.UserRequestId(uuid.New().ID())
	coord.UserRequests[requestId] = &coordinator.UserRequest{
		Id:     requestId,
		Status: coordinator.PROCESSING,
		Result: "result",
	}

	response := coord.GetUserRequestStatus(requestId)
	assert.Equal(t, coordinator.PROCESSING, response.Status)
	assert.Equal(t, "result", response.Result)
}

func TestCrack(t *testing.T) {
	coord := coordinator.NewCoordinator()
	request := &coordinator.UserRequest{
		Hash:      "hash",
		MaxLength: 5,
	}

	requestId := coord.Crack(request)
	assert.NotEqual(t, shared.UserRequestId(0), requestId)
	assert.Equal(t, coordinator.PROCESSING, coord.UserRequests[requestId].Status)
}

func TestRegisterWorker(t *testing.T) {
	coord := coordinator.NewCoordinator()
	worker := &coordinator.Worker{
		Address: "127.0.0.1:8080",
		Status:  shared.IDLE,
		LastHB:  time.Now(),
	}

	coord.RegisterWorker(worker)
	assert.NotEqual(t, shared.WorkerId(0), worker.Id)
	assert.Equal(t, worker, coord.Workers[worker.Id])
}

func TestDeleteWorker(t *testing.T) {
	coord := coordinator.NewCoordinator()
	worker := &coordinator.Worker{
		Id:      shared.WorkerId(uuid.New().ID()),
		Address: "127.0.0.1:8080",
		Status:  shared.IDLE,
		LastHB:  time.Now(),
	}

	coord.Workers[worker.Id] = worker
	coord.DeleteWorker(worker)
	assert.Nil(t, coord.Workers[worker.Id])
}

func TestTaskLaunch(t *testing.T) {
	coord := coordinator.NewCoordinator()
	worker := &coordinator.Worker{
		Id:      shared.WorkerId(uuid.New().ID()),
		Address: "127.0.0.1:8080",
		Status:  shared.IDLE,
		LastHB:  time.Now(),
	}

	coord.Workers[worker.Id] = worker

	// Mock HTTP server to simulate task launch
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	worker.Address = server.URL[len("http://"):]
	task := &shared.WorkerTask{
		Id:        shared.TaskId(uuid.New().ID()),
		RequestId: shared.UserRequestId(uuid.New().ID()),
		Hash:      "hash",
		MaxLength: 5,
		Status:    shared.IN_PROGRESS,
	}

	success, err := coord.TaskLaunch(worker, task)
	assert.True(t, success)
	assert.Nil(t, err)
	assert.Equal(t, worker.Id, task.WorkerId)
}

func TestTaskStatus(t *testing.T) {
	coord := coordinator.NewCoordinator()
	worker := &coordinator.Worker{
		Id:      shared.WorkerId(uuid.New().ID()),
		Address: "127.0.0.1:8080",
		Status:  shared.IDLE,
		LastHB:  time.Now(),
	}

	coord.Workers[worker.Id] = worker

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusResponse := shared.TaskStatusResponse{
			Status: shared.DONE_SUCCESS,
		}
		json.NewEncoder(w).Encode(statusResponse)
	}))
	defer server.Close()

	worker.Address = server.URL[len("http://"):]
	task := &shared.WorkerTask{
		Id:        shared.TaskId(uuid.New().ID()),
		RequestId: shared.UserRequestId(uuid.New().ID()),
		Hash:      "hash",
		MaxLength: 5,
		Status:    shared.IN_PROGRESS,
		WorkerId:  worker.Id,
	}

	status := coord.TaskStatus(task)
	assert.Equal(t, shared.DONE_SUCCESS, status)
}

func TestTaskKill(t *testing.T) {
	coord := coordinator.NewCoordinator()
	worker := &coordinator.Worker{
		Id:      shared.WorkerId(uuid.New().ID()),
		Address: "127.0.0.1:8080",
		Status:  shared.IDLE,
		LastHB:  time.Now(),
	}

	coord.Workers[worker.Id] = worker

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	worker.Address = server.URL[len("http://"):]
	task := &shared.WorkerTask{
		Id:        shared.TaskId(uuid.New().ID()),
		RequestId: shared.UserRequestId(uuid.New().ID()),
		Hash:      "hash",
		MaxLength: 5,
		Status:    shared.IN_PROGRESS,
		WorkerId:  worker.Id,
	}

	err := coord.TaskKill(task)
	assert.Nil(t, err)
	assert.Equal(t, shared.KILLED, task.Status)
}

func TestUpdateTask(t *testing.T) {
	coord := coordinator.NewCoordinator()
	requestId := shared.UserRequestId(uuid.New().ID())
	task := &shared.WorkerTask{
		Id:        shared.TaskId(uuid.New().ID()),
		RequestId: requestId,
		Hash:      "hash",
		MaxLength: 5,
		Status:    shared.IN_PROGRESS,
		Result:    "result",
	}

	coord.UserRequests[requestId] = &coordinator.UserRequest{
		Id:             requestId,
		Status:         coordinator.PROCESSING,
		TasksDone:      0,
		TasksScheduled: 1,
	}
	coord.UserRequestsToTasks[requestId] = append(coord.UserRequestsToTasks[requestId], task)

	coord.UpdateTask(task)
	assert.Equal(t, 1, coord.UserRequests[requestId].TasksDone)
	assert.Equal(t, coordinator.READY, coord.UserRequests[requestId].Status)
	assert.Equal(t, "result", coord.UserRequests[requestId].Result)
}
