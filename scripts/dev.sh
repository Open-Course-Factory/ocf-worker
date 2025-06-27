#!/bin/bash
set -e

echo "🚀 Starting OCF Worker in development mode..."

# Créer les répertoires locaux
mkdir -p storage logs

# Démarrer avec Docker Compose
docker-compose up --build

echo "✅ OCF Worker started!"
echo "📊 Health check: http://localhost:8081/health"
echo "🔍 Logs: docker-compose logs -f ocf-worker"
