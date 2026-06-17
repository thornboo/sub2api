#!/usr/bin/env bash
# =============================================================================
# Sub2API dev-zz deployment backup helper
# =============================================================================
# Creates a timestamped backup before updating the Docker deployment.
#
# Backed up:
#   - .env, .env.example, docker-compose*.yml, backup-dev-zz.sh
#   - PostgreSQL logical dump via pg_dump inside sub2api-postgres
#   - local app/Redis data directories when present: data/ redis_data/
#
# Not backed up:
#   - postgres_data/ raw files. Use the pg_dump artifact instead; raw PostgreSQL
#     directory copies are not a consistent online backup while the DB is running.
# =============================================================================
set -euo pipefail

DEPLOY_DIR="."
BACKUP_ROOT="backups"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-sub2api-postgres}"
SKIP_DB=false
SKIP_DATA=false

print_info() { printf '\033[0;34m[INFO]\033[0m %s\n' "$1"; }
print_success() { printf '\033[0;32m[SUCCESS]\033[0m %s\n' "$1"; }
print_warning() { printf '\033[1;33m[WARNING]\033[0m %s\n' "$1"; }
print_error() { printf '\033[0;31m[ERROR]\033[0m %s\n' "$1" >&2; }

usage() {
  cat <<'EOF'
Usage: ./backup-dev-zz.sh [options]

Create a safe deployment backup before updating sub2api dev-zz.

Options:
  --dir <path>                 Deployment directory. Default: current directory.
  --out <path>                 Backup root directory. Default: backups.
  --postgres-container <name>  PostgreSQL container name. Default: sub2api-postgres.
  --skip-db                    Skip PostgreSQL pg_dump.
  --skip-data                  Skip data/ and redis_data/ archive.
  -h, --help                   Show this help.

Environment:
  POSTGRES_CONTAINER           Same as --postgres-container.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dir)
      shift
      [ "$#" -gt 0 ] || { print_error "--dir requires a path."; exit 1; }
      DEPLOY_DIR="$1"
      ;;
    --out)
      shift
      [ "$#" -gt 0 ] || { print_error "--out requires a path."; exit 1; }
      BACKUP_ROOT="$1"
      ;;
    --postgres-container)
      shift
      [ "$#" -gt 0 ] || { print_error "--postgres-container requires a name."; exit 1; }
      POSTGRES_CONTAINER="$1"
      ;;
    --skip-db)
      SKIP_DB=true
      ;;
    --skip-data)
      SKIP_DATA=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      print_error "Unknown option: $1"
      usage
      exit 1
      ;;
  esac
  shift
done

command_exists() { command -v "$1" >/dev/null 2>&1; }
require_command() {
  command_exists "$1" || { print_error "$1 is required but was not found."; exit 1; }
}

read_env_value() {
  local name="$1"
  local default_value="${2:-}"
  local value=""

  if [ -f ".env" ]; then
    value="$(grep -E "^${name}=" .env | tail -n 1 | cut -d= -f2- || true)"
  fi

  value="${value%\"}"
  value="${value#\"}"
  value="${value%\'}"
  value="${value#\'}"

  if [ -n "$value" ]; then
    printf '%s' "$value"
  else
    printf '%s' "$default_value"
  fi
}

require_command tar
if [ "$SKIP_DB" = false ]; then
  require_command docker
fi

cd "$DEPLOY_DIR"

if [ ! -f ".env" ]; then
  print_error ".env was not found in $(pwd). Run this from the deployment directory or pass --dir."
  exit 1
fi

timestamp="$(date +%Y%m%d-%H%M%S)"
backup_dir="${BACKUP_ROOT%/}/manual-${timestamp}"
config_dir="${backup_dir}/config"
mkdir -p "$config_dir"

print_info "Creating backup: ${backup_dir}"

for file in .env .env.example backup-dev-zz.sh docker-compose.yml docker-compose.local.yml docker-compose.override.yml docker-compose.dev-zz.yml; do
  if [ -f "$file" ]; then
    cp -a "$file" "$config_dir/"
  fi
done

if [ "$SKIP_DB" = false ]; then
  postgres_user="$(read_env_value POSTGRES_USER sub2api)"
  postgres_db="$(read_env_value POSTGRES_DB sub2api)"
  if ! docker inspect "$POSTGRES_CONTAINER" >/dev/null 2>&1; then
    print_error "PostgreSQL container '${POSTGRES_CONTAINER}' was not found."
    print_error "Pass --postgres-container <name> if this deployment uses a custom container name."
    exit 1
  fi

  dump_file="${backup_dir}/sub2api-${timestamp}.dump"
  print_info "Dumping PostgreSQL database ${postgres_db} from ${POSTGRES_CONTAINER}"
  docker exec "$POSTGRES_CONTAINER" sh -c '
    : "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is not set in the PostgreSQL container}"
    export PGPASSWORD="$POSTGRES_PASSWORD"
    exec pg_dump -U "$1" -d "$2" -Fc
  ' sh "$postgres_user" "$postgres_db" > "$dump_file"
  print_success "Database dump: ${dump_file}"
else
  print_warning "Skipping PostgreSQL dump (--skip-db)."
fi

if [ "$SKIP_DATA" = false ]; then
  data_dirs=()
  for dir in data redis_data; do
    if [ -d "$dir" ]; then
      data_dirs+=("$dir")
    fi
  done

  if [ "${#data_dirs[@]}" -gt 0 ]; then
    data_file="${backup_dir}/app-data-${timestamp}.tgz"
    print_info "Archiving local data directories: ${data_dirs[*]}"
    tar czf "$data_file" "${data_dirs[@]}"
    print_success "Data archive: ${data_file}"
  else
    print_warning "No local data/ or redis_data/ directories found; skipped data archive."
  fi
else
  print_warning "Skipping local data archive (--skip-data)."
fi

cat > "${backup_dir}/README.txt" <<EOF
Sub2API dev-zz backup

Created at: ${timestamp}
Deploy dir: $(pwd)
PostgreSQL container: ${POSTGRES_CONTAINER}

Artifacts:
- config/: deployment config and env files
- sub2api-${timestamp}.dump: PostgreSQL custom-format dump, if --skip-db was not used
- app-data-${timestamp}.tgz: local data and redis_data archive, if present

Restore notes:
- Restore the PostgreSQL dump with pg_restore into a clean compatible database.
- Restore config files deliberately; do not overwrite a newer .env without checking secrets.
- postgres_data/ raw files are intentionally not archived while the DB is online.
EOF

print_success "Backup complete: ${backup_dir}"
