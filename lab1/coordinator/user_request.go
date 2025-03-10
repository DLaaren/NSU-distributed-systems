package coordinator

import "lab1/shared"

type UserRequestStatus int

const (
	PROCESSING UserRequestStatus = iota
	READY
	TIMEOUT_ERROR
)

type UserRequest struct {
	Id        shared.Id
	Hash      string `json:"hash"`
	MaxLength uint32 `json:"maxLength"`
	Status    UserRequestStatus
	Result    string
}

type UserResponse struct {
	RequestId shared.Id `json:"requestId"`
}

type UserStatusResponse struct {
	Status UserRequestStatus `json:"status"`
	Result string            `json:"result"`
}
