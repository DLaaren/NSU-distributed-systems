package test

import (
	"lab1/coordinator"
	"lab1/shared"
)

type MockCoordinator struct{}

func (m *MockCoordinator) Crack(request *coordinator.UserRequest) shared.UserRequestId {
	return shared.UserRequestId(123)
}

func (m *MockCoordinator) GetUserRequestStatus(id shared.UserRequestId) coordinator.UserStatusResponse {
	return coordinator.UserStatusResponse{
		Status: coordinator.READY,
		Result: "aboba",
	}
}

func (m *MockCoordinator) RegisterWorker(worker *coordinator.Worker) {}

func (m *MockCoordinator) CheckWorkers() {}

func (m *MockCoordinator) UpdateTask(task *shared.WorkerTask) {}
