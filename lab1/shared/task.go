package shared

type TaskStatus int

const (
	IN_PROGRESS TaskStatus = iota
	DONE_SUCCESS
	DONE_FAILURE
	KILLED
)

type WorkerTask struct {
	Id         Id
	RequestId  Id
	Hash       string     `json:"hash"`
	InputRange string     `json:"inputRange"` // like "aaa-ddd"
	MaxLength  uint32     `json:"maxLength"`
	Status     TaskStatus `json:"status"`
	Result     string     `json:"result"`
}
