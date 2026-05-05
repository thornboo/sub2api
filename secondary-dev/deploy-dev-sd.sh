#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-sub2api:dev-sd}"
BRANCH_NAME="${BRANCH_NAME:-dev-sd}"
START_STACK=true
BUILD_IMAGE=true
BUILD_ONLY=false
FORCE_ENV=false
FORCE_OVERRIDE=false

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
Usage: ./secondary-dev/deploy-dev-sd.sh [options]

Build and deploy the dev-sd secondary-development Docker image from source.

Options:
  --no-start        Build image and prepare deploy files, but do not start Docker Compose.
  --build-only      Only build the Docker image. Do not create deploy files or start Compose.
  --no-build        Skip docker build and only prepare/start Compose with the existing image.
  --force-env       Recreate deploy/.env from deploy/.env.example and regenerate secrets.
  --force-override  Recreate deploy/docker-compose.override.yml.
  -h, --help        Show this help.

Environment:
  IMAGE_NAME        Docker image tag to build/use. Default: sub2api:dev-sd
  BRANCH_NAME       Expected Git branch. Default: dev-sd
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

if ! docker compose version >/dev/null 2>&1; then
  print_error "docker compose is required but is not available."
  exit 1
fi

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
  docker compose
  -f docker-compose.local.yml
  -f docker-compose.override.yml
)

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
  docker compose -f docker-compose.local.yml -f docker-compose.override.yml ps
  docker compose -f docker-compose.local.yml -f docker-compose.override.yml logs -f sub2api
  docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d --no-deps --force-recreate sub2api

EOF
