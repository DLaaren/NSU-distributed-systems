package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"lab1/coordinator"
	"lab1/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGetRequestStatusHandler(t *testing.T) {
	coord := coordinator.NewCoordinator()
	handler := coordinator.GetRequestStatusHandler(coord)

	requestId := shared.UserRequestId(uuid.New().ID())
	coord.UserRequests[requestId] = &coordinator.UserRequest{
		Id:     requestId,
		Status: coordinator.PROCESSING,
		Result: "result",
	}

	req := httptest.NewRequest("GET", "/status?requestId="+strconv.FormatUint(uint64(requestId), 10), nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response coordinator.UserStatusResponse
	json.NewDecoder(w.Body).Decode(&response)
	assert.Equal(t, coordinator.PROCESSING, response.Status)
	assert.Equal(t, "result", response.Result)
}

func TestSubmitRequestCrackHandler(t *testing.T) {
	coord := coordinator.NewCoordinator()
	handler := coordinator.SubmitRequestCrackHandler(coord)

	requestBody := coordinator.UserRequest{
		Hash:      "hash",
		MaxLength: 5,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(requestBody); err != nil {
		t.Fatal(err)

	}
	req := httptest.NewRequest("POST", "/crack", &buf)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response coordinator.UserResponse
	json.NewDecoder(w.Body).Decode(&response)
	assert.NotEqual(t, shared.UserRequestId(0), response.RequestId)
}

func TestRegisterNewWorkerHandler(t *testing.T) {
	coord := coordinator.NewCoordinator()
	handler := coordinator.RegisterNewWorkerHandler(coord)

	workerStatus := shared.WorkerStatusResponse{
		Status: shared.IDLE,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(workerStatus); err != nil {
		t.Fatal(err)

	}
	req := httptest.NewRequest("POST", "/register", &buf)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, coord.Workers)
}

func TestGetTaskResultHandler(t *testing.T) {
	coord := coordinator.NewCoordinator()
	handler := coordinator.GetTaskResultHandler(coord)

	taskId := shared.TaskId(uuid.New().ID())
	task := shared.WorkerTask{
		Id:        taskId,
		RequestId: shared.UserRequestId(uuid.New().ID()),
		Hash:      "hash",
		MaxLength: 5,
		Status:    shared.IN_PROGRESS,
		Result:    "result",
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(task); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/task?taskId="+strconv.FormatUint(uint64(taskId), 10), &buf)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
