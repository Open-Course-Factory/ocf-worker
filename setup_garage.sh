#!/bin/bash
echo "🚀 Configuration automatique de Garage..."

# Démarrer Garage si pas déjà fait
docker compose --profile garage up -d garage

# Attendre qu'il soit prêt
for i in {1..60}; do
    if curl -s --connect-timeout 3 http://localhost:3903/health >/dev/null 2>&1; then
        echo "✅ Garage prêt"
        break
    fi
    sleep 1
done

# Configuration
NODE_ID=$(docker compose exec -T garage /garage node id -q 2>/dev/null | cut -d '@' -f1 | tr -d '\r\n')
echo "Node ID: $NODE_ID"

docker compose exec -T garage /garage layout assign "$NODE_ID" -z dc1 -c 1
docker compose exec -T garage /garage layout apply --version 1
sleep 10

# Clés
ACCESS_KEY="GK31c2f218a2e44f485b94239e"
SECRET_KEY="4420d99ef7aa26b56b5130ad7913a6a5c77653a5e7a47a3b4c9b8b9c5f8b7b4d"

#docker compose exec -T garage /garage key new --name test-key 2>/dev/null || true
docker compose exec -T garage /garage key import \
    "$ACCESS_KEY" \
    "$SECRET_KEY" \
    -n test-key

# Bucket
docker compose exec -T garage /garage bucket create "ocf-courses" 2>/dev/null || true
docker compose exec -T garage /garage bucket allow \
    --read --write "ocf-courses" \
    --key test-key 2>/dev/null || true

echo "🎉 Configuration terminée !"
echo "📊 Vérification..."
docker compose exec -T garage /garage key list
docker compose exec -T garage /garage bucket list