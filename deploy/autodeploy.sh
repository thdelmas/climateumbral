#!/bin/sh -eu
# Auto-deploy: converge this box on origin/main. Fired by
# climateumbral-deploy.timer every 2 minutes; a run that finds nothing
# new costs one git fetch. The box is a deploy target, not a
# workstation — local drift loses to main (reset --hard).
#
# Per-box overrides live in /etc/default/climateumbral:
#   COMPOSE=deploy/docker-compose.ionos.yml   # host-nginx variant
cd "$(dirname "$0")/.."
COMPOSE="${COMPOSE:-deploy/docker-compose.prod.yml}"
git fetch -q origin main
have=$(git rev-parse HEAD)
want=$(git rev-parse origin/main)
[ "$have" = "$want" ] && exit 0
echo "deploying $(git rev-parse --short HEAD) -> $(git rev-parse --short origin/main)"
git reset --hard origin/main
docker compose -f "$COMPOSE" up -d --build
docker image prune -f >/dev/null
echo "deployed: $(git log -1 --oneline)"
