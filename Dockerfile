FROM golang:1.13.6

RUN mkdir /app

COPY ./go.mod /app/go.mod
COPY ./server.go /app/server.go

WORKDIR /app

RUN go build -o main .

CMD ["/app/main"]