FROM golang:latest

WORKDIR /app

COPY . .

ENV GOPROXY=https://goproxy.io,direct

RUN go mod tidy
 
RUN go build -gcflags="all=-N -l" -o bin/coordinator coordinator_main/*.go
RUN cp coordinator_main/config.yaml bin/
 
EXPOSE 8080

WORKDIR /app/bin

CMD [ "./coordinator"]