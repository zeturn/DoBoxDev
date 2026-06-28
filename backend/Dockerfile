# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies for CGO
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=1 GOOS=linux go build -o server cmd/server/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates git sqlite-libs

WORKDIR /root/

COPY --from=builder /app/server .

EXPOSE 3000

CMD ["./server"]
