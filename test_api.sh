#!/bin/bash
set -e

echo "🧪 Running API tests..."

# Tests unitaires
echo "📝 Running unit tests..."
go test -v ./internal/api/

# Tests d'intégration (optionnel)
echo "🔗 Running integration tests..."
go test -v ./internal/storage/filesystem/
go test -v ./internal/config/

echo "✅ All tests completed!"
