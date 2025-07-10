# .devcontainer/Dockerfile - Version robuste et simplifiée
FROM golang:1.24-bookworm

# Installer les dépendances système
RUN apt-get update && apt-get install -y \
    curl \
    git \
    nodejs \
    npm \
    ca-certificates \
    tzdata \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# Installer Node.js 24 (version plus récente)
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - \
    && apt-get install -y nodejs

# Installer les outils globalement
RUN npm install -g @slidev/cli@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install golang.org/x/tools/gopls@latest \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Créer un utilisateur pour VS Code avec sudo
RUN groupadd --gid 1000 vscode \
    && useradd --uid 1000 --gid vscode --shell /bin/bash --create-home vscode \
    && echo vscode ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/vscode \
    && chmod 0440 /etc/sudoers.d/vscode

# Créer les répertoires de travail
RUN mkdir -p /workspace \
    && chown -R vscode:vscode /workspace

RUN mkdir -p /app/logs \
    && chown -R vscode:vscode /app/logs
RUN mkdir -p /app/storage \
    && chown -R vscode:vscode /app/storage
RUN mkdir -p /app/workspaces \
    && chown -R vscode:vscode /app/workspaces

# Configuration de l'environnement Go pour vscode
ENV GOPATH=/home/vscode/go
ENV GOBIN=/home/vscode/go/bin
ENV PATH="${GOBIN}:${PATH}"

# Créer les répertoires Go
RUN mkdir -p /home/vscode/go/{bin,src,pkg} \
    && chown -R vscode:vscode /home/vscode

# Passer à l'utilisateur vscode
USER vscode

# Définir le répertoire de travail
WORKDIR /workspace

# Commande par défaut
CMD ["sleep", "infinity"]