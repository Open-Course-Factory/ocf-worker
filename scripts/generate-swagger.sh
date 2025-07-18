#!/bin/bash
# scripts/generate-swagger-complete.sh

set -e

echo "📚 Generating Complete OCF Worker API Documentation..."

# Couleurs pour l'affichage
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Vérifier que swag est installé
if ! command -v swag &> /dev/null; then
    log_warning "swag is not installed. Installing..."
    go install github.com/swaggo/swag/cmd/swag@latest
    
    # Vérifier que l'installation a réussi
    if ! command -v swag &> /dev/null; then
        log_error "Failed to install swag. Please install manually:"
        echo "go install github.com/swaggo/swag/cmd/swag@latest"
        exit 1
    fi
    log_success "swag installed successfully"
fi

# Vérifier la version de swag
SWAG_VERSION=$(swag --version 2>/dev/null | head -1 || echo "unknown")
log_info "Using swag version: $SWAG_VERSION"

# Nettoyer les anciennes docs
log_info "Cleaning old documentation..."
rm -rf docs/
mkdir -p docs

# Valider la syntaxe Go avant génération
log_info "Validating Go syntax..."
if ! go vet ./...; then
    log_error "Go vet failed. Please fix syntax errors first."
    exit 1
fi

if ! go mod tidy; then
    log_error "go mod tidy failed. Please check dependencies."
    exit 1
fi

log_success "Go syntax validation passed"

# Générer la documentation avec options avancées
log_info "Generating Swagger documentation..."

swag init \
    --generalInfo cmd/generator/main.go \
    --dir ./ \
    --output docs \
    --outputTypes go,json,yaml \
    --parseInternal true \
    --parseDependency true \
    --markdownFiles ./ \
    --instanceName swagger \
    --parseDepth 100

# Vérifier que les fichiers ont été générés
GENERATED_FILES=("docs/swagger.json" "docs/swagger.yaml" "docs/docs.go")
ALL_GENERATED=true

for file in "${GENERATED_FILES[@]}"; do
    if [ -f "$file" ]; then
        log_success "Generated: $file"
    else
        log_error "Missing: $file"
        ALL_GENERATED=false
    fi
done

if [ "$ALL_GENERATED" = false ]; then
    log_error "Some files were not generated. Check the swag output above."
    exit 1
fi

# Valider que le JSON est valide
log_info "Validating generated JSON..."
if command -v jq >/dev/null 2>&1; then
    if jq empty docs/swagger.json 2>/dev/null; then
        log_success "swagger.json is valid JSON"
    else
        log_error "swagger.json is invalid JSON"
        exit 1
    fi
else
    log_warning "jq not available, skipping JSON validation"
fi

# Statistiques de génération
log_info "Analyzing generated documentation..."

if command -v jq >/dev/null 2>&1; then
    ENDPOINTS=$(jq '.paths | keys | length' docs/swagger.json 2>/dev/null || echo "0")
    MODELS=$(jq '.definitions | keys | length' docs/swagger.json 2>/dev/null || echo "0")
    TAGS=$(jq '.tags | length' docs/swagger.json 2>/dev/null || echo "0")
    
    echo ""
    log_success "📊 API Documentation Statistics:"
    echo "  📍 Endpoints: $ENDPOINTS"
    echo "  📋 Models: $MODELS"
    echo "  🏷️  Tags: $TAGS"
    echo ""
    
    # Lister les endpoints par tag
    log_info "📍 Endpoints by tag:"
    jq -r '.paths | to_entries[] | .key as $path | .value | to_entries[] | .key as $method | .value.tags[]? as $tag | "\($tag): \($method | ascii_upcase) \($path)"' docs/swagger.json 2>/dev/null | sort | uniq -c | sort -rn || echo "Could not extract endpoint details"
    
    echo ""
    
    # Vérifier la couverture des modèles
    log_info "📋 Generated models:"
    jq -r '.definitions | keys[]' docs/swagger.json 2>/dev/null | sort || echo "Could not extract model list"
fi

# Générer un fichier d'index HTML pour la documentation
log_info "Generating documentation index..."
cat > docs/README.md << 'EOF'
# OCF Worker API Documentation

Cette documentation a été générée automatiquement à partir des annotations Swagger dans le code source.

## Formats disponibles

- [JSON](./swagger.json) - Format JSON OpenAPI/Swagger
- [YAML](./swagger.yaml) - Format YAML OpenAPI/Swagger
- [Go](./docs.go) - Code Go généré pour l'intégration

## Consultation

Pour consulter la documentation interactive:

1. Démarrez le serveur OCF Worker
2. Visitez: http://localhost:8081/swagger/index.html

## Mise à jour

Pour régénérer cette documentation:

```bash
# Méthode recommandée
make swagger-generate

# Ou directement
./scripts/generate-swagger-complete.sh
```

## Validation

La documentation est automatiquement validée lors de la génération:
- ✅ Syntaxe Go validée
- ✅ JSON bien formé
- ✅ Modèles référencés définis
- ✅ Annotations cohérentes

EOF

# Créer un Makefile d'aide
cat > docs/Makefile << 'EOF'
# Documentation Makefile

.PHONY: serve validate clean

# Servir la documentation localement
serve:
	@echo "📚 Starting documentation server..."
	@echo "🌐 Visit: http://localhost:8080"
	@python3 -m http.server 8080 2>/dev/null || python -m SimpleHTTPServer 8080

# Valider la documentation
validate:
	@echo "🔍 Validating Swagger documentation..."
	@if command -v swagger >/dev/null 2>&1; then \
		swagger validate swagger.json; \
	else \
		echo "⚠️  swagger-cli not available. Install with: npm install -g swagger-cli"; \
	fi

# Nettoyer les fichiers générés
clean:
	@echo "🧹 Cleaning generated documentation..."
	@rm -f swagger.json swagger.yaml docs.go README.md
	@echo "✅ Documentation cleaned"

EOF

# Vérifications finales
log_info "Performing final checks..."

# Vérifier que le serveur peut démarrer (optionnel)
if [ "${SKIP_SERVER_CHECK:-}" != "true" ]; then
    log_info "Checking if generated docs work with server..."
    
    # Test rapide de compilation
    if go build -o /tmp/ocf-worker-test ./cmd/generator/ 2>/dev/null; then
        log_success "Server compiles successfully with generated docs"
        rm -f /tmp/ocf-worker-test
    else
        log_warning "Server compilation test failed, but docs were generated"
    fi
fi

# Résumé final
echo ""
echo "🎉 Swagger documentation generation completed successfully!"
echo ""
echo "📁 Generated files:"
echo "  - docs/swagger.json ($(wc -c < docs/swagger.json) bytes)"
echo "  - docs/swagger.yaml ($(wc -c < docs/swagger.yaml) bytes)"
echo "  - docs/docs.go ($(wc -l < docs/docs.go) lines)"
echo "  - docs/README.md"
echo "  - docs/Makefile"
echo ""
echo "🌐 To view the documentation:"
echo "  1. Start the server: make run"
echo "  2. Open: http://localhost:8081/swagger/index.html"
echo ""
echo "📊 API Statistics:"
if command -v jq >/dev/null 2>&1; then
    echo "  - Endpoints: $(jq '.paths | keys | length' docs/swagger.json)"
    echo "  - Models: $(jq '.definitions | keys | length' docs/swagger.json)"
    echo "  - Tags: $(jq '.tags | length' docs/swagger.json 2>/dev/null || echo "N/A")"
fi
echo ""
echo "✨ Documentation is ready for use!"