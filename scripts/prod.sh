#!/bin/bash
set -e

echo "🚀 Starting OCF Worker in production mode..."

# Vérifier que le fichier .env.prod existe
if [ ! -f .env.prod ]; then
    echo "❌ .env.prod file not found!"
    echo "📝 Copy .env.prod.example to .env.prod and configure it"
    exit 1
fi

# Build l'image de production
docker build -t ocf-worker:latest -f deployments/docker/Dockerfile .

# Démarrer avec le compose de production
docker-compose -f docker-compose.prod.yml --env-file .env.prod up -d

echo "✅ OCF Worker started in production mode!"
echo "📊 Health check: http://localhost:8081/health"
