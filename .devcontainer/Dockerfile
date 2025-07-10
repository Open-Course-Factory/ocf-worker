# deployments/docker/Dockerfile - Go environment with Slidev
# Stage 1: Build Node.js dependencies and install Slidev
FROM node:24-bookworm-slim AS node-builder

# Installer les dépendances système pour Node.js
RUN apt update && apt install -y \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Installer Slidev et Playwright globalement
RUN npm install -g @slidev/cli@latest playwright-chromium \
    && npx playwright install-deps chromium \
    && npm cache clean --force

# Stage 2: Runtime avec Go comme environnement principal
FROM golang:1.24

# Installer Node.js dans l'environnement Go
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - \
    && apt update && apt install -y \
    nodejs \
    curl \
    ca-certificates \
    tzdata \
    chromium \
    && rm -rf /var/lib/apt/lists/*

# Copier les installations globales de Node.js depuis le stage précédent
COPY --from=node-builder /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=node-builder /usr/local/bin/slidev /usr/local/bin/slidev
COPY --from=node-builder /usr/local/bin/playwright /usr/local/bin/playwright

# Créer les liens symboliques nécessaires
RUN ln -sf /usr/local/lib/node_modules/@slidev/cli/bin/slidev.mjs /usr/local/bin/slidev \
    && chmod +x /usr/local/bin/slidev

# Créer un utilisateur non-root (optionnel pour devcontainer)
RUN groupadd -r -g 1000 appuser && useradd -m -r -u 1000 -g appuser -s /bin/bash appuser

# Créer les répertoires nécessaires
RUN mkdir -p /app/storage /app/logs /app/workspaces \
    && chown -R appuser:appuser /app

# Configuration du workspace
WORKDIR /app

RUN go install github.com/go-delve/delve/cmd/dlv@latest

USER appuser

# Variables d'environnement pour Playwright
ENV PLAYWRIGHT_BROWSERS_PATH=/usr/local/lib/node_modules/playwright-chromium
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1

RUN go install -v github.com/go-delve/delve/cmd/dlv@latest 
RUN go install -v golang.org/x/tools/gopls@latest

COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Exposer les ports
EXPOSE 8081 3030

# Commande par défaut (peut être surchargée par devcontainer)
CMD ["bash"]