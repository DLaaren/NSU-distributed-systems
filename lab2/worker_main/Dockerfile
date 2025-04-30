FROM golang:alpine3.21

WORKDIR /app

COPY . .

ENV GOPROXY=https://goproxy.io,direct

RUN go mod tidy
 
RUN go build -gcflags="all=-N -l" -o bin/worker worker_main/*.go
RUN cp worker_main/config.yaml bin/
 
WORKDIR /app/bin

CMD [ "./worker"]