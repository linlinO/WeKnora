#!/usr/bin/env bash
set -euo pipefail
staging=/opt/data/weknora-replay-build-20261013-z12k3i
cd /opt/data/WeKnora
test -s "$staging/build-out/WeKnora"
test -s "$staging/ui.tar.gz"
test "$(grep -c 'weknora-app:memory-insights-20261012' docker-compose.override.yml)" = 1
test "$(grep -c 'weknora-ui:list-style-20261012' docker-compose.override.yml)" = 1
mkdir -p backups
backup_dir=$(mktemp -d /opt/data/WeKnora/backups/memory-replay-20261013-XXXXXX)
chmod 700 "$backup_dir"
cp docker-compose.override.yml "$backup_dir/docker-compose.override.yml"
tar --exclude='./.git' --exclude='./frontend/node_modules' --exclude='./backups' --exclude='./data' --exclude='./logs' --exclude='./models' --exclude='./WeKnora' --exclude='./frontend/dist' -czf "$backup_dir/source.tar.gz" .
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$backup_dir/database.dump"
test -s "$backup_dir/database.dump"
docker compose exec -T postgres pg_restore -l < "$backup_dir/database.dump" > "$backup_dir/database.contents"
docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT version,dirty FROM schema_migrations; SELECT count(*) FROM knowledges; SELECT count(*) FROM answer_feedback; SELECT count(*) FROM memory_items;"' > "$backup_dir/counts-before.txt"
printf 'BACKUP=%s\n' "$backup_dir"
cd "$staging"
mkdir -p runtime/migrations ui
cp build-out/WeKnora runtime/WeKnora
cp -a migrations/. runtime/migrations/
cp docker/Dockerfile.memory-replay-runtime runtime/Dockerfile
tar -xzf ui.tar.gz -C ui
cp docker/Dockerfile.memory-replay-ui ui/Dockerfile
docker build --network none -t weknora-app:memory-replay-20261013 runtime > runtime-build.log 2>&1
docker build --network none -t weknora-ui:memory-replay-20261013 ui > ui-build.log 2>&1
cd /opt/data/WeKnora
tar -xzf "$staging/source.tar.gz"
sed -i 's/weknora-app:memory-insights-20261012/weknora-app:memory-replay-20261013/g; s/weknora-ui:list-style-20261012/weknora-ui:memory-replay-20261013/g' docker-compose.override.yml
docker compose up -d --no-deps app frontend
for attempt in {1..12}; do
  health=$(docker inspect --format '{{.State.Health.Status}}' WeKnora-app 2>/dev/null || true)
  if test "$health" = healthy; then
    docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT version,dirty FROM schema_migrations; SELECT count(*) FROM knowledges; SELECT count(*) FROM answer_feedback; SELECT count(*) FROM memory_items;"' > "$backup_dir/counts-after.txt"
    printf 'DEPLOYMENT_HEALTHY BACKUP=%s\n' "$backup_dir"
    exit 0
  fi
  sleep 5
done
cp "$backup_dir/docker-compose.override.yml" docker-compose.override.yml
docker compose up -d --no-deps app frontend
printf 'ROLLED_BACK_TO_PREVIOUS_IMAGES BACKUP=%s\n' "$backup_dir"
exit 1
