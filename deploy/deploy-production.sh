#!/usr/bin/env bash
# =============================================================================
# Sub2API Production Verification & Deployment Script
# =============================================================================
# This script automates the process of:
#   1. Loading production environment variables
#   2. Initializing the database and user in the PostgreSQL container
#   3. Setting up the .env file
#   4. Deploying using docker-compose.production.yml
# =============================================================================

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env.production"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.production.yml"
DB_CONTAINER="uds-postgres-prd"

if [ ! -f "${ENV_FILE}" ]; then
    print_error ".env.production not found in ${SCRIPT_DIR}"
    exit 1
fi

# Load database config from .env.production
# Use grep/sed to avoid shell variable collision
DB_USER=$(grep "^DATABASE_USER=" "${ENV_FILE}" | cut -d'=' -f2)
DB_PASS=$(grep "^DATABASE_PASSWORD=" "${ENV_FILE}" | cut -d'=' -f2)
DB_NAME=$(grep "^DATABASE_DBNAME=" "${ENV_FILE}" | cut -d'=' -f2)

print_info "Using Database User: ${DB_USER}, Database: ${DB_NAME}"

# 1. Initialize Database
print_info "Checking if ${DB_CONTAINER} is running..."
if ! docker ps --format '{{.Names}}' | grep -q "^${DB_CONTAINER}$"; then
    print_error "PostgreSQL container '${DB_CONTAINER}' is not running."
    exit 1
fi

print_info "Initializing Database and User..."
# Create User if not exists
docker exec "${DB_CONTAINER}" psql -U postgres -tc "SELECT 1 FROM pg_roles WHERE rolname = '${DB_USER}'" | grep -q 1 || \
    docker exec "${DB_CONTAINER}" psql -U postgres -c "CREATE USER ${DB_USER} WITH PASSWORD '${DB_PASS}';"

# Create Database if not exists
docker exec "${DB_CONTAINER}" psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'" | grep -q 1 || \
    docker exec "${DB_CONTAINER}" psql -U postgres -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};"

# Grant Privileges
docker exec "${DB_CONTAINER}" psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"

print_success "Database initialization completed."

# 2. Setup .env
print_info "Setting up .env file..."
cp "${ENV_FILE}" "${SCRIPT_DIR}/.env"
print_success ".env file updated."

# 3. Deploy
print_info "Deploying application using docker-compose.production.yml..."
docker compose -f "${COMPOSE_FILE}" up -d --remove-orphans

print_success "Deployment successful! Checking status..."
docker ps --filter "name=sub2api-prd"
print_info "To view logs, run: docker logs -f sub2api-prd"
