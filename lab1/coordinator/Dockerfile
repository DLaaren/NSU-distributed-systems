FROM golang:1.23.6

WORKDIR /app

COPY . .

RUN go mod tidy
 
RUN go build -gcflags="all=-N -l" -o bin/coordinator coordinator/main/main.go
RUN cp coordinator/main/config.yaml bin/
 
EXPOSE 8080

WORKDIR /app/bin

CMD [ "./coordinator"]