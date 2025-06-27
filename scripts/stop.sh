#!/bin/bash

echo "🛑 Stopping OCF Worker..."

# Arrêter tous les services
docker-compose down
docker-compose --profile dev down
docker-compose -f docker-compose.prod.yml down

echo "✅ OCF Worker stopped!"
