FROM golang:latest

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod tidy

COPY . .

RUN go build -o worker ./cmd/worker

CMD ["./worker"]