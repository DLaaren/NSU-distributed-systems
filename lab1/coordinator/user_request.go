package coordinator

type UserRequestId uint32

type UserRequestStatus int

const (
	IN_PROGRESS UserRequestStatus = iota
	READY
	TIMEPIT_ERROR
)

type UserRequest struct {
	RequestId UserRequestId
	Hash      string `json:"hash"`
	MaxLength uint32 `json:"maxLength"`
	Status    UserRequestStatus
	Result    string
}

type UserResponse struct {
	RequestId UserRequestId `json:"requestId"`
}

type UserStatusResponse struct {
	Status UserRequestStatus `json:"status"`
	Result string            `json:"result"`
}
