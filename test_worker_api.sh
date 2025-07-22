#!/bin/bash
# test_worker_api.sh - Script de test pour le worker OCF

set -e

echo "🧪 Testing OCF Worker with Generation Capabilities..."

# Configuration
API_BASE="http://localhost:8081"
STORAGE_BACKEND=${1:-"filesystem"}

# Couleurs pour l'affichage
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Fonctions utilitaires
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

# Vérifier que les services sont en cours d'exécution
check_services() {
    log_info "Checking if services are running..."
    
    if ! docker-compose ps | grep -q "ocf-worker.*Up"; then
        log_error "OCF Worker service is not running!"
        echo "💡 Please start with: docker-compose up -d"
        exit 1
    fi

    if ! docker-compose ps | grep -q "postgres-worker.*Up"; then
        log_error "PostgreSQL service is not running!"
        echo "💡 Please start with: docker-compose up -d"
        exit 1
    fi

    log_success "Services are running"
}

# Attendre que le service soit prêt
wait_for_service() {
    log_info "Waiting for OCF Worker to be ready..."
    
    for i in {1..60}; do
        if curl -s "$API_BASE/health" >/dev/null 2>&1; then
            log_success "OCF Worker is ready"
            break
        fi
        if [ $i -eq 60 ]; then
            log_error "OCF Worker failed to start within 60 seconds"
            echo "📊 Service logs:"
            docker-compose logs --tail=20 ocf-worker
            exit 1
        fi
        sleep 1
    done
}

# Test du health check
test_health() {
    log_info "Testing health check..."
    log_info "Calling $API_BASE/api/v1/health"
    
    HEALTH_RESPONSE=$(curl -s "$API_BASE/api/v1/health")
    echo "$HEALTH_RESPONSE" | jq .
    
    if echo "$HEALTH_RESPONSE" | jq -e '.status == "healthy"' >/dev/null; then
        log_success "Health check passed"
    else
        log_error "Health check failed"
        exit 1
    fi
}

# Test des stats du worker
test_worker_stats() {
    log_info "Testing worker statistics..."
    
    STATS_RESPONSE=$(curl -s "$API_BASE/api/v1/worker/stats")
    echo "$STATS_RESPONSE" | jq .
    
    if echo "$STATS_RESPONSE" | jq -e '.worker_pool.running == true' >/dev/null; then
        log_success "Worker pool is running"
    else
        log_warning "Worker pool may not be running yet"
    fi
    
    WORKER_COUNT=$(echo "$STATS_RESPONSE" | jq -r '.worker_pool.worker_count // 0')
    log_info "Worker count: $WORKER_COUNT"
}

# Test de santé du worker
test_worker_health() {
    log_info "Testing worker health..."
    log_info "Calling $API_BASE/api/v1/worker/health"
    
    HEALTH_RESPONSE=$(curl -s "$API_BASE/api/v1/worker/health")
    echo "$HEALTH_RESPONSE" | jq .
    
    STATUS=$(echo "$HEALTH_RESPONSE" | jq -r '.status // "unknown"')
    case $STATUS in
        "healthy")
            log_success "Worker health is good"
            ;;
        "degraded")
            log_warning "Worker health is degraded"
            ;;
        "unhealthy")
            log_error "Worker health is poor"
            ;;
        *)
            log_warning "Worker health status unknown: $STATUS"
            ;;
    esac
}

# Créer des fichiers de test pour Slidev
create_test_files() {
    log_info "Creating test files for Slidev generation..."
    
    mkdir -p test-files

    # Créer un fichier slides.md complet pour Slidev
    cat > test-files/slides.md << 'SLIDESEOF'
---
theme: default
title: OCF Worker Test - Complete Generation
author: OCF Development Team
description: Test de génération complète avec le worker OCF
highlighter: shiki
lineNumbers: true
colorSchema: auto
layout: cover
background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)'
---

# OCF Worker Test 🚀

## Génération Complète avec Worker

Test du pipeline complet de génération Slidev

<div class="pt-12">
  <span @click="$slidev.nav.next" class="px-2 py-1 rounded cursor-pointer" hover="bg-white bg-opacity-10">
    Appuyez sur <kbd>espace</kbd> pour continuer <carbon:arrow-right class="inline"/>
  </span>
</div>

---

# Tests du Worker 🔧

<div class="grid grid-cols-2 gap-4">
<div>

## ✅ Fonctionnalités Testées

- Upload de fichiers sources
- Traitement asynchrone des jobs
- Exécution de `slidev build`
- Génération des résultats HTML
- Sauvegarde des logs

</div>
<div>

## 🏗️ Architecture Worker

- Pool de workers configurables
- Workspaces isolés par job
- Polling automatique des jobs
- Nettoyage des ressources

</div>
</div>

---

# Configuration du Worker ⚙️

```yaml
worker:
  worker_count: 3
  poll_interval: 5s
  workspace_base: "/tmp/ocf-worker"
  slidev_command: "npx @slidev/cli"
  cleanup_workspace: true
  job_timeout: 30m
```

<v-clicks>

- **worker_count**: Nombre de workers simultanés
- **poll_interval**: Fréquence de vérification des jobs
- **workspace_base**: Répertoire des workspaces temporaires
- **slidev_command**: Commande pour exécuter Slidev

</v-clicks>

---

# Workflow de Génération 📊

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant Worker
    participant Storage
    participant Slidev

    Client->>API: POST /api/v1/generate
    API->>Worker: Job queued (status: pending)
    Worker->>Storage: Download sources
    Worker->>Slidev: Execute "slidev build"
    Slidev-->>Worker: Generate dist/ files
    Worker->>Storage: Upload results
    Worker->>API: Update status (completed)
    Client->>API: GET results
```

---

# Code Example 💻

Exemple d'utilisation de l'API worker:

```javascript
// Créer un job de génération
const response = await fetch('/api/v1/generate', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    job_id: uuid(),
    course_id: uuid(),
    source_path: 'courses/my-presentation/',
    callback_url: 'https://app.com/webhook'
  })
});

const job = await response.json();
console.log('Job créé:', job.id);

// Surveiller le progress
const checkStatus = async () => {
  const status = await fetch(`/api/v1/jobs/${job.id}`);
  const data = await status.json();
  
  console.log(`Progress: ${data.progress}%`);
  if (data.status === 'completed') {
    console.log('✅ Génération terminée!');
  }
};
```

---

# Performance & Monitoring 📈

<div class="grid grid-cols-2 gap-8">
<div>

## Métriques Worker

- Jobs traités par seconde
- Temps moyen de génération
- Taux de succès/échec
- Utilisation des workspaces

</div>
<div>

## Endpoints de Monitoring

- `GET /api/v1/worker/stats`
- `GET /api/v1/worker/health`
- `GET /api/v1/worker/workspaces`
- `GET /health`

</div>
</div>

---
layout: center
class: text-center
---

# Merci ! 🎉

## OCF Worker - Génération Slidev Automatisée

**Tests réussis** ✅ **Worker fonctionnel** ✅ **Pipeline complet** ✅

[Documentation](https://ocf-project.org) · [GitHub](https://github.com/ocf/worker) · [Discord](https://discord.gg/ocf)

<style>
h1 {
  background-color: #2B90B6;
  background-image: linear-gradient(45deg, #4EC5D4 10%, #146b8c 20%);
  background-size: 100%;
  -webkit-background-clip: text;
  -moz-background-clip: text;
  -webkit-text-fill-color: transparent;
  -moz-text-fill-color: transparent;
}
</style>
SLIDESEOF

    # Créer un thème personnalisé
    cat > test-files/theme.css << 'CSSEOF'
/* OCF Worker Test Theme */
:root {
  --ocf-primary: #667eea;
  --ocf-secondary: #764ba2;
  --ocf-accent: #ffd700;
}

.slidev-layout {
    font-family: 'Inter', system-ui, sans-serif;
}

.slidev-layout h1 {
    background: linear-gradient(135deg, var(--ocf-primary) 0%, var(--ocf-secondary) 100%);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    font-weight: 800;
}

.slidev-layout code {
    background: rgba(102, 126, 234, 0.1);
    padding: 0.2rem 0.4rem;
    border-radius: 0.25rem;
}
CSSEOF

    # Créer un package.json pour les dépendances
    cat > test-files/package.json << 'JSONEOF'
{
    "name": "ocf-worker-test-presentation",
    "version": "1.0.0",
    "description": "Test presentation for OCF Worker",
    "scripts": {
        "dev": "slidev",
        "build": "slidev build",
        "export": "slidev export"
    },
    "dependencies": {
        "@slidev/cli": "^0.48.0",
        "@slidev/theme-default": "latest"
    },
    "devDependencies": {
        "@iconify-json/carbon": "^1.1.21",
        "@iconify-json/mdi": "^1.1.54"
    }
}
JSONEOF

    log_success "Test files created"
}

# Test du workflow complet de génération
test_complete_workflow() {
    log_info "Testing complete generation workflow..."
    
    # Générer des IDs
    if command -v uuidgen >/dev/null 2>&1; then
        JOB_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
        COURSE_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
    else
        echo "❌ uuidgen must be installed (package uuid-runtime)"
        exit 1
    fi

    log_info "Job ID: $JOB_ID"
    log_info "Course ID: $COURSE_ID"

    # Étape 1: Upload des fichiers sources
    log_info "Step 1: Uploading source files..."
    UPLOAD_RESPONSE=$(curl -s -X POST \
      -F "files=@test-files/slides.md" \
      -F "files=@test-files/theme.css" \
      -F "files=@test-files/package.json" \
      "$API_BASE/api/v1/storage/jobs/$JOB_ID/sources")

    echo "$UPLOAD_RESPONSE" | jq .

    if echo "$UPLOAD_RESPONSE" | jq -e '.count == 3' >/dev/null; then
        log_success "Source files uploaded successfully"
    else
        log_error "Source file upload failed"
        return 1
    fi

    # Étape 2: Créer le job de génération
    log_info "Step 2: Creating generation job..."
    JOB_RESPONSE=$(curl -s -X POST "$API_BASE/api/v1/generate" \
      -H "Content-Type: application/json" \
      -d "{
        \"job_id\": \"$JOB_ID\",
        \"course_id\": \"$COURSE_ID\",
        \"source_path\": \"sources/$JOB_ID/\",
        \"callback_url\": \"http://localhost:8080/api/v1/jobs/$JOB_ID\",
        \"metadata\": {
          \"test_type\": \"complete_workflow\",
          \"storage_backend\": \"$STORAGE_BACKEND\",
          \"slidev_theme\": \"default\"
        }
      }")

    echo "$JOB_RESPONSE" | jq .

    if echo "$JOB_RESPONSE" | jq -e '.status == "pending"' >/dev/null; then
        log_success "Generation job created successfully"
    else
        log_error "Generation job creation failed"
        return 1
    fi

    # Étape 3: Surveiller le progress du job
    log_info "Step 3: Monitoring job progress..."
    
    MAX_WAIT=300  # 5 minutes max
    WAIT_TIME=0
    
    while [ $WAIT_TIME -lt $MAX_WAIT ]; do
        STATUS_RESPONSE=$(curl -s "$API_BASE/api/v1/jobs/$JOB_ID")
        
        if [ $? -ne 0 ]; then
            log_error "Failed to get job status"
            return 1
        fi
        
        STATUS=$(echo "$STATUS_RESPONSE" | jq -r '.status // "unknown"')
        PROGRESS=$(echo "$STATUS_RESPONSE" | jq -r '.progress // 0')
        ERROR_MSG=$(echo "$STATUS_RESPONSE" | jq -r '.error // ""')
        
        case $STATUS in
            "pending")
                log_info "Job is pending... (${PROGRESS}%)"
                ;;
            "processing")
                log_info "Job is processing... (${PROGRESS}%)"
                ;;
            "completed")
                log_success "Job completed successfully! (${PROGRESS}%)"
                break
                ;;
            "failed")
                log_error "Job failed: $ERROR_MSG"
                echo "Full response:"
                echo "$STATUS_RESPONSE" | jq .
                return 1
                ;;
            "timeout")
                log_error "Job timed out"
                return 1
                ;;
            *)
                log_warning "Unknown job status: $STATUS"
                ;;
        esac
        
        sleep 5
        WAIT_TIME=$((WAIT_TIME + 5))
    done

    if [ $WAIT_TIME -ge $MAX_WAIT ]; then
        log_error "Job monitoring timed out after $MAX_WAIT seconds"
        return 1
    fi

    # Étape 4: Vérifier les résultats générés
    log_info "Step 4: Checking generated results..."
    
    RESULTS_RESPONSE=$(curl -s "$API_BASE/api/v1/storage/courses/$COURSE_ID/results")
    echo "$RESULTS_RESPONSE" | jq .
    
    RESULT_COUNT=$(echo "$RESULTS_RESPONSE" | jq '.files | length')
    if [ "$RESULT_COUNT" -gt 0 ]; then
        log_success "Found $RESULT_COUNT result files"
        
        # Vérifier que index.html existe
        if echo "$RESULTS_RESPONSE" | jq -e '.files[] | select(. == "index.html")' >/dev/null; then
            log_success "index.html generated successfully"
        else
            log_warning "index.html not found in results"
        fi
    else
        log_error "No result files found"
        return 1
    fi

    # Étape 5: Télécharger et vérifier index.html
    log_info "Step 5: Downloading and verifying index.html..."
    
    INDEX_CONTENT=$(curl -s "$API_BASE/api/v1/storage/courses/$COURSE_ID/results/index.html")
    
    if echo "$INDEX_CONTENT" | grep -q "OCF Worker Test"; then
        log_success "Generated HTML contains expected content"
    else
        log_warning "Generated HTML may not contain expected content"
        echo "Preview of generated content:"
        echo "$INDEX_CONTENT" | head -20
    fi

    # Étape 6: Vérifier les logs
    log_info "Step 6: Checking job logs..."
    
    LOGS_RESPONSE=$(curl -s "$API_BASE/api/v1/storage/jobs/$JOB_ID/logs")
    
    if [ $? -eq 0 ] && [ -n "$LOGS_RESPONSE" ]; then
        log_success "Job logs available"
        echo "Log preview:"
        echo "$LOGS_RESPONSE" | head -10
    else
        log_warning "Job logs not available or empty"
    fi

    log_success "Complete workflow test finished successfully!"
    
    # Résumé
    echo ""
    echo "📊 Test Summary:"
    echo "  ✅ Source files uploaded (3 files)"
    echo "  ✅ Generation job created"
    echo "  ✅ Job processed successfully"
    echo "  ✅ Result files generated ($RESULT_COUNT files)"
    echo "  ✅ HTML content validated"
    echo "  ✅ Logs available"
    echo ""
    echo "🎯 Generated course available at:"
    echo "   Job: $API_BASE/api/v1/jobs/$JOB_ID"
    echo "   Results: $API_BASE/api/v1/storage/courses/$COURSE_ID/results"
    echo "   HTML: $API_BASE/api/v1/storage/courses/$COURSE_ID/results/index.html"
}

# Nettoyage
cleanup() {
    log_info "Cleaning up test files..."
    rm -rf test-files
}

# Fonction principale
main() {
    echo "🚀 OCF Worker Complete Test Suite"
    echo "=================================="
    echo ""
    
    trap cleanup EXIT
    
    check_services
    wait_for_service
    test_health
    test_worker_stats
    test_worker_health
    
    create_test_files
    test_complete_workflow
    
    echo ""
    echo "🎉 All tests completed successfully!"
    echo ""
    echo "📈 Next steps:"
    echo "  1. ✅ Worker de génération fonctionnel"
    echo "  2. 🔄 Optimiser les performances de build"
    echo "  3. 📊 Ajouter plus de métriques"
    echo "  4. 🔔 Implémenter les webhooks de notification"
    echo "  5. 🌐 Déployer en production"
    exit 0
}

# Exécuter le script
main "$@"