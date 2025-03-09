package coordinator

type UserRequestId uint32

type UserRequest struct {
	Hash      string `json:"hash"`
	MaxLength uint32 `json:"maxLength"`
}

type UserResponse struct {
	RequestId UserRequestId `json:"requestId"`
}

type UserRequestStatus int

const (
	IN_PROGRESS UserRequestStatus = iota
	READY
	TIMEOUT
)

type UserStatusResponse struct {
	Status UserRequestStatus `json:"status"`
	Result string            `json:"result"`
}
