#!/usr/bin/env bash
# =============================================================================
# Sub2API dev-zz — local multi-arch image builder
# =============================================================================
# Builds the sub2api image LOCALLY for one or more target architectures and
# exports each as a standalone, loadable image file. No compilation happens on
# the server: you build here, copy the .tar.gz over (scp / USB / chat / etc.),
# and run `docker load` on the target.
#
# Pipeline per run:
#   1. Build the frontend once (arch-independent static assets, embedded in Go).
#   2. For each target arch: cross-compile the Go binary on this host.
#   3. docker build the runtime image (Dockerfile.dist) for that arch.
#   4. docker save + gzip into dist/sub2api-dev-zz-<arch>.tar.gz
#
# Default targets linux/amd64 and linux/arm64 — the same arches the upstream
# project publishes, covering Linux servers, macOS (Docker Desktop) and
# Windows WSL.
#
# Usage:
#   ./deploy/build-image-dev-zz.sh                 # both amd64 + arm64
#   ./deploy/build-image-dev-zz.sh --arch amd64    # amd64 only
#   ./deploy/build-image-dev-zz.sh --arch arm64    # arm64 only
#   ./deploy/build-image-dev-zz.sh --no-frontend   # reuse existing frontend dist
#   ./deploy/build-image-dev-zz.sh --tag sub2api:my-tag
#
# On the target machine:
#   docker load < sub2api-dev-zz-amd64.tar.gz
#   # then start/replace the container via docker compose
# =============================================================================
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-sub2api:dev-zz}"
ARCHES_DEFAULT=("amd64" "arm64")
ARCHES=()
BUILD_FRONTEND=true

print_info()    { printf '\033[0;34m[INFO]\033[0m %s\n' "$1"; }
print_success() { printf '\033[0;32m[SUCCESS]\033[0m %s\n' "$1"; }
print_warning() { printf '\033[1;33m[WARNING]\033[0m %s\n' "$1"; }
print_error()   { printf '\033[0;31m[ERROR]\033[0m %s\n' "$1" >&2; }

usage() {
  cat <<'EOF'
Usage: ./deploy/build-image-dev-zz.sh [options]

Build the sub2api dev-zz image locally and export one loadable file per arch.

Options:
  --arch <amd64|arm64>   Build only this architecture (repeatable). Default: amd64 + arm64.
  --no-frontend          Skip the frontend build and reuse the existing dist.
  --tag <name:tag>       Image name/tag to build. Default: sub2api:dev-zz
  -h, --help             Show this help.

Environment:
  IMAGE_NAME             Same as --tag. Default: sub2api:dev-zz

Output:
  dist/sub2api-dev-zz-<arch>.tar.gz   (one per built architecture)

Load on target:
  docker load < sub2api-dev-zz-amd64.tar.gz
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --arch)
      shift
      [ "$#" -gt 0 ] || { print_error "--arch requires a value (amd64 or arm64)."; exit 1; }
      case "$1" in
        amd64|arm64) ARCHES+=("$1") ;;
        *) print_error "Unsupported arch: $1 (use amd64 or arm64)."; exit 1 ;;
      esac
      ;;
    --no-frontend) BUILD_FRONTEND=false ;;
    --tag)
      shift
      [ "$#" -gt 0 ] || { print_error "--tag requires a value."; exit 1; }
      IMAGE_NAME="$1"
      ;;
    -h|--help) usage; exit 0 ;;
    *) print_error "Unknown option: $1"; usage; exit 1 ;;
  esac
  shift
done

if [ "${#ARCHES[@]}" -eq 0 ]; then
  ARCHES=("${ARCHES_DEFAULT[@]}")
fi

command_exists() { command -v "$1" >/dev/null 2>&1; }
require_command() {
  command_exists "$1" || { print_error "$1 is required but was not found."; exit 1; }
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
cd "$repo_root"

# Sanity check: this must run from a complete checkout.
if [ ! -f "Dockerfile.dist" ] || [ ! -d "backend" ] || [ ! -d "frontend" ]; then
  print_error "Run this from a complete sub2api checkout (Dockerfile.dist, backend/, frontend/ required)."
  exit 1
fi

require_command go
require_command docker
if [ "$BUILD_FRONTEND" = true ]; then
  require_command pnpm
fi

# Detect whether `docker buildx` is available. buildx is the robust path for
# cross-arch builds (--platform); fall back to the legacy builder otherwise.
USE_BUILDX=false
if docker buildx version >/dev/null 2>&1; then
  USE_BUILDX=true
  print_info "docker buildx detected — using it for cross-arch builds."
else
  print_warning "docker buildx not found — falling back to the legacy builder."
  print_warning "Cross-arch image layers may be unreliable without buildx; arm64 host building amd64 (or vice versa) is best done with buildx."
fi

dist_dir="${repo_root}/dist"
mkdir -p "$dist_dir"

# Resolve version/commit/date the same way the source Dockerfile does, so the
# built binary reports consistent build metadata.
VERSION_VALUE="$(tr -d '\r\n' < backend/cmd/server/VERSION 2>/dev/null || echo "dev")"
COMMIT_VALUE="$(git rev-parse --short HEAD 2>/dev/null || echo "local")"
DATE_VALUE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
print_info "Version=${VERSION_VALUE} Commit=${COMMIT_VALUE} Date=${DATE_VALUE}"

# -----------------------------------------------------------------------------
# 1. Frontend build (once; arch-independent, embedded into the Go binary)
# -----------------------------------------------------------------------------
frontend_dist="${repo_root}/backend/internal/web/dist"
if [ "$BUILD_FRONTEND" = true ]; then
  print_info "Building frontend (pnpm install + build)"
  (
    cd "${repo_root}/frontend"
    pnpm install --frozen-lockfile
    pnpm run build
  )
  print_success "Frontend built into backend/internal/web/dist"
else
  if [ ! -d "$frontend_dist" ] || [ -z "$(ls -A "$frontend_dist" 2>/dev/null)" ]; then
    print_error "--no-frontend was given but ${frontend_dist} is empty. Build the frontend first."
    exit 1
  fi
  print_info "Reusing existing frontend dist (--no-frontend)"
fi

# -----------------------------------------------------------------------------
# 2-4. Per-arch: cross-compile Go binary, docker build, docker save
# -----------------------------------------------------------------------------
built_files=()
for arch in "${ARCHES[@]}"; do
  print_info "==================  ${arch}  =================="

  binary_path="${dist_dir}/sub2api-${arch}"
  print_info "Cross-compiling Go binary for linux/${arch} (CGO disabled, pure Go — host-native speed)"
  (
    cd "${repo_root}/backend"
    CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" go build \
      -tags embed \
      -ldflags="-s -w -X main.Version=${VERSION_VALUE} -X main.Commit=${COMMIT_VALUE} -X main.Date=${DATE_VALUE} -X main.BuildType=release" \
      -trimpath \
      -o "${binary_path}" \
      ./cmd/server
  )
  print_success "Built binary: ${binary_path}"

  arch_tag="${IMAGE_NAME}-${arch}"
  print_info "Building image ${arch_tag} for linux/${arch}"
  # --platform makes the runtime base layers (alpine, postgres client) match the
  # target arch; the binary is already cross-compiled for it.
  if [ "$USE_BUILDX" = true ]; then
    # --load imports the result into the local docker image store so it can be saved.
    docker buildx build \
      --platform "linux/${arch}" \
      -f Dockerfile.dist \
      --build-arg "SUB2API_BINARY=dist/sub2api-${arch}" \
      -t "${arch_tag}" \
      --load \
      .
  else
    # Legacy builder (no buildx). MUST use DOCKER_BUILDKIT=0 — setting it to 1
    # would force BuildKit, which itself requires the buildx component.
    DOCKER_BUILDKIT=0 docker build \
      --platform "linux/${arch}" \
      -f Dockerfile.dist \
      --build-arg "SUB2API_BINARY=dist/sub2api-${arch}" \
      -t "${arch_tag}" \
      .
  fi
  print_success "Built image: ${arch_tag}"

  # Tag without the arch suffix so the exported file loads directly as the name
  # docker-compose expects (e.g. sub2api:dev-zz) — no manual `docker tag` needed
  # on the target. Each arch is shipped as a separate file and loaded on a
  # machine of that arch, so the shared name never collides across architectures.
  docker tag "${arch_tag}" "${IMAGE_NAME}"

  out_file="${dist_dir}/sub2api-dev-zz-${arch}.tar.gz"
  print_info "Exporting ${IMAGE_NAME} (linux/${arch}) -> ${out_file}"
  docker save "${IMAGE_NAME}" | gzip > "${out_file}"
  built_files+=("${out_file}")
  print_success "Exported: ${out_file}"
done

echo
print_success "Done. Image file(s) ready in dist/:"
for f in "${built_files[@]}"; do
  size="$(du -h "$f" | cut -f1)"
  printf '  %s  (%s)\n' "$f" "$size"
done

cat <<EOF

Next steps (on the target machine — Linux server / macOS / Windows WSL):
  1. Copy the file matching that machine's architecture over (scp, USB, etc.):
       x86_64 / Intel / WSL                   -> sub2api-dev-zz-amd64.tar.gz
       ARM (Apple Silicon Linux, ARM servers) -> sub2api-dev-zz-arm64.tar.gz
  2. Load it (the image loads directly as '${IMAGE_NAME}', no retag needed):
       docker load < sub2api-dev-zz-amd64.tar.gz
  3. Recreate the container (from the deploy directory):
       docker compose -f docker-compose.local.yml -f docker-compose.override.yml \\
         up -d --no-deps --force-recreate sub2api
EOF
