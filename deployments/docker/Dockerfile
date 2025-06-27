FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o ocf-worker cmd/generator/main.go

FROM node:21-alpine

# Install Slidev and dependencies
RUN npm install -g @slidev/cli playwright-chromium
RUN npx playwright install-deps

# Copy Go binary
COPY --from=builder /app/ocf-worker /usr/local/bin/

# Create working directories
RUN mkdir -p /app/storage /app/logs
WORKDIR /app

EXPOSE 8081

CMD ["ocf-worker"]
