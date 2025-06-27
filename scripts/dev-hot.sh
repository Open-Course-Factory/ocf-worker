#!/bin/bash
set -e

echo "🔥 Starting OCF Worker with hot reload..."

# Démarrer seulement la base de données
docker-compose up -d postgres-worker

# Attendre que la base soit prête
echo "⏳ Waiting for database..."
until docker-compose exec postgres-worker pg_isready -U ocf_worker -d ocf_worker_db; do
  sleep 1
done

echo "✅ Database ready!"

# Démarrer le worker en mode dev avec profil
docker-compose --profile dev up --build ocf-worker-dev

echo "🔥 Hot reload active on http://localhost:8082"
