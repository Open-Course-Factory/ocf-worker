#!/bin/bash
# Vérification des données Garage avec upload test

echo "🧪 Garage Data Verification Test"
echo "================================"

# Test 1: Upload un fichier et tracer son chemin
echo ""
echo "1️⃣ Upload Test with Tracing"
echo "---------------------------"

# Créer un fichier unique
TEST_CONTENT="Garage test data - $(date) - $$"
echo "$TEST_CONTENT" > /tmp/garage-trace-test.txt

echo "📤 Uploading test file..."
JOB_ID="trace-test-$(date +%s)"

# Surveiller les volumes AVANT upload
echo "📊 Volume state BEFORE upload:"
for vol in $(docker volume ls -q | grep garage); do
    MOUNT_POINT=$(docker volume inspect $vol | jq -r '.[0].Mountpoint')
    SIZE_BEFORE=$(sudo du -s "$MOUNT_POINT" 2>/dev/null | cut -f1 || echo "0")
    echo "  $vol: ${SIZE_BEFORE} KB"
done

# Surveiller l'utilisation dans le container AVANT
echo "📦 Container usage BEFORE:"
docker exec ocf-worker-garage df -h /data /meta 2>/dev/null || echo "Cannot get container usage"

# Upload via OCF Worker
UPLOAD_RESPONSE=$(curl -s -X POST \
    -F "files=@/tmp/garage-trace-test.txt" \
    http://localhost:8081/api/v1/storage/jobs/$JOB_ID/sources)

echo "Upload response: $UPLOAD_RESPONSE"

if echo "$UPLOAD_RESPONSE" | grep -q '"count"'; then
    echo "✅ Upload successful"
    
    # Attendre un peu pour s'assurer que l'écriture est terminée
    sleep 2
    
    # Surveiller les volumes APRÈS upload
    echo ""
    echo "📊 Volume state AFTER upload:"
    for vol in $(docker volume ls -q | grep garage); do
        MOUNT_POINT=$(docker volume inspect $vol | jq -r '.[0].Mountpoint')
        SIZE_AFTER=$(sudo du -s "$MOUNT_POINT" 2>/dev/null | cut -f1 || echo "0")
        echo "  $vol: ${SIZE_AFTER} KB"
    done
    
    # Surveiller l'utilisation dans le container APRÈS
    echo "📦 Container usage AFTER:"
    docker exec ocf-worker-garage df -h /data /meta 2>/dev/null || echo "Cannot get container usage"
    
    # Vérifier le téléchargement
    echo ""
    echo "📥 Testing download..."
    DOWNLOAD_CONTENT=$(curl -s http://localhost:8081/api/v1/storage/jobs/$JOB_ID/sources/garage-trace-test.txt)
    
    if [ "$DOWNLOAD_CONTENT" = "$TEST_CONTENT" ]; then
        echo "✅ Download successful - content matches"
        echo "🎯 Data is being stored and retrieved correctly"
    else
        echo "❌ Download failed or content mismatch"
        echo "Expected: $TEST_CONTENT"
        echo "Got: $DOWNLOAD_CONTENT"
    fi
    
else
    echo "❌ Upload failed"
fi

# Nettoyer
rm -f /tmp/garage-trace-test.txt

echo ""
echo "2️⃣ Garage Internal Data Inspection"
echo "-----------------------------------"

# Regarder dans les répertoires Garage internes
echo "🔍 Exploring Garage data structure..."

echo "Contents of /data (container):"
docker exec ocf-worker-garage find /data -type f -ls 2>/dev/null | head -10 || echo "No files found or access denied"

echo ""
echo "Contents of /meta (container):"
docker exec ocf-worker-garage find /meta -type f -ls 2>/dev/null | head -10 || echo "No files found or access denied"

# Vérifier les bases de données SQLite
echo ""
echo "🗃️ SQLite databases in Garage:"
docker exec ocf-worker-garage find /meta /data -name "*.sqlite" -o -name "*.db" 2>/dev/null || echo "No SQLite files found"

echo ""
echo "3️⃣ Volume Mount Verification"
echo "-----------------------------"

# Vérifier que les volumes sont bien montés
echo "🔗 Volume mounts verification:"
CONTAINER_ID=$(docker ps -q --filter name=ocf-worker-garage)

if [ -n "$CONTAINER_ID" ]; then
    echo "Container ID: $CONTAINER_ID"
    echo "Mount points:"
    docker inspect $CONTAINER_ID | jq -r '.[0].Mounts[] | select(.Destination | contains("/data") or contains("/meta")) | "  " + .Source + " -> " + .Destination + " (" + .Type + ")"'
    
    # Vérifier qu'on peut écrire
    echo ""
    echo "✍️ Write test:"
    if docker exec ocf-worker-garage touch /data/write-test 2>/dev/null; then
        echo "  ✅ Can write to /data"
        docker exec ocf-worker-garage rm /data/write-test
    else
        echo "  ❌ Cannot write to /data"
    fi
    
    if docker exec ocf-worker-garage touch /meta/write-test 2>/dev/null; then
        echo "  ✅ Can write to /meta"
        docker exec ocf-worker-garage rm /meta/write-test
    else
        echo "  ❌ Cannot write to /meta"
    fi
else
    echo "❌ Container not found"
fi

echo ""
echo "4️⃣ Expected vs Actual Behavior"
echo "------------------------------"

echo "📋 What you SHOULD see:"
echo "  - Files in /data and /meta inside container"
echo "  - SQLite database files"
echo "  - Binary/encoded data (not readable text files)"
echo "  - Changes in volume sizes after uploads"
echo ""

echo "📋 What you SHOULD NOT expect:"
echo "  - Raw uploaded files visible as-is"
echo "  - Human-readable file names"
echo "  - Direct file structure matching uploads"
echo ""

echo "🎯 Current behavior analysis:"

# Analyser si le comportement est normal
DATA_FILES=$(docker exec ocf-worker-garage find /data -type f 2>/dev/null | wc -l)
META_FILES=$(docker exec ocf-worker-garage find /meta -type f 2>/dev/null | wc -l)

echo "  Data files in /data: $DATA_FILES"
echo "  Meta files in /meta: $META_FILES"

if [ "$DATA_FILES" -gt 0 ] || [ "$META_FILES" -gt 0 ]; then
    echo "  ✅ NORMAL: Garage is storing data in internal format"
    echo "  💡 Data exists but not in human-readable format"
else
    echo "  ❌ ABNORMAL: No files found in Garage directories"
    echo "  🚨 This indicates a real problem"
fi

echo ""
echo "==============================================="
echo "🎯 Final Diagnosis"
echo "==============================================="

if [ "$DATA_FILES" -gt 0 ] || [ "$META_FILES" -gt 0 ]; then
    echo "✅ VERDICT: Garage is working correctly"
    echo ""
    echo "📚 Explanation:"
    echo "  - Garage stores data in optimized binary format"
    echo "  - Files are not stored as separate readable files"
    echo "  - Data is in SQLite databases and encoded blocks"
    echo "  - This is EXPECTED behavior for Garage"
    echo ""
    echo "🔍 To verify data is really there:"
    echo "  - Use the S3 API (aws s3 ls)"
    echo "  - Use OCF Worker API endpoints"
    echo "  - Check Garage metrics"
else
    echo "❌ VERDICT: Garage is NOT storing data correctly"
    echo ""
    echo "🚨 Possible issues:"
    echo "  - Volume mount problems"
    echo "  - Permission issues"
    echo "  - Garage configuration errors"
    echo "  - Layout not properly configured"
    echo ""
    echo "🔧 Recommended fixes:"
    echo "  1. Check logs: make garage-logs"
    echo "  2. Reset and reconfigure: make garage-reset"
    echo "  3. Verify volume mounts in docker-compose.yml"
fi