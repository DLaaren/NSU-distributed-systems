FROM golang:latest

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod tidy

COPY . .

RUN go build -o coordinator ./cmd/coordinator

EXPOSE 8080

CMD ["./coordinator"]