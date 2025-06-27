-- Script d'initialisation pour PostgreSQL du worker
-- Ce script est exécuté automatiquement au premier démarrage

-- Créer des extensions utiles
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";

-- Créer un utilisateur en lecture seule pour monitoring (optionnel)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'ocf_worker_readonly') THEN
        CREATE ROLE ocf_worker_readonly;
    END IF;
END
$$;

-- Accorder les permissions de lecture
GRANT CONNECT ON DATABASE ocf_worker_db TO ocf_worker_readonly;
GRANT USAGE ON SCHEMA public TO ocf_worker_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO ocf_worker_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO ocf_worker_readonly;

-- Configuration pour les performances
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET max_connections = 200;
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
ALTER SYSTEM SET maintenance_work_mem = '64MB';
ALTER SYSTEM SET checkpoint_completion_target = 0.9;
ALTER SYSTEM SET wal_buffers = '16MB';
ALTER SYSTEM SET default_statistics_target = 100;

-- Note: Ces changements nécessitent un redémarrage de PostgreSQL
-- mais ils ne sont appliqués qu'au premier démarrage
