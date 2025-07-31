# OCF Worker

[![Go Report Card](https://goreportcard.com/badge/github.com/Open-Course-Factory/ocf-worker-sdk)](https://goreportcard.com/report/github.com/Open-Course-Factory/ocf-worker)
[![License](https://img.shields.io/badge/license-AGPLv3-green.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.23+-blue.svg)](https://golang.org)

**OCF Worker** est un microservice Go pour la génération asynchrone de cours dans le cadre du projet **Open Course Factory (OCF)**. Il traite des jobs de génération de manière asynchrone avec une API REST et un système de storage abstrait supportant filesystem et Garage S3.

## 🚀 Fonctionnalités

- ✅ **API REST complète** pour la gestion des jobs de génération
- ✅ **Storage abstrait** avec backends filesystem et Garage S3
- ✅ **Upload multipart** pour les fichiers sources
- ✅ **Gestion asynchrone** des jobs avec statuts et progression
- ✅ **Base PostgreSQL** avec GORM et types JSON personnalisés
- ✅ **Docker ready** avec docker-compose pour le développement
- ✅ **Tests complets** unitaires et d'intégration
- ✅ **Cleanup automatique** des jobs anciens
- ✅ **Health checks** et monitoring

## 🏗️ Architecture

```shell
ocf-worker/
├── cmd/generator/main.go          # Point d'entrée
├── internal/
│   ├── api/                       # HTTP handlers et routes
│   │   ├── handlers.go           # Handlers pour jobs
│   │   ├── storage_handlers.go   # Handlers pour storage
│   │   └── router.go            # Configuration des routes
│   ├── config/                   # Configuration (env vars)
│   ├── database/                 # Connexion PostgreSQL + GORM
│   ├── jobs/                     # Service et repository pour jobs
│   └── storage/                  # Storage abstraction
│       ├── filesystem/           # Backend filesystem
│       └── garage/              # Backend Garage S3
├── pkg/
│   ├── models/                   # Models GORM avec types JSON
│   └── storage/                  # Interface storage
├── deployments/docker/           # Docker + docker-compose
├── scripts/                      # Scripts d'aide et de test
└── tests/                       # Tests d'intégration
```

## 📊 API Endpoints

### Jobs de génération

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/v1/generate` | Créer un nouveau job |
| `GET` | `/api/v1/jobs/{id}` | Statut d'un job |
| `GET` | `/api/v1/jobs` | Liste des jobs (avec filtres) |

### Storage des fichiers

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `POST` | `/api/v1/storage/jobs/{job_id}/sources` | Upload fichiers sources |
| `GET` | `/api/v1/storage/jobs/{job_id}/sources` | Liste fichiers sources |
| `GET` | `/api/v1/storage/jobs/{job_id}/sources/{filename}` | Download fichier source |
| `GET` | `/api/v1/storage/courses/{course_id}/results` | Liste résultats |
| `GET` | `/api/v1/storage/courses/{course_id}/results/{filename}` | Download résultat |
| `GET` | `/api/v1/storage/jobs/{job_id}/logs` | Logs d'un job |

### Monitoring

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/storage/info` | Information storage |

## 🛠️ Installation et Démarrage

### Prérequis

- **Go 1.23+**
- **Docker & Docker Compose**
- **PostgreSQL 15** (via Docker)

### Démarrage rapide

```bash
# 1. Cloner le projet
git clone https://github.com/your-org/ocf-worker.git
cd ocf-worker

# 2. Copier la configuration
cp .env.example .env

# 3. Démarrer avec Docker Compose
docker-compose up -d

# 4. Vérifier que tout fonctionne
curl http://localhost:8081/health
```

### Configuration

#### Variables d'environnement

```bash
# Serveur
PORT=8081
LOG_LEVEL=info
ENVIRONMENT=development

# Base de données
DATABASE_URL=postgres://ocf_worker:password@postgres-worker:5432/ocf_worker_db?sslmode=disable

# Storage (filesystem par défaut)
STORAGE_TYPE=filesystem
STORAGE_PATH=./storage

# Ou storage Garage S3
STORAGE_TYPE=garage
GARAGE_ENDPOINT=https://s3.garage.example.com
GARAGE_ACCESS_KEY=your_access_key
GARAGE_SECRET_KEY=your_secret_key
GARAGE_BUCKET=ocf-courses
GARAGE_REGION=us-east-1

# Jobs
JOB_TIMEOUT=30m
CLEANUP_INTERVAL=1h
```

## 🧪 Tests

### Tests unitaires

```bash
# Tous les tests
make test

# Tests spécifiques
go test -v ./internal/api/
go test -v ./internal/storage/filesystem/
go test -v ./internal/storage/garage/
go test -v ./internal/config/
```

### Tests d'intégration

```bash
# Test de l'API storage avec filesystem
./test_storage_api.sh

# Test avec Garage S3
./test_storage_api.sh garage

# Test complet avec Garage
./scripts/test-garage-integration.sh
```

### Tests automatisés

```bash
# Script complet de test
./test_api.sh
```

## 📦 Storage Backends

### Filesystem Storage

Storage local sur le système de fichiers.

**Configuration :**

```bash
STORAGE_TYPE=filesystem
STORAGE_PATH=./storage
```

**Structure :**

```shell
storage/
├── sources/{job_id}/
├── results/{course_id}/
└── logs/{job_id}/
```

### Garage S3 Storage

Storage distribué S3-compatible avec [Garage](https://garagehq.deuxfleurs.fr/).

**Configuration :**

```bash
STORAGE_TYPE=garage
GARAGE_ENDPOINT=https://s3.garage.example.com
GARAGE_ACCESS_KEY=GK31c2f218a2e44f485b94239e
GARAGE_SECRET_KEY=4420d99ef7aa26b56b5130ad7913a6a5c77653a5e7a47a3b4c9b8b9c5f8b7b4d
GARAGE_BUCKET=ocf-courses
GARAGE_REGION=garage
```

**Avantages :**

- ✅ Distribué et répliqué
- ✅ Compatible AWS S3
- ✅ URLs présignées
- ✅ Auto-hébergé

## 🐳 Docker

### Développement

```bash
# Démarrage standard
make docker-dev

# Avec hot reload
make docker-dev-hot

# Logs
make logs
```

### Production

```bash
# Copier la configuration
cp .env.example .env.prod

# Configurer les variables
vim .env.prod

# Démarrer en production
make docker-prod
```

### Images Docker

- **Base** : `golang:1.23` + `node:24-bookworm-slim`
- **Runtime** : Utilisateur non-root pour la sécurité
- **Health checks** : Intégrés
- **Volumes** : Storage et logs persistants

## 📊 Modèles de données

### GenerationJob

```go
type GenerationJob struct {
    ID          uuid.UUID   `json:"id"`
    CourseID    uuid.UUID   `json:"course_id"`
    Status      JobStatus   `json:"status"`          // pending, processing, completed, failed, timeout
    Progress    int         `json:"progress"`        // 0-100
    SourcePath  string      `json:"source_path"`
    ResultPath  string      `json:"result_path"`
    CallbackURL string      `json:"callback_url"`
    Error       string      `json:"error,omitempty"`
    Logs        StringSlice `json:"logs"`            // JSONB array
    Metadata    JSON        `json:"metadata"`        // JSONB object
    CreatedAt   time.Time   `json:"created_at"`
    UpdatedAt   time.Time   `json:"updated_at"`
    StartedAt   *time.Time  `json:"started_at,omitempty"`
    CompletedAt *time.Time  `json:"completed_at,omitempty"`
}
```

### Types JSON personnalisés

- **`JSON`** : `map[string]interface{}` avec support PostgreSQL JSONB
- **`StringSlice`** : `[]string` avec support PostgreSQL JSONB

## 🔄 Workflow d'utilisation

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant DB
    participant Storage
    participant Worker

    Client->>API: POST /api/v1/generate
    API->>DB: Créer job (status: pending)
    API-->>Client: Job créé (job_id)
    
    Client->>API: POST /api/v1/storage/jobs/{id}/sources
    API->>Storage: Upload fichiers sources
    Storage-->>API: Upload confirmé
    
    Worker->>DB: Récupérer jobs pending
    Worker->>Storage: Download sources
    Worker->>Worker: Exécuter Slidev build
    Worker->>Storage: Upload résultats
    Worker->>DB: Mettre à jour status (completed)
    
    Client->>API: GET /api/v1/jobs/{id}
    API->>DB: Récupérer status
    API-->>Client: Status job
    
    Client->>API: GET /api/v1/storage/courses/{id}/results
    API->>Storage: Liste résultats
    API-->>Client: Liste fichiers
```

## 🔧 Développement

### Structure du code

- **`cmd/`** : Points d'entrée des applications
- **`internal/`** : Code interne à l'application
- **`pkg/`** : Packages réutilisables
- **`deployments/`** : Configuration Docker
- **`scripts/`** : Scripts d'aide

### Ajout d'un nouveau backend storage

1. Créer `internal/storage/newbackend/storage.go`
2. Implémenter l'interface `storage.Storage`
3. Ajouter le backend dans `factory.go`
4. Créer les tests dans `storage_test.go`
5. Mettre à jour la configuration

### Guidelines

- **Tests** : Couverture > 80%
- **Logs** : Structured logging avec niveaux
- **Erreurs** : Wrapping avec context
- **API** : REST avec JSON
- **Docker** : Multi-stage builds

## 🚦 État du projet

### ✅ Terminé

- [x] API REST complète
- [x] Storage filesystem et Garage
- [x] Modèles avec types JSON
- [x] Base PostgreSQL avec GORM
- [x] Docker et docker-compose
- [x] Tests unitaires et d'intégration
- [x] Scripts de test automatisés
- [x] Worker de génération Slidev
- [x] Traitement asynchrone des jobs
- [x] Génération des résultats
- [x] Webhooks de notification

### 📋 À venir

- [ ] Monitoring avec Prometheus
- [ ] Métriques et alertes
- [ ] Scaling horizontal
- [ ] Cache Redis
- [ ] Rate limiting

## 🤝 Contribution

1. **Fork** le projet
2. **Créer** une branche feature (`git checkout -b feature/amazing-feature`)
3. **Commit** les changements (`git commit -m 'Add amazing feature'`)
4. **Push** vers la branche (`git push origin feature/amazing-feature`)
5. **Ouvrir** une Merge Request

### Règles de contribution

- Tests obligatoires pour toute nouvelle fonctionnalité
- Documentation mise à jour
- Code formaté avec `go fmt`
- Linting avec `golangci-lint`

## 📄 Licence

Ce projet est sous licence **GNU AGPL 3.0**. Voir le fichier [LICENSE](LICENSE) pour plus de détails.

## 🆘 Support

- **Issues** : [Gitlab Issues](https://usine.solution-libre.fr/open-course-factory/ocf-worker/-/issues)

## 🙏 Remerciements

- [Gin](https://gin-gonic.com/) pour le framework web
- [GORM](https://gorm.io/) pour l'ORM PostgreSQL
- [AWS SDK Go v2](https://aws.github.io/aws-sdk-go-v2/) pour S3
- [Garage](https://garagehq.deuxfleurs.fr/) pour le storage distribué
- [Slidev](https://sli.dev/) pour la génération de présentations

---

Made with ❤️ by the OCF Team
