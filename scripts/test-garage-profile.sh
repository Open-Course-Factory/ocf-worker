#!/bin/bash
# scripts/test-garage-profile.sh - Test Garage avec profil docker-compose

set -e

echo "🚀 Testing Garage storage using docker-compose profile..."

# Vérifier que docker-compose existe
if ! command -v docker-compose >/dev/null 2>&1; then
    echo "❌ docker-compose not found"
    exit 1
fi

# Vérifier que le fichier de config existe
if [ ! -f "deployments/garage/garage.toml" ]; then
    echo "❌ Garage configuration file not found: deployments/garage/garage.toml"
    echo "💡 Create it first with the provided configuration"
    exit 1
fi

# Fonction de nettoyage
cleanup() {
    echo "🧹 Cleaning up Garage..."
    docker compose -f docker-compose.yml --profile garage down -v 2>/dev/null || true
}

# S'assurer du nettoyage même en cas d'erreur
trap cleanup EXIT

echo "📦 Starting Garage with profile..."
docker compose --profile garage up -d garage

echo "⏳ Waiting for Garage to be healthy..."
for i in {1..60}; do
    if curl -s http://localhost:3903/health >/dev/null 2>&1; then
        echo "✅ Garage is healthy and ready"
        break
    fi
    if [ $i -eq 60 ]; then
        echo "❌ Garage failed to start within 60 seconds"
        echo "📊 Container status:"
        docker compose --profile garage ps
        echo "📝 Container logs:"
        docker compose --profile garage logs garage
        exit 1
    fi
    echo "⏳ Waiting... ($i/60)"
    sleep 1
done

# Configuration des clés et buckets pour les tests
echo "🔑 Setting up test credentials and bucket..."

ACCESS_KEY="GK31c2f218a2e44f485b94239e"
SECRET_KEY="4420d99ef7aa26b56b5130ad7913a6a5c77653a5e7a47a3b4c9b8b9c5f8b7b4d"
BUCKET="ocf-test"
NODE_ID=$(docker compose exec -it garage /garage node id -q | cut -d '@' -f1)

# Création du Layout
echo "Creating layout..."
docker compose exec -it garage \
    /garage layout assign $NODE_ID -z dc1 -c 1
docker compose exec -it garage \
    /garage layout apply --version 1

# Créer la clé d'accès
echo "🔐 Creating access key..."
docker compose exec garage \
    /garage key new --name test-key 2>/dev/null || echo "Key may already exist"

docker compose exec garage \
    /garage key import \
    --name test-key \
    --access-key-id "${ACCESS_KEY}" \
    --secret-access-key "${SECRET_KEY}" 2>/dev/null || echo "Key import may have failed (possibly already exists)"

# Créer le bucket
echo "🪣 Creating test bucket..."
docker compose exec garage \
    /garage bucket create "${BUCKET}" 2>/dev/null || echo "Bucket may already exist"

# Autoriser l'accès
echo "🔓 Setting bucket permissions..."
docker compose exec garage \
    /garage bucket allow \
    --read --write \
    "${BUCKET}" \
    --key test-key 2>/dev/null || echo "Permission setting may have failed"

# Vérifier la configuration
echo "📋 Verifying Garage configuration..."
echo "Keys:"
docker compose exec garage /garage key list || true
echo "Buckets:"
docker compose exec garage /garage bucket list || true

# Configurer les variables d'environnement pour les tests Go
export TEST_GARAGE_ENDPOINT="http://localhost:3900"
export TEST_GARAGE_ACCESS_KEY="${ACCESS_KEY}"
export TEST_GARAGE_SECRET_KEY="${SECRET_KEY}"
export TEST_GARAGE_BUCKET="${BUCKET}"
export TEST_GARAGE_REGION="garage"

echo "🧪 Running Garage storage tests..."
echo "Test configuration:"
echo "  📡 Endpoint: $TEST_GARAGE_ENDPOINT"
echo "  🪣 Bucket: $TEST_GARAGE_BUCKET"
echo "  🔑 Access Key: ${TEST_GARAGE_ACCESS_KEY:0:10}..."
echo "  🌍 Region: $TEST_GARAGE_REGION"

# Test de connectivité basique
echo "🔍 Testing basic connectivity..."
if curl -s --connect-timeout 5 "$TEST_GARAGE_ENDPOINT" >/dev/null 2>&1; then
    echo "✅ Garage S3 API is accessible"
else
    echo "❌ Cannot reach Garage S3 API at $TEST_GARAGE_ENDPOINT"
    echo "🔧 Debug information:"
    echo "Container status:"
    docker compose --profile garage ps
    echo "Port mappings:"
    docker port ocf-worker-garage 2>/dev/null || echo "No port mappings found"
    exit 1
fi

# Lancer les tests Go
echo "▶️  Running Go tests for Garage storage..."
if go test -v ./internal/storage/garage/ -run TestGarageStorageIntegration; then
    echo "✅ Garage storage tests passed!"
else
    echo "❌ Garage storage tests failed"
    echo "📝 Recent Garage logs:"
    docker compose --profile garage logs --tail=20 garage
    exit 1
fi

echo ""
echo "🎉 All Garage tests completed successfully!"
echo ""
echo "📊 Test Summary:"
echo "  ✅ Garage container started with profile"
echo "  ✅ Health check passed"
echo "  ✅ Test credentials configured"
echo "  ✅ Test bucket created"
echo "  ✅ API connectivity verified"
echo "  ✅ Go integration tests passed"
echo ""
echo "🔧 Garage is now available for development:"
echo "  S3 API: http://localhost:3900"
echo "  Admin API: http://localhost:3903"
echo "  Web UI: http://localhost:3902"
echo ""
echo "💡 To stop Garage: docker compose -f docker-compose.yml --profile garage down"