#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-sub2api:dev-zz}"
BRANCH_NAME="${BRANCH_NAME:-dev-zz}"
START_STACK=true
BUILD_IMAGE=true
BUILD_ONLY=false
FORCE_ENV=false
FORCE_OVERRIDE=false
RUN_BACKUP=true

print_info() {
  printf '\033[0;34m[INFO]\033[0m %s\n' "$1"
}

print_success() {
  printf '\033[0;32m[SUCCESS]\033[0m %s\n' "$1"
}

print_warning() {
  printf '\033[1;33m[WARNING]\033[0m %s\n' "$1"
}

print_error() {
  printf '\033[0;31m[ERROR]\033[0m %s\n' "$1" >&2
}

usage() {
  cat <<'EOF'
Usage: ./deploy/deploy-dev-zz.sh [options]

Build and deploy the dev-zz secondary-development Docker image from source.

Options:
  --no-start        Build image and prepare deploy files, but do not start Docker Compose.
  --build-only      Only build the Docker image. Do not create deploy files or start Compose.
  --no-build        Skip docker build and only prepare/start Compose with the existing image.
  --skip-backup     Skip the pre-start deployment backup.
  --force-env       Recreate deploy/.env from deploy/.env.example and regenerate secrets.
  --force-override  Recreate deploy/docker-compose.override.yml.
  -h, --help        Show this help.

Environment:
  IMAGE_NAME        Docker image tag to build/use. Default: sub2api:dev-zz
  BRANCH_NAME       Expected Git branch. Default: dev-zz
  BACKUP_DIR        Backup output directory. Default: deploy/backups
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --no-start)
      START_STACK=false
      ;;
    --build-only)
      BUILD_ONLY=true
      BUILD_IMAGE=true
      START_STACK=false
      ;;
    --no-build)
      BUILD_IMAGE=false
      ;;
    --skip-backup)
      RUN_BACKUP=false
      ;;
    --force-env)
      FORCE_ENV=true
      ;;
    --force-override)
      FORCE_OVERRIDE=true
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

if [ "$BUILD_ONLY" = true ] && [ "$BUILD_IMAGE" = false ]; then
  print_error "--build-only and --no-build cannot be used together."
  exit 1
fi

if [[ "$IMAGE_NAME" =~ [[:space:]] ]]; then
  print_error "IMAGE_NAME must not contain whitespace."
  exit 1
fi

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

require_command() {
  if ! command_exists "$1"; then
    print_error "$1 is required but was not found."
    exit 1
  fi
}

generate_secret() {
  openssl rand -hex 32
}

replace_env_value() {
  key="$1"
  value="$2"
  file="$3"

  if grep -q "^${key}=" "$file"; then
    tmp="$(mktemp "${file}.tmp.XXXXXX")"
    awk -v key="$key" -v value="$value" '
      BEGIN { prefix = key "=" }
      index($0, prefix) == 1 { print key "=" value; next }
      { print }
    ' "$file" > "$tmp"
    mv "$tmp" "$file"
  else
    printf '%s=%s\n' "$key" "$value" >> "$file"
  fi
}

directory_has_entries() {
  dir="$1"
  [ -d "$dir" ] && find "$dir" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null | read -r _
}

has_existing_deployment_state() {
  target_deploy_dir="$1"
  [ -f "${target_deploy_dir}/.env" ] ||
    [ -f "${target_deploy_dir}/docker-compose.override.yml" ] ||
    directory_has_entries "${target_deploy_dir}/data" ||
    directory_has_entries "${target_deploy_dir}/postgres_data" ||
    directory_has_entries "${target_deploy_dir}/redis_data"
}

compose_service_running() {
  target_deploy_dir="$1"
  service="$2"
  container_id="$(
    cd "$target_deploy_dir"
    "${compose_cmd[@]}" ps -q "$service" 2>/dev/null | head -n 1
  )"
  [ -n "$container_id" ] && [ "$(docker inspect -f '{{.State.Running}}' "$container_id" 2>/dev/null || true)" = "true" ]
}

create_pre_start_backup() {
  target_deploy_dir="$1"

  if [ "$RUN_BACKUP" != true ] || [ "$START_STACK" != true ]; then
    return
  fi

  if ! has_existing_deployment_state "$target_deploy_dir"; then
    print_info "No existing deployment state found; backup skipped."
    return
  fi

  backup_root="${BACKUP_DIR:-${target_deploy_dir}/backups}"
  timestamp="$(date +%Y%m%d%H%M%S)"
  backup_path="${backup_root}/${timestamp}"
  mkdir -p "$backup_path"

  print_info "Creating pre-start backup: $backup_path"

  if compose_service_running "$target_deploy_dir" postgres; then
    (
      cd "$target_deploy_dir"
      "${compose_cmd[@]}" exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"'
    ) > "${backup_path}/postgres.sql"
    print_success "PostgreSQL logical backup created: ${backup_path}/postgres.sql"
  else
    print_warning "PostgreSQL service is not running; database pg_dump backup skipped."
    print_warning "For first deployment this is expected. For upgrades, start the existing stack before updating if you need a DB backup."
  fi

  files_to_archive=()
  [ -f "${target_deploy_dir}/.env" ] && files_to_archive+=(".env")
  [ -f "${target_deploy_dir}/docker-compose.override.yml" ] && files_to_archive+=("docker-compose.override.yml")
  [ -d "${target_deploy_dir}/data" ] && files_to_archive+=("data")

  if [ "${#files_to_archive[@]}" -gt 0 ]; then
    (
      cd "$target_deploy_dir"
      tar czf "${backup_path}/deploy-files.tar.gz" "${files_to_archive[@]}"
    )
    print_success "Deployment file backup created: ${backup_path}/deploy-files.tar.gz"
  else
    print_warning "No deployment files found to archive."
  fi

  print_info "Raw postgres_data/redis_data directories are not archived by default because live file-level database backups can be inconsistent. Use pg_dump for PostgreSQL."
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
deploy_dir="${repo_root}/deploy"

cd "$repo_root"

if [ ! -f "Dockerfile" ] || [ ! -f "deploy/docker-compose.local.yml" ] || [ ! -f "deploy/.env.example" ]; then
  print_error "Run this script from a complete sub2api source checkout. Required files are missing."
  exit 1
fi

require_command git
require_command docker
require_command openssl

if docker compose version >/dev/null 2>&1; then
  compose_base=(docker compose)
  compose_display="docker compose"
elif command_exists docker-compose; then
  compose_base=(docker-compose)
  compose_display="docker-compose"
else
  print_error "Docker Compose is required. Install either 'docker compose' or 'docker-compose'."
  exit 1
fi
print_info "Using Docker Compose command: ${compose_display}"

current_branch="$(git branch --show-current 2>/dev/null || true)"
if [ "$current_branch" != "$BRANCH_NAME" ]; then
  print_warning "Current Git branch is '${current_branch:-detached}', expected '$BRANCH_NAME'."
  print_warning "Continuing anyway. Set BRANCH_NAME to override the expected branch."
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  print_warning "Working tree has local changes. The Docker image will include current working tree content."
fi

if [ "$BUILD_IMAGE" = true ]; then
  print_info "Building Docker image: $IMAGE_NAME"
  docker build -t "$IMAGE_NAME" .
  print_success "Built Docker image: $IMAGE_NAME"
fi

if [ "$BUILD_ONLY" = true ]; then
  print_success "Build-only mode complete."
  exit 0
fi

mkdir -p "$deploy_dir/data" "$deploy_dir/postgres_data" "$deploy_dir/redis_data"

env_file="${deploy_dir}/.env"
if [ "$FORCE_ENV" = true ] || [ ! -f "$env_file" ]; then
  if [ "$FORCE_ENV" = true ] && [ -f "$env_file" ]; then
    backup="${env_file}.bak.$(date +%Y%m%d%H%M%S)"
    cp "$env_file" "$backup"
    print_warning "Existing deploy/.env backed up to ${backup}"
  fi

  print_info "Creating deploy/.env from deploy/.env.example"
  cp "${deploy_dir}/.env.example" "$env_file"
  replace_env_value "POSTGRES_PASSWORD" "$(generate_secret)" "$env_file"
  replace_env_value "JWT_SECRET" "$(generate_secret)" "$env_file"
  replace_env_value "TOTP_ENCRYPTION_KEY" "$(generate_secret)" "$env_file"
  chmod 600 "$env_file"
  print_success "Created deploy/.env with generated secrets"
else
  print_info "Reusing existing deploy/.env"
fi

override_file="${deploy_dir}/docker-compose.override.yml"
if [ "$FORCE_OVERRIDE" = true ] || [ ! -f "$override_file" ]; then
  if [ "$FORCE_OVERRIDE" = true ] && [ -f "$override_file" ]; then
    backup="${override_file}.bak.$(date +%Y%m%d%H%M%S)"
    cp "$override_file" "$backup"
    print_warning "Existing deploy/docker-compose.override.yml backed up to ${backup}"
  fi

  print_info "Creating deploy/docker-compose.override.yml"
  cat > "$override_file" <<EOF
services:
  sub2api:
    image: ${IMAGE_NAME}
EOF
  print_success "Created deploy/docker-compose.override.yml"
else
  print_info "Reusing existing deploy/docker-compose.override.yml"
fi

compose_cmd=(
  "${compose_base[@]}"
  -f docker-compose.local.yml
  -f docker-compose.override.yml
)

create_pre_start_backup "$deploy_dir"

if [ "$START_STACK" = true ]; then
  print_info "Starting Docker Compose stack"
  (
    cd "$deploy_dir"
    "${compose_cmd[@]}" up -d
    "${compose_cmd[@]}" up -d --no-deps --force-recreate sub2api
  )
  print_success "Sub2API stack started"
else
  print_info "Start skipped by --no-start."
fi

cat <<EOF

Deployment files:
  ${deploy_dir}/.env
  ${deploy_dir}/docker-compose.local.yml
  ${deploy_dir}/docker-compose.override.yml
  ${deploy_dir}/data
  ${deploy_dir}/postgres_data
  ${deploy_dir}/redis_data

Useful commands:
  cd ${deploy_dir}
  ${compose_display} -f docker-compose.local.yml -f docker-compose.override.yml ps
  ${compose_display} -f docker-compose.local.yml -f docker-compose.override.yml logs -f sub2api
  ${compose_display} -f docker-compose.local.yml -f docker-compose.override.yml up -d --no-deps --force-recreate sub2api

EOF
