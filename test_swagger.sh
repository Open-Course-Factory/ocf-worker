#!/bin/bash
# scripts/test-swagger.sh

echo "🧪 Testing Swagger Documentation..."

# Démarrer le serveur en arrière-plan
echo "🚀 Starting OCF Worker..."
make run &
SERVER_PID=$!

# Attendre que le serveur soit prêt
echo "⏳ Waiting for server to start..."
for i in {1..30}; do
    if curl -s http://localhost:8081/health >/dev/null 2>&1; then
        echo "✅ Server is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ Server failed to start"
        kill $SERVER_PID
        exit 1
    fi
    sleep 1
done

# Tester les endpoints Swagger
echo "📊 Testing Swagger endpoints..."

# Test de l'interface Swagger UI
if curl -s http://localhost:8081/swagger/index.html | grep -q "Swagger UI"; then
    echo "✅ Swagger UI accessible"
else
    echo "❌ Swagger UI not accessible"
fi

# Test du JSON Swagger
if curl -s http://localhost:8081/swagger/doc.json | jq . >/dev/null 2>&1; then
    echo "✅ Swagger JSON valid"
    
    # Afficher quelques statistiques
    ENDPOINTS=$(curl -s http://localhost:8081/swagger/doc.json | jq -r '.paths | keys | length')
    MODELS=$(curl -s http://localhost:8081/swagger/doc.json | jq -r '.definitions | keys | length')
    echo "📈 API Statistics:"
    echo "   - Endpoints: $ENDPOINTS"
    echo "   - Models: $MODELS"
else
    echo "❌ Swagger JSON invalid"
fi

# Test des redirections
for endpoint in "/docs" "/api-docs" "/swagger.json"; do
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081$endpoint)
    if [ "$STATUS" = "301" ] || [ "$STATUS" = "200" ]; then
        echo "✅ Redirect $endpoint working ($STATUS)"
    else
        echo "❌ Redirect $endpoint failing ($STATUS)"
    fi
done

# Arrêter le serveur
echo "🛑 Stopping server..."
kill $SERVER_PID

echo "🎉 Swagger testing completed!"