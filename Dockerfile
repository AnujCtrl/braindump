# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /todo ./cmd/todo/
RUN CGO_ENABLED=0 go build -o /server ./cmd/server/

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache tzdata
COPY --from=builder /todo /usr/local/bin/todo
COPY --from=builder /server /usr/local/bin/server
ENTRYPOINT ["/usr/local/bin/server"]
