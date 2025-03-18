package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lab1/shared"
	"lab1/worker"

	"github.com/stretchr/testify/assert"
)

func TestGetWorkerStatusHandler(t *testing.T) {
	workerCtx := &worker.WorkerContext{
		Status: shared.IDLE,
		Tasks:  make(map[shared.TaskId]*shared.WorkerTask),
	}

	handler := worker.GetWorkerStatusHandler(workerCtx)

	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response shared.WorkerStatusResponse
	json.NewDecoder(w.Body).Decode(&response)
	assert.Equal(t, shared.IDLE, response.Status)
}

func TestSubmitTaskHandler(t *testing.T) {
	workerCtx := &worker.WorkerContext{
		Status: shared.IDLE,
		Tasks:  make(map[shared.TaskId]*shared.WorkerTask),
	}

	coordinatorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer coordinatorServer.Close()

	handler := worker.SubmitTaskHandler(workerCtx, coordinatorServer.URL[len("http://"):])

	task := shared.WorkerTask{
		Id:         shared.TaskId(1),
		RequestId:  shared.UserRequestId(1),
		Hash:       "hash",
		InputRange: "aaa-zzz",
		MaxLength:  3,
		Status:     shared.IN_PROGRESS,
	}

	jsonBody, _ := json.Marshal(task)
	req := httptest.NewRequest("POST", "/task?requestId=1", bytes.NewBuffer(jsonBody))
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, workerCtx.Tasks[task.Id])
}

func TestKillTaskHandler(t *testing.T) {
	workerCtx := &worker.WorkerContext{
		Status: shared.IDLE,
		Tasks:  make(map[shared.TaskId]*shared.WorkerTask),
	}

	_, cancel := context.WithCancel(context.Background())
	task := &shared.WorkerTask{
		Id:         shared.TaskId(1),
		RequestId:  shared.UserRequestId(1),
		Hash:       "hash",
		InputRange: "aaa-zzz",
		MaxLength:  3,
		Status:     shared.IN_PROGRESS,
		CancelFunc: cancel,
	}

	workerCtx.Tasks[task.Id] = task

	handler := worker.KillTaskHandler(workerCtx)

	req := httptest.NewRequest("POST", "/kill?requestId=1", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, shared.KILLED, workerCtx.Tasks[task.Id].Status)
}

func TestSendAnswer(t *testing.T) {
	coordinatorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer coordinatorServer.Close()

	task := shared.WorkerTask{
		Id:         shared.TaskId(1),
		RequestId:  shared.UserRequestId(1),
		Hash:       "hash",
		InputRange: "aaa-zzz",
		MaxLength:  3,
		Status:     shared.DONE_SUCCESS,
		Result:     "result",
	}

	worker.SendTaskResultToCoordinator(task, coordinatorServer.URL[len("http://"):])
}

func TestRegisterTask(t *testing.T) {
	workerCtx := &worker.WorkerContext{
		Status: shared.IDLE,
		Tasks:  make(map[shared.TaskId]*shared.WorkerTask),
	}

	task := shared.WorkerTask{
		Id:         shared.TaskId(1),
		RequestId:  shared.UserRequestId(1),
		Hash:       "hash",
		InputRange: "aaa-zzz",
		MaxLength:  3,
		Status:     shared.IN_PROGRESS,
	}

	worker.RegisterTask(workerCtx, &task)

	assert.NotNil(t, workerCtx.Tasks[task.Id])
}

func TestTaskExecution(t *testing.T) {
	workerCtx := &worker.WorkerContext{
		Status: shared.IDLE,
		Tasks:  make(map[shared.TaskId]*shared.WorkerTask),
	}

	coordinatorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer coordinatorServer.Close()

	handler := worker.SubmitTaskHandler(workerCtx, coordinatorServer.URL[len("http://"):])

	task := shared.WorkerTask{
		Id:         shared.TaskId(1),
		RequestId:  shared.UserRequestId(1),
		Hash:       "5d41402abc4b2a76b9719d911017c592", // MD5 hash of "hello"
		InputRange: "hella-hello",
		MaxLength:  5,
		Status:     shared.IN_PROGRESS,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(task); err != nil {
		t.Fatal(err)

	}
	req := httptest.NewRequest("POST", "/task?requestId=1", &buf)
	w := httptest.NewRecorder()
	handler(w, req)

	time.Sleep(1 * time.Second)

	assert.Equal(t, shared.DONE_SUCCESS, workerCtx.Tasks[task.Id].Status)
	assert.Equal(t, "hello", workerCtx.Tasks[task.Id].Result)
}
