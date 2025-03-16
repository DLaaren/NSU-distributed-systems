package coordinator

import "lab1/shared"

type UserRequestStatus string

const (
	PROCESSING    UserRequestStatus = "PROCESSING"
	READY         UserRequestStatus = "READY"
	TIMEOUT_ERROR UserRequestStatus = "TIMEOUT_ERROR"
	ERROR         UserRequestStatus = "ERROR"
)

type UserRequest struct {
	Id        shared.UserRequestId
	Hash      string `json:"hash"`
	MaxLength uint32 `json:"maxLength"`
	Status    UserRequestStatus
	Result    string
}

type UserResponse struct {
	RequestId shared.UserRequestId `json:"requestId"`
}

type UserStatusResponse struct {
	Status UserRequestStatus `json:"status"`
	Result string            `json:"result"`
}
