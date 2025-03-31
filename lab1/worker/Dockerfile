FROM golang:1.23.6

WORKDIR /app

COPY . .

RUN go mod tidy
 
RUN go build -gcflags="all=-N -l" -o bin/worker worker/main/main.go
RUN cp worker/main/config.yaml bin/
 
WORKDIR /app/bin

CMD [ "./worker"]