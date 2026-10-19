#!/usr/bin/env bash
set -euo pipefail
staging=/opt/data/weknora-memory-build-20261012-VOycn2/list-style
cd /opt/data/WeKnora
test "$(grep -c 'weknora-ui:memory-insights-20261012-r2' docker-compose.override.yml)" = 1
backup_dir=$(mktemp -d /opt/data/WeKnora/backups/list-style-20261012-XXXXXX)
chmod 700 "$backup_dir"
tar -czf "$backup_dir/source.tar.gz" docker-compose.override.yml frontend/src/views/settings/AnswerFeedback.vue frontend/src/views/settings/QAInsights.vue
/bin/cp docker-compose.override.yml "$backup_dir/docker-compose.override.yml"
printf 'BACKUP=%s\n' "$backup_dir"
tar -xzf "$staging/ui.tar.gz" -C "$staging"
/bin/cp docker/Dockerfile.evolution-ui "$staging/Dockerfile"
docker build --network none -t weknora-ui:list-style-20261012 "$staging" > "$staging/build.log" 2>&1
/bin/cp "$staging/src/views/settings/AnswerFeedback.vue" frontend/src/views/settings/AnswerFeedback.vue
/bin/cp "$staging/src/views/settings/QAInsights.vue" frontend/src/views/settings/QAInsights.vue
/bin/cp "$staging/src/utils/tableTooltip.ts" frontend/src/utils/tableTooltip.ts
sed -i 's/weknora-ui:memory-insights-20261012-r2/weknora-ui:list-style-20261012/' docker-compose.override.yml
docker compose up -d --no-deps frontend
for attempt in {1..10}; do
  if curl --noproxy '*' -fsS --max-time 3 http://127.0.0.1/platform/qa-insights > /dev/null; then
    printf 'FRONTEND_READY BACKUP=%s\n' "$backup_dir"
    exit 0
  fi
  sleep 2
done
/bin/cp "$backup_dir/docker-compose.override.yml" docker-compose.override.yml
docker compose up -d --no-deps frontend
printf 'ROLLED_BACK BACKUP=%s\n' "$backup_dir"
exit 1
