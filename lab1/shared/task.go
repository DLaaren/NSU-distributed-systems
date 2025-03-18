package shared

import "context"

type TaskStatus string

const (
	IN_PROGRESS  TaskStatus = "IN_PROGRESS"
	DONE_SUCCESS TaskStatus = "DONE_SUCCESS"
	DONE_FAILURE TaskStatus = "DONE_FAILURE"
	KILLED       TaskStatus = "KILLED"
	UNKNOWN      TaskStatus = "UNKNOWN"
)

type WorkerTask struct {
	Id         TaskId             `json:"id"`
	RequestId  UserRequestId      `json:"requestId"`
	WorkerId   WorkerId           `json:"-"`
	Hash       string             `json:"hash"`
	InputRange string             `json:"inputRange"` // like "aaa-ddd"
	MaxLength  uint32             `json:"maxLength"`
	Status     TaskStatus         `json:"status"`
	Result     string             `json:"result"`
	CancelFunc context.CancelFunc `json:"-"`
}

type TaskStatusResponse struct {
	Status TaskStatus `json:"status"`
}

type TaskResultResponse struct {
	Status TaskStatus `json:"status"`
	Result string     `json:"result"`
}
