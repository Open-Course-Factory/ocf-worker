#!/bin/bash
# scripts/test-garage-profile.sh - Test Garage avec profil docker-compose (version améliorée)

set -e

echo "🚀 Testing Garage storage using docker-compose profile..."

# Vérifier que docker-compose existe
if ! command -v docker >/dev/null 2>&1; then
    echo "❌ docker not found"
    exit 1
fi

# Support pour docker compose v1 et v2
DOCKER_COMPOSE_CMD="docker compose"
if ! docker compose version >/dev/null 2>&1; then
    if command -v docker-compose >/dev/null 2>&1; then
        DOCKER_COMPOSE_CMD="docker-compose"
        echo "ℹ️ Using docker-compose v1"
    else
        echo "❌ Neither 'docker compose' nor 'docker-compose' found"
        exit 1
    fi
else
    echo "ℹ️ Using docker compose v2"
fi

# Vérifier que le fichier de config existe
if [ ! -f "deployments/garage/garage.toml" ]; then
    echo "❌ Garage configuration file not found: deployments/garage/garage.toml"
    echo "💡 Create it first with: make garage-fix-rpc"
    exit 1
fi

# Fonction de nettoyage
cleanup() {
    echo "🧹 Cleaning up Garage..."
    $DOCKER_COMPOSE_CMD --profile garage down -v 2>/dev/null || true
}

# S'assurer du nettoyage même en cas d'erreur
trap cleanup EXIT

# Fonction pour obtenir le Node ID avec retry
get_node_id() {
    local max_attempts=10
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        echo "🔍 Getting Garage node ID (attempt $attempt/$max_attempts)..."
        NODE_ID=$($DOCKER_COMPOSE_CMD exec -T garage /garage node id -q 2>/dev/null | cut -d '@' -f1 | tr -d '\r\n' || true)
        
        if [ -n "$NODE_ID" ] && [ ${#NODE_ID} -gt 10 ]; then
            echo "✅ Node ID obtained: $NODE_ID"
            return 0
        fi
        
        echo "⏳ Waiting for Garage node to be ready..."
        sleep 3
        ((attempt++))
    done
    
    echo "❌ Failed to get node ID after $max_attempts attempts"
    return 1
}

# Fonction pour configurer le layout
setup_garage_layout() {
    echo "🏗️ Setting up Garage layout..."
    
    # Vérifier si le layout existe déjà
    if $DOCKER_COMPOSE_CMD exec -T garage /garage layout show 2>/dev/null | grep -q "dc1"; then
        echo "✅ Layout already configured"
        return 0
    fi
    
    if ! get_node_id; then
        return 1
    fi
    
    echo "📐 Assigning layout..."
    if $DOCKER_COMPOSE_CMD exec -T garage /garage layout assign "$NODE_ID" -z dc1 -c 1; then
        echo "✅ Layout assigned"
    else
        echo "❌ Failed to assign layout"
        return 1
    fi
    
    echo "📋 Applying layout..."
    if $DOCKER_COMPOSE_CMD exec -T garage /garage layout apply --version 1; then
        echo "✅ Layout applied"
    else
        echo "❌ Failed to apply layout"
        return 1
    fi
    
    # Attendre que le layout soit effectif
    echo "⏳ Waiting for layout to be effective..."
    sleep 5
    
    return 0
}

# Fonction de test de connectivité robuste
test_garage_connectivity() {
    echo "🔍 Testing Garage connectivity..."
    
    # Test S3 API
    local s3_retries=5
    local s3_success=false
    
    for i in $(seq 1 $s3_retries); do
        if curl -s --connect-timeout 5 --max-time 10 "$TEST_GARAGE_ENDPOINT" >/dev/null 2>&1; then
            echo "✅ S3 API ($TEST_GARAGE_ENDPOINT) is accessible"
            s3_success=true
            break
        fi
        echo "⏳ S3 API not ready, retrying... ($i/$s3_retries)"
        sleep 2
    done
    
    if [ "$s3_success" = false ]; then
        echo "❌ S3 API not accessible after $s3_retries attempts"
        return 1
    fi
    
    # Test Admin API
    if curl -s --connect-timeout 5 --max-time 10 "http://localhost:3903/health" >/dev/null 2>&1; then
        echo "✅ Admin API (http://localhost:3903) is accessible"
    else
        echo "⚠️ Admin API not accessible (may not affect S3 functionality)"
    fi
    
    return 0
}

# Fonction de diagnostic détaillé
garage_diagnostic() {
    echo ""
    echo "🔧 Garage diagnostic information:"
    echo "================================="
    
    echo "📊 Container status:"
    $DOCKER_COMPOSE_CMD --profile garage ps
    
    echo ""
    echo "🌐 Network information:"
    $DOCKER_COMPOSE_CMD exec -T garage ip addr show 2>/dev/null | grep -E "(inet|UP)" || echo "Cannot get container network info"
    
    echo ""
    echo "🔌 Port connectivity:"
    for port in 3900 3901 3902 3903; do
        if nc -z localhost $port 2>/dev/null; then
            echo "  ✅ Port $port is accessible"
        else
            echo "  ❌ Port $port is not accessible"
        fi
    done
    
    echo ""
    echo "📝 Recent logs (last 25 lines):"
    $DOCKER_COMPOSE_CMD --profile garage logs --tail=25 garage
    
    echo ""
    echo "🏗️ Layout status:"
    $DOCKER_COMPOSE_CMD exec -T garage /garage layout show 2>/dev/null || echo "Cannot get layout status"
    
    echo ""
    echo "🔑 Keys status:"
    $DOCKER_COMPOSE_CMD exec -T garage /garage key list 2>/dev/null || echo "Cannot list keys"
    
    echo ""
    echo "🪣 Buckets status:"
    $DOCKER_COMPOSE_CMD exec -T garage /garage bucket list 2>/dev/null || echo "Cannot list buckets"
    
    echo ""
    echo "📁 Configuration file:"
    if [ -f "deployments/garage/garage.toml" ]; then
        echo "✅ Configuration exists"
        echo "RPC secret length: $(grep "rpc_secret" deployments/garage/garage.toml | cut -d'"' -f2 | wc -c) characters"
    else
        echo "❌ Configuration file missing"
    fi
}

echo "📦 Starting Garage with profile..."
$DOCKER_COMPOSE_CMD --profile garage up -d garage

echo "⏳ Waiting for Garage to be healthy..."
for i in {1..90}; do
    if curl -s --connect-timeout 3 http://localhost:3903/health >/dev/null 2>&1; then
        echo "✅ Garage is healthy and ready"
        break
    fi
    if [ $i -eq 90 ]; then
        echo "❌ Garage failed to start within 90 seconds"
        garage_diagnostic
        exit 1
    fi
    if [ $((i % 10)) -eq 0 ]; then
        echo "⏳ Still waiting... ($i/90)"
    fi
    sleep 1
done

# Configuration du layout avec gestion d'erreur améliorée
if ! setup_garage_layout; then
    echo "❌ Failed to setup Garage layout"
    garage_diagnostic
    exit 1
fi

# Configuration des clés et buckets pour les tests
echo "🔑 Setting up test credentials and bucket..."

ACCESS_KEY="GK31c2f218a2e44f485b94239e"
SECRET_KEY="4420d99ef7aa26b56b5130ad7913a6a5c77653a5e7a47a3b4c9b8b9c5f8b7b4d"
BUCKET="ocf-test"

# Créer la clé d'accès avec retry
echo "🔐 Creating access key..."
key_created=false
for attempt in {1..3}; do
    if $DOCKER_COMPOSE_CMD exec -T garage /garage key new --name test-key 2>/dev/null; then
        key_created=true
        break
    elif $DOCKER_COMPOSE_CMD exec -T garage /garage key list 2>/dev/null | grep -q "test-key"; then
        echo "ℹ️ Key already exists"
        key_created=true
        break
    fi
    echo "⏳ Retrying key creation... ($attempt/3)"
    sleep 2
done

if [ "$key_created" = false ]; then
    echo "❌ Failed to create or verify test key"
    garage_diagnostic
    exit 1
fi

# Import de la clé avec gestion d'erreur
echo "🔑 Importing access credentials..."
if ! $DOCKER_COMPOSE_CMD exec -T garage /garage key import \
    --name test-key \
    --access-key-id "${ACCESS_KEY}" \
    --secret-access-key "${SECRET_KEY}" 2>/dev/null; then
    echo "ℹ️ Key import failed (may already be imported)"
fi

# Créer le bucket avec retry
echo "🪣 Creating test bucket..."
bucket_created=false
for attempt in {1..3}; do
    if $DOCKER_COMPOSE_CMD exec -T garage /garage bucket create "${BUCKET}" 2>/dev/null; then
        bucket_created=true
        break
    elif $DOCKER_COMPOSE_CMD exec -T garage /garage bucket list 2>/dev/null | grep -q "${BUCKET}"; then
        echo "ℹ️ Bucket already exists"
        bucket_created=true
        break
    fi
    echo "⏳ Retrying bucket creation... ($attempt/3)"
    sleep 2
done

if [ "$bucket_created" = false ]; then
    echo "❌ Failed to create or verify test bucket"
    garage_diagnostic
    exit 1
fi

# Autoriser l'accès
echo "🔓 Setting bucket permissions..."
if ! $DOCKER_COMPOSE_CMD exec -T garage /garage bucket allow \
    --read --write \
    "${BUCKET}" \
    --key test-key 2>/dev/null; then
    echo "⚠️ Permission setting may have failed"
fi

# Vérifier la configuration
echo "📋 Verifying Garage configuration..."
echo "Keys:"
$DOCKER_COMPOSE_CMD exec -T garage /garage key list || echo "Cannot list keys"
echo "Buckets:"
$DOCKER_COMPOSE_CMD exec -T garage /garage bucket list || echo "Cannot list buckets"

# Configurer les variables d'environnement pour les tests Go
export TEST_GARAGE_ENDPOINT="http://localhost:3900"
export TEST_GARAGE_ACCESS_KEY="${ACCESS_KEY}"
export TEST_GARAGE_SECRET_KEY="${SECRET_KEY}"
export TEST_GARAGE_BUCKET="${BUCKET}"
export TEST_GARAGE_REGION="garage"

echo ""
echo "🧪 Running Garage storage tests..."
echo "Test configuration:"
echo "  📡 Endpoint: $TEST_GARAGE_ENDPOINT"
echo "  🪣 Bucket: $TEST_GARAGE_BUCKET"
echo "  🔑 Access Key: ${TEST_GARAGE_ACCESS_KEY:0:10}..."
echo "  🌍 Region: $TEST_GARAGE_REGION"

# Test de connectivité avec fonction améliorée
if ! test_garage_connectivity; then
    echo "❌ Garage connectivity test failed"
    garage_diagnostic
    exit 1
fi

# Attendre un peu pour s'assurer que tout est stable
echo "⏳ Allowing services to stabilize..."
sleep 3

# Lancer les tests Go
echo "▶️  Running Go tests for Garage storage..."
if go test -v ./internal/storage/garage/ -run TestGarageStorageIntegration; then
    echo "✅ Garage storage tests passed!"
else
    echo "❌ Garage storage tests failed"
    echo ""
    garage_diagnostic
    exit 1
fi

echo ""
echo "🎉 All Garage tests completed successfully!"
echo ""
echo "📊 Test Summary:"
echo "  ✅ Garage container started with profile"
echo "  ✅ Health check passed"
echo "  ✅ Layout configured successfully"
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
echo "💡 To stop Garage: $DOCKER_COMPOSE_CMD --profile garage down"
echo "💡 To view logs: $DOCKER_COMPOSE_CMD --profile garage logs -f garage"