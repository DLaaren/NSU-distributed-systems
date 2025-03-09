package worker

import "net"

type WorkerInfo struct {
	address  net.TCPAddr
	currTask *WorkerTask
	status   WorkerStatus
}
