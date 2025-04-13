package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"lab2/coordinator"
	"lab2/shared"
	"lab2/task"
	"lab2/user_request"
	"lab2/worker"
)

func GetRequestStatusHandler(coord *coordinator.Coordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()

		requestId := queryParams.Get("requestId")
		if requestId == "" {
			http.Error(w, "Missing requestId parameter", http.StatusBadRequest)
			return
		}

		value, err := strconv.ParseUint(requestId, 10, 32)
		if err != nil {
			http.Error(w, "Invalid requestId parameter", http.StatusInternalServerError)
			return
		}

		response := coord.GetUserRequestStatus(shared.UserRequestId(value))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func SubmitRequestCrackHandler(coord *coordinator.Coordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userRequest user_request.UserRequest
		if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		requestId, err := coord.Crack(&userRequest)
		if err != nil {
			log.Printf("Failed to process crack request: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "Failed to process request",
				"details": err.Error(),
			})
			return
		}

		response := user_request.UserResponse{
			RequestId: requestId,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func RegisterNewWorkerHandler(coord *coordinator.Coordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var worker worker.Worker
		var workerStatus shared.WorkerStatusResponse
		err := json.NewDecoder(r.Body).Decode(&workerStatus)
		if err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		worker.Address = r.RemoteAddr
		worker.LastHB = time.Now()
		worker.Status = workerStatus.Status
		coord.RegisterWorker(&worker)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Worker registered successfully"))

		log.Println("new worker with address", worker.Address, "was registered")
	}
}

func GetTaskResultHandler(coord *coordinator.Coordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var task task.Task
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		coord.UpdateTask(&task)
	}
}
