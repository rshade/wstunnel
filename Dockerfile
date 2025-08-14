FROM golang:1.25-alpine as builder

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . ./

RUN go build -v -o server

FROM alpine:latest

COPY --from=builder /app/server /app/server

ENTRYPOINT ["/app/server"]
CMD ["srv"]
