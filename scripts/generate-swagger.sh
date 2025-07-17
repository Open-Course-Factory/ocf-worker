#!/bin/bash
# scripts/generate-swagger.sh

set -e

echo "📚 Generating OCF Worker API Documentation..."

# Vérifier que swag est installé
if ! command -v swag &> /dev/null; then
    echo "❌ swag is not installed. Installing..."
    go install github.com/swaggo/swag/cmd/swag@latest
fi

# Nettoyer les anciennes docs
echo "🧹 Cleaning old documentation..."
rm -rf docs/

# Générer la nouvelle documentation
echo "🔄 Generating new documentation..."
swag init \
    -g cmd/generator/main.go \
    -o docs \
    --parseInternal \
    --parseDependency \
    --markdownFiles README.md

# Vérifier que les fichiers ont été générés
if [ -f "docs/swagger.json" ] && [ -f "docs/swagger.yaml" ]; then
    echo "✅ Swagger documentation generated successfully!"
    echo ""
    echo "📁 Generated files:"
    echo "  - docs/swagger.json"
    echo "  - docs/swagger.yaml"
    echo "  - docs/docs.go"
    echo ""
    echo "🌐 To view the documentation:"
    echo "  1. Start the server: make run"
    echo "  2. Open: http://localhost:8081/swagger/"
    echo ""
    echo "📊 API Statistics:"
    echo "  - Endpoints: $(grep -c '"paths"' docs/swagger.json)"
    echo "  - Models: $(grep -c '"definitions"' docs/swagger.json)"
else
    echo "❌ Failed to generate Swagger documentation"
    exit 1
fi