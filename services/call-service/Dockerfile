FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.work .
COPY shared/go.mod shared/
COPY services/gateway/go.mod services/gateway/
COPY services/auth-service/go.mod services/auth-service/
COPY services/chat-service/go.mod services/chat-service/
COPY services/call-service/go.mod services/call-service/

RUN go work sync || true

COPY shared/ shared/
COPY services/call-service/ services/call-service/

RUN CGO_ENABLED=0 GOOS=linux go build -o /call-service services/call-service/cmd/main.go

FROM alpine:3.19
WORKDIR /
COPY --from=builder /call-service /call-service
EXPOSE 50053
CMD ["/call-service"]
