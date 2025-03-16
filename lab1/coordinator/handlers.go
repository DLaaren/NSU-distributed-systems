package coordinator

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"lab1/shared"
)

func GetRequestStatusHandler(coordinator CoordinatorI) http.HandlerFunc {
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

		response := coordinator.GetUserRequestStatus(shared.UserRequestId(value))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func SubmitRequestCrackHandler(coordinator CoordinatorI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userRequest UserRequest
		if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		requestId := coordinator.Crack(&userRequest)

		response := UserResponse{
			RequestId: requestId,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func RegisterNewWorkerHandler(coordinator CoordinatorI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var worker Worker
		var workerStatus shared.WorkerStatusResponse
		err := json.NewDecoder(r.Body).Decode(&workerStatus)
		if err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		worker.Address = r.RemoteAddr
		worker.LastHB = time.Now()
		worker.Status = workerStatus.Status
		coordinator.RegisterWorker(&worker)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Worker registered successfully"))

		log.Println("new worker with address", worker.Address, "was registered")
	}
}

func GetTaskResultHandler(coordinator CoordinatorI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryParams := r.URL.Query()

		taskIdStr := queryParams.Get("taskId")
		if taskIdStr == "" {
			http.Error(w, "missing requestId parameter", http.StatusBadRequest)
			return
		}

		taskId, err := strconv.ParseUint(taskIdStr, 10, 32)
		if err != nil {
			log.Fatal(err)
		}

		var task shared.WorkerTask
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		task.Id = shared.TaskId(taskId)
		coordinator.UpdateTask(&task)
	}
}
