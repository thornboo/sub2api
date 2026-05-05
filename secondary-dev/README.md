# Secondary Development Records

This directory records local secondary-development changes on top of the upstream project.

## Files

- `CHANGELOG.md`: chronological summary of visible product or behavior changes.
- `PATCHES.md`: implementation notes for patches, affected modules, and verification.
- `MERGELOG.md`: upstream synchronization records for secondary-development branches.

## Recording Rules

- Add an entry whenever a patch changes user-facing behavior, styles, modules, routes, or build/runtime behavior.
- Record the date, scope, affected files or modules, and verification commands.
- Keep entries factual and concise. Avoid storing secrets, access tokens, private credentials, or environment-specific values.

## Docker Deployment for `dev-sd`

This secondary-development branch must be deployed from the forked source code, not from the upstream public image.

Do not deploy this branch with:

```bash
docker pull weishaw/sub2api:latest
```

That image is the upstream project image and does not contain the `dev-sd` secondary-development changes.

### 1. Clone the secondary-development branch

On the server:

```bash
git clone -b dev-sd https://github.com/thornboo/sub2api.git
cd sub2api
```

For an existing checkout:

```bash
cd sub2api
git fetch origin
git switch dev-sd
git pull --ff-only origin dev-sd
```

### 2. Build the local Docker image

Build from the repository root, where the project `Dockerfile` exists:

```bash
docker build -t sub2api:dev-sd .
```

The repository `Dockerfile` builds the frontend first, embeds the frontend output into the Go backend, and creates a runtime image containing `/app/sub2api`.

### 3. Prepare deployment files

Use the deployment files from this same `dev-sd` checkout:

```bash
cd deploy
cp .env.example .env
mkdir -p data postgres_data redis_data
```

Edit `deploy/.env` and set at least:

```env
POSTGRES_PASSWORD=change_this_to_a_strong_password
JWT_SECRET=change_this_to_a_fixed_32_byte_hex_secret
TOTP_ENCRYPTION_KEY=change_this_to_a_fixed_32_byte_hex_secret
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=change_this_to_a_strong_admin_password
SERVER_PORT=8080
TZ=Asia/Shanghai
```

Generate fixed secrets on the server with:

```bash
openssl rand -hex 32
```

Keep `JWT_SECRET` and `TOTP_ENCRYPTION_KEY` stable across restarts. Changing them later can invalidate login sessions or existing TOTP setup.

### 4. Override the upstream image name

`deploy/docker-compose.local.yml` uses `weishaw/sub2api:latest` by default. Create a local override file in `deploy/` so Compose runs the locally built secondary-development image:

```bash
cat > docker-compose.override.yml <<'EOF'
services:
  sub2api:
    image: sub2api:dev-sd
EOF
```

This override file is intentionally local deployment state. `deploy/docker-compose.override.yml` is ignored by this repository. Do not put secrets in it.

### 5. Start the stack

From `deploy/`:

```bash
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d
```

Check service status and logs:

```bash
docker compose -f docker-compose.local.yml -f docker-compose.override.yml ps
docker compose -f docker-compose.local.yml -f docker-compose.override.yml logs -f sub2api
```

The app listens on `SERVER_PORT` from `.env`, defaulting to:

```text
http://SERVER_IP:8080
```

### 6. Update an existing deployment

From the repository checkout on the server:

```bash
git switch dev-sd
git pull --ff-only origin dev-sd
docker build -t sub2api:dev-sd .
cd deploy
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d --no-deps --force-recreate sub2api
```

PostgreSQL, Redis, and `/app/data` are persisted by local directories under `deploy/`:

```text
deploy/data
deploy/postgres_data
deploy/redis_data
```

Do not remove those directories during normal upgrades.

### 7. Backup before upgrades

At minimum, stop the stack or ensure a quiet maintenance window, then archive deployment data:

```bash
cd deploy
tar czf sub2api-deploy-backup-$(date +%F-%H%M).tar.gz .env data postgres_data redis_data
```

For production database backups, prefer a PostgreSQL logical dump in addition to the directory archive:

```bash
docker compose -f docker-compose.local.yml -f docker-compose.override.yml exec -T postgres \
  sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' > sub2api-$(date +%F-%H%M).sql
```

### 8. Data-retention defaults in this branch

The `dev-sd` branch defaults automatic deletion to disabled for commercial retention:

```env
DASHBOARD_AGGREGATION_RETENTION_AUTO_CLEANUP_ENABLED=false
OPS_CLEANUP_AUTO_CLEANUP_ENABLED=false
```

These values are already present in `deploy/.env.example`. Keep them disabled unless automatic retention cleanup is intentionally re-enabled.

### Operational cautions

- Do not run `docker compose down -v` unless intentionally deleting Docker volumes.
- Do not delete `deploy/data`, `deploy/postgres_data`, or `deploy/redis_data` unless intentionally destroying the deployment.
- Do not use the upstream one-click deployment script for this branch, because it downloads deployment files from upstream `Wei-Shaw/sub2api/main`.
- Rebuild the local image after each `git pull`; Compose does not rebuild `sub2api:dev-sd` automatically.
