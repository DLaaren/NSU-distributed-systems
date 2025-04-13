FROM golang:latest

WORKDIR /app

COPY . .

RUN go mod tidy
 
RUN go build -gcflags="all=-N -l" -o bin/worker worker_main/*.go
RUN cp worker_main/config.yaml bin/
 
WORKDIR /app/bin

CMD [ "./worker"]