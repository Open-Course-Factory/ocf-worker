#!/bin/bash
# Diagnostic des volumes Garage

echo "🔍 Garage Volume Diagnostic"
echo "==========================="

# Détection de docker compose
DOCKER_COMPOSE_CMD="docker compose"
if ! docker compose version >/dev/null 2>&1; then
    if command -v docker-compose >/dev/null 2>&1; then
        DOCKER_COMPOSE_CMD="docker-compose"
    fi
fi

echo ""
echo "1️⃣ Volume Information"
echo "--------------------"

# Lister les volumes Garage
echo "📦 Garage volumes:"
docker volume ls | grep garage || echo "❌ No garage volumes found"

echo ""
echo "📊 Volume details:"
for vol in $(docker volume ls -q | grep garage); do
    echo "Volume: $vol"
    docker volume inspect $vol | jq -r '.[0] | "  Path: " + .Mountpoint + "\n  Driver: " + .Driver + "\n  Created: " + .CreatedAt'
    echo ""
done

echo ""
echo "2️⃣ Container Mount Points"
echo "-------------------------"

if docker ps | grep -q "ocf-worker-garage"; then
    echo "🐳 Container mounts:"
    docker inspect ocf-worker-garage | jq -r '.[0].Mounts[] | "  Source: " + .Source + "\n  Destination: " + .Destination + "\n  Type: " + .Type'
    
    echo ""
    echo "📁 Container internal directories:"
    echo "Contents of /data:"
    docker exec ocf-worker-garage ls -la /data/ 2>/dev/null || echo "❌ Cannot access /data"
    
    echo ""
    echo "Contents of /meta:"  
    docker exec ocf-worker-garage ls -la /meta/ 2>/dev/null || echo "❌ Cannot access /meta"
    
    echo ""
    echo "💾 Disk usage inside container:"
    docker exec ocf-worker-garage df -h /data /meta 2>/dev/null || echo "❌ Cannot get disk usage"
else
    echo "❌ Garage container not running"
fi

echo ""
echo "3️⃣ Host Volume Contents"
echo "-----------------------"

# Vérifier le contenu des volumes depuis l'hôte
for vol in $(docker volume ls -q | grep garage); do
    echo "🔍 Inspecting volume: $vol"
    MOUNT_POINT=$(docker volume inspect $vol | jq -r '.[0].Mountpoint')
    echo "  Mount point: $MOUNT_POINT"
    
    if [ -d "$MOUNT_POINT" ]; then
        echo "  Contents:"
        sudo ls -la "$MOUNT_POINT" 2>/dev/null || ls -la "$MOUNT_POINT" 2>/dev/null || echo "  ❌ Cannot access $MOUNT_POINT"
        echo "  Size:"
        sudo du -sh "$MOUNT_POINT" 2>/dev/null || du -sh "$MOUNT_POINT" 2>/dev/null || echo "  ❌ Cannot get size"
    else
        echo "  ❌ Mount point does not exist"
    fi
    echo ""
done

echo ""
echo "4️⃣ Garage Configuration Analysis"
echo "--------------------------------"

echo "📄 Configuration file:"
if [ -f "deployments/garage/garage.toml" ]; then
    echo "✅ garage.toml exists"
    echo "Data directory setting:"
    grep "data_dir" deployments/garage/garage.toml || echo "❌ No data_dir found"
    echo "Metadata directory setting:"
    grep "metadata_dir" deployments/garage/garage.toml || echo "❌ No metadata_dir found"
else
    echo "❌ garage.toml missing"
fi

echo ""
echo "🐳 Docker compose volume mapping:"
$DOCKER_COMPOSE_CMD --profile garage config | grep -A 10 -B 2 volumes: || echo "❌ Cannot get compose config"

echo ""
echo "5️⃣ Garage Internal Status"
echo "-------------------------"

if docker ps | grep -q "ocf-worker-garage"; then
    echo "🏗️ Layout status:"
    docker exec ocf-worker-garage /garage layout show 2>/dev/null || echo "❌ Cannot get layout"
    
    echo ""
    echo "📊 Node status:"
    docker exec ocf-worker-garage /garage node id 2>/dev/null || echo "❌ Cannot get node ID"
    
    echo ""
    echo "🔑 Keys:"
    docker exec ocf-worker-garage /garage key list 2>/dev/null || echo "❌ Cannot list keys"
    
    echo ""
    echo "🪣 Buckets:"
    docker exec ocf-worker-garage /garage bucket list 2>/dev/null || echo "❌ Cannot list buckets"
else
    echo "❌ Garage container not running"
fi

echo ""
echo "6️⃣ Data Write Test"
echo "------------------"

if docker ps | grep -q "ocf-worker-garage"; then
    echo "🧪 Testing data persistence..."
    
    # Créer un fichier test dans le container
    echo "Creating test file in /data..."
    docker exec ocf-worker-garage sh -c "echo 'test data' > /data/test-file.txt" 2>/dev/null || echo "❌ Cannot write to /data"
    
    # Vérifier qu'il existe
    echo "Checking test file:"
    docker exec ocf-worker-garage ls -la /data/test-file.txt 2>/dev/null || echo "❌ Test file not found"
    
    # Vérifier sur l'hôte
    DATA_VOLUME=$(docker volume ls -q | grep garage_data)
    if [ -n "$DATA_VOLUME" ]; then
        MOUNT_POINT=$(docker volume inspect $DATA_VOLUME | jq -r '.[0].Mountpoint')
        echo "Checking on host volume ($MOUNT_POINT):"
        sudo ls -la "$MOUNT_POINT/test-file.txt" 2>/dev/null || ls -la "$MOUNT_POINT/test-file.txt" 2>/dev/null || echo "❌ Test file not found on host"
    fi
    
    # Nettoyer
    docker exec ocf-worker-garage rm -f /data/test-file.txt 2>/dev/null || true
else
    echo "❌ Cannot test - container not running"
fi

echo ""
echo "7️⃣ Possible Issues"
echo "------------------"

echo "🔍 Common problems:"

# Vérifier si les volumes sont bien mappés
if ! docker inspect ocf-worker-garage 2>/dev/null | grep -q "/data"; then
    echo "  ❌ /data volume not mounted correctly"
fi

if ! docker inspect ocf-worker-garage 2>/dev/null | grep -q "/meta"; then
    echo "  ❌ /meta volume not mounted correctly"  
fi

# Vérifier les permissions
echo ""
echo "📋 Permission check:"
if docker ps | grep -q "ocf-worker-garage"; then
    echo "Container user:"
    docker exec ocf-worker-garage whoami 2>/dev/null || echo "❌ Cannot get container user"
    echo "Directory permissions:"
    docker exec ocf-worker-garage ls -ld /data /meta 2>/dev/null || echo "❌ Cannot check permissions"
fi

echo ""
echo "==============================================="
echo "🎯 Diagnosis Summary"
echo "==============================================="

# Résumer les problèmes potentiels
ISSUES_FOUND=0

if ! docker volume ls | grep -q garage_data; then
    echo "❌ garage_data volume missing"
    ((ISSUES_FOUND++))
fi

if ! docker ps | grep -q "ocf-worker-garage"; then
    echo "❌ Garage container not running"
    ((ISSUES_FOUND++))
fi

if [ ! -f "deployments/garage/garage.toml" ]; then
    echo "❌ Garage configuration missing"
    ((ISSUES_FOUND++))
fi

if [ $ISSUES_FOUND -eq 0 ]; then
    echo "🤔 No obvious issues found - data might be stored but not visible"
    echo ""
    echo "💡 Possible explanations:"
    echo "  - Garage uses internal data format (not human-readable files)"
    echo "  - Data is compressed/encoded"
    echo "  - Files stored in database format"
    echo "  - Permission issues preventing visibility"
else
    echo "🚨 Found $ISSUES_FOUND potential issues"
fi

echo ""
echo "🔧 Recommended actions:"
echo "  1. Check volume mounts: docker inspect ocf-worker-garage"
echo "  2. Verify Garage is actually storing data: upload a file"
echo "  3. Check Garage logs: make garage-logs"
echo "  4. Restart with fresh volumes: make garage-reset"