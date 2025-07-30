# deployments/docker/Dockerfile - Fixed version with proper permissions
# Build stage
FROM golang:1.23 AS builder

# Installer les dépendances de build
RUN apt update && apt install -y git ca-certificates tzdata

WORKDIR /app

# Copier les fichiers de dépendances
COPY go.mod go.sum ./
RUN go mod download

# Copier le code source
COPY . .

# Install Go Library & Swagger
RUN go get golang.org/x/text/transform \
    && go get golang.org/x/text/unicode/norm \
    && go install github.com/swaggo/swag/cmd/swag@v1.8.12

# Init Swagger
RUN swag init -g ./cmd/generator/main.go --parseDependency --parseInternal \
# Build l'application
    && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ocf-worker cmd/generator/main.go

# Runtime stage avec Node.js pour Slidev
FROM node:24-bookworm-slim

# Installer les dépendances système
RUN apt update && apt install -y \
    curl \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Installer Slidev et Playwright avec dépendances
# hadolint ignore=DL3016
RUN npm install -g @slidev/cli@latest playwright-chromium \
    && npx playwright install-deps chromium \
    && npm cache clean --force \
    && npx @slidev/cli --version

# Créer les répertoires nécessaires avec les bonnes permissions
RUN mkdir -p /app/storage /app/logs /app/workspaces \
    && chown -R node:node /app \
    && chmod -R 755 /app/storage /app/logs /app/workspaces

COPY --from=builder /app/ocf-worker /usr/local/bin/ocf-worker
RUN chmod +x /usr/local/bin/ocf-worker \
# S'assurer que node peut écrire dans les répertoires après la copie
    && chown -R node:node /app/storage /app/logs /app/workspaces

USER node

WORKDIR /app

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8081/health || exit 1

EXPOSE 8081

CMD ["ocf-worker"]
