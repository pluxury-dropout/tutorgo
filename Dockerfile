FROM golang:alpine AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/pressly/goose/v3/cmd/goose@latest
COPY . .
RUN go build -o main .

FROM alpine:latest
RUN apk add --no-cache ca-certificates netcat-openbsd
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /go/bin/goose .
COPY migrations/ ./migrations/
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh
EXPOSE 8080
CMD ["./entrypoint.sh"]
