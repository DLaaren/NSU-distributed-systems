package shared

type TaskStatus string

const (
	IN_PROGRESS  TaskStatus = "IN_PROGRESS"
	DONE_SUCCESS            = "DONE_SUCCESS"
	DONE_FAILURE            = "DONE_FAILURE"
	KILLED                  = "KILLED"
	UNKNOWN                 = "UNKNOWN"
)

type WorkerTask struct {
	Id         TaskId
	RequestId  UserRequestId
	WorkerId   WorkerId
	Hash       string     `json:"hash"`
	InputRange string     `json:"inputRange"` // like "aaa-ddd"
	MaxLength  uint32     `json:"maxLength"`
	Status     TaskStatus `json:"status"`
	Result     string     `json:"result"`
}

type TaskStatusResponse struct {
	Status TaskStatus `json:"status"`
}
