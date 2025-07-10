#!/bin/bash
# Vérifications manuelles de Garage

echo "🔍 Manual Garage Verification"
echo "============================="
echo ""

# Détection de docker compose
DOCKER_COMPOSE_CMD="docker compose"
if ! docker compose version >/dev/null 2>&1; then
    if command -v docker-compose >/dev/null 2>&1; then
        DOCKER_COMPOSE_CMD="docker-compose"
    fi
fi

echo "1️⃣ Service Status Check"
echo "----------------------"
$DOCKER_COMPOSE_CMD --profile garage ps

echo ""
echo "2️⃣ API Connectivity Tests"
echo "-------------------------"

# Test S3 API
echo "🌐 S3 API (port 3900):"
if curl -s --connect-timeout 5 http://localhost:3900 >/dev/null 2>&1; then
    echo "  ✅ S3 API is accessible"
    echo "  Response headers:"
    curl -I -s --connect-timeout 5 http://localhost:3900 | head -3
else
    echo "  ❌ S3 API not accessible"
fi

echo ""
echo "🔧 Admin API (port 3903):"
if curl -s --connect-timeout 5 http://localhost:3903/health >/dev/null 2>&1; then
    echo "  ✅ Admin API is accessible"
    HEALTH=$(curl -s http://localhost:3903/health)
    echo "  Health: $HEALTH"
else
    echo "  ❌ Admin API not accessible"
fi

echo ""
echo "🌐 Web UI (port 3902):"
if curl -s --connect-timeout 5 http://localhost:3902 >/dev/null 2>&1; then
    echo "  ✅ Web UI is accessible"
    echo "  👉 Open in browser: http://localhost:3902"
else
    echo "  ❌ Web UI not accessible"
fi

echo ""
echo "3️⃣ Garage Internal Status"
echo "-------------------------"

echo "🏗️ Layout:"
$DOCKER_COMPOSE_CMD exec -T garage /garage layout show 2>/dev/null || echo "❌ Cannot get layout"

echo ""
echo "🔑 Keys:"
$DOCKER_COMPOSE_CMD exec -T garage /garage key list 2>/dev/null || echo "❌ Cannot list keys"

echo ""
echo "🪣 Buckets:"
$DOCKER_COMPOSE_CMD exec -T garage /garage bucket list 2>/dev/null || echo "❌ Cannot list buckets"

echo ""
echo "4️⃣ S3 API Direct Test"
echo "---------------------"

# Test avec credentials de test
ACCESS_KEY="GK31c2f218a2e44f485b94239e"
SECRET_KEY="4420d99ef7aa26b56b5130ad7913a6a5c77653a5e7a47a3b4c9b8b9c5f8b7b4d"
BUCKET="ocf-test"
ENDPOINT="http://localhost:3900"

echo "📋 Test credentials:"
echo "  Endpoint: $ENDPOINT"
echo "  Bucket: $BUCKET"
echo "  Access Key: ${ACCESS_KEY:0:10}..."

# Test avec aws CLI si disponible
if command -v aws >/dev/null 2>&1; then
    echo ""
    echo "🛠️ AWS CLI Test:"
    export AWS_ACCESS_KEY_ID="$ACCESS_KEY"
    export AWS_SECRET_ACCESS_KEY="$SECRET_KEY"
    export AWS_DEFAULT_REGION="garage"
    
    echo "  Listing buckets:"
    aws s3 ls --endpoint-url "$ENDPOINT" 2>/dev/null || echo "  ❌ Cannot list buckets with AWS CLI"
    
    echo "  Testing bucket access:"
    aws s3 ls "s3://$BUCKET" --endpoint-url "$ENDPOINT" 2>/dev/null || echo "  ❌ Cannot access test bucket"
else
    echo "  ⚠️ AWS CLI not available for S3 testing"
fi

echo ""
echo "5️⃣ File Upload Test"
echo "-------------------"

# Créer un fichier de test
echo "Hello from manual verification!" > /tmp/garage-test.txt

echo "📤 Uploading test file via OCF Worker API..."
TEST_JOB_ID="manual-test-$(date +%s)"

UPLOAD_RESPONSE=$(curl -s -X POST \
    -F "files=@/tmp/garage-test.txt" \
    http://localhost:8081/api/v1/storage/jobs/$TEST_JOB_ID/sources)

echo "Upload response:"
echo "$UPLOAD_RESPONSE" | jq . 2>/dev/null || echo "$UPLOAD_RESPONSE"

if echo "$UPLOAD_RESPONSE" | grep -q '"count"'; then
    echo "✅ File uploaded successfully"
    
    echo ""
    echo "📥 Testing file download..."
    DOWNLOAD_CONTENT=$(curl -s http://localhost:8081/api/v1/storage/jobs/$TEST_JOB_ID/sources/garage-test.txt)
    
    if [ "$DOWNLOAD_CONTENT" = "Hello from manual verification!" ]; then
        echo "✅ File downloaded successfully with correct content"
    else
        echo "❌ File download failed or content mismatch"
        echo "Expected: Hello from manual verification!"
        echo "Got: $DOWNLOAD_CONTENT"
    fi
    
    echo ""
    echo "📁 Listing uploaded files..."
    curl -s http://localhost:8081/api/v1/storage/jobs/$TEST_JOB_ID/sources | jq . 2>/dev/null
    
else
    echo "❌ File upload failed"
fi

# Nettoyage
rm -f /tmp/garage-test.txt

echo ""
echo "6️⃣ Garage Metrics"
echo "-----------------"

if curl -s --connect-timeout 5 http://localhost:3903/metrics >/dev/null 2>&1; then
    echo "📊 Metrics endpoint accessible:"
    echo "  👉 View metrics: http://localhost:3903/metrics"
    echo ""
    echo "Key metrics:"
    curl -s http://localhost:3903/metrics | grep -E "(garage_|http_requests_total)" | head -5
else
    echo "❌ Metrics endpoint not accessible"
fi

echo ""
echo "7️⃣ Performance Test"
echo "-------------------"

echo "⚡ Quick performance test..."
START_TIME=$(date +%s%N)

# Test avec un fichier plus gros
dd if=/dev/zero of=/tmp/perf-test.bin bs=1024 count=100 2>/dev/null
echo "Test performance file" >> /tmp/perf-test.bin

PERF_RESPONSE=$(curl -s -X POST \
    -F "files=@/tmp/perf-test.bin" \
    http://localhost:8081/api/v1/storage/jobs/perf-test-$(date +%s)/sources)

END_TIME=$(date +%s%N)
DURATION=$(( (END_TIME - START_TIME) / 1000000 ))

if echo "$PERF_RESPONSE" | grep -q '"count"'; then
    echo "✅ Performance test completed in ${DURATION}ms (~100KB file)"
else
    echo "❌ Performance test failed"
fi

rm -f /tmp/perf-test.bin

echo ""
echo "8️⃣ Configuration Verification"
echo "-----------------------------"

echo "📁 Configuration file:"
if [ -f "deployments/garage/garage.toml" ]; then
    echo "✅ garage.toml exists"
    echo "Key settings:"
    grep -E "(rpc_secret|api_bind_addr|s3_region)" deployments/garage/garage.toml
else
    echo "❌ garage.toml missing"
fi

echo ""
echo "🐳 Docker configuration:"
$DOCKER_COMPOSE_CMD --profile garage config 2>/dev/null | grep -A 10 -B 5 "garage:" || echo "Cannot get docker config"

echo ""
echo "==============================================="
echo "🎉 Manual Verification Complete!"
echo "==============================================="
echo ""
echo "📊 Summary:"
echo "  🌐 S3 API: http://localhost:3900"
echo "  🔧 Admin API: http://localhost:3903"  
echo "  🌐 Web UI: http://localhost:3902"
echo "  📊 Metrics: http://localhost:3903/metrics"
echo ""
echo "💡 Useful commands:"
echo "  make garage-status    # Quick status check"
echo "  make garage-logs      # View logs"
echo "  make garage-debug     # Detailed debug info"
echo ""
echo "🧪 Test storage:"
echo "  make test-storage-garage    # Full test suite"
echo "  make test-storage-api garage # API tests"