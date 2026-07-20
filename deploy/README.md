# Deploy — climateumbral.eu on a tiny instance

Near-free hosting per INTENT: one small VPS, one domain, no managed
services. Two manual steps (console + registrar), the rest is
cloud-init.

## 1. Instance (Scaleway console)

- **Stardust1-S** (1 vCPU / 1 GB, cheapest) or **DEV1-S** (2 vCPU /
  2 GB) if Stardust is out of stock. **Ubuntu 24.04**.
- Attach a **routed IPv4** (flexible IP).
- Advanced options → **cloud-init**: paste `deploy/cloud-init.yml`.
- Create. First boot builds the stack (~5 min; the 2G swapfile covers
  the npm build on 1 GB instances).

## 2. Domain (Gandi)

Buy `climateumbral.eu`, then DNS records:

```
@    A     <instance-ip>
www  A     <instance-ip>
```

Caddy fetches Let's Encrypt certificates on first request after DNS
propagates — nothing to configure.

## 3. Verify

```
curl -fsS https://climateumbral.eu/api/health
```

Then open the map, pledge something, and check the act survives a
`docker compose restart` (the ledger lives in `/opt/climateumbral/data`,
outside the containers).

## Operate

- Update: automatic — push or merge to `main` and the box redeploys
  itself within ~2 minutes (`climateumbral-deploy.timer` runs
  `deploy/autodeploy.sh`, which fetches, hard-resets to
  `origin/main` and rebuilds only when it moved). The box is a
  deploy target: don't hand-edit `/opt/climateumbral`, it loses to
  main on the next cycle.
- Deploy now / watch: `systemctl start climateumbral-deploy` ·
  `journalctl -u climateumbral-deploy -f`
- Enable auto-deploy on a box that predates it (once):

  ```
  cd /opt/climateumbral && git pull
  cp deploy/climateumbral-deploy.service deploy/climateumbral-deploy.timer /etc/systemd/system/
  systemctl daemon-reload && systemctl enable --now climateumbral-deploy.timer
  ```

  On the host-nginx variant, first point the script at its compose
  file: `echo COMPOSE=deploy/docker-compose.ionos.yml > /etc/default/climateumbral`.
  If the checkout lives elsewhere than `/opt/climateumbral`, adjust
  `ExecStart` in the service file (and `git config --system --add
  safe.directory <path>` if the repo owner differs from root).
- Moderation: add `CLIMATEUMBRAL_ADMIN_TOKEN=<long random>` to
  `/etc/default/climateumbral` (the auto-deploy unit exports it to
  compose). That token erases any act through the same endpoints
  players use:
  `curl -X DELETE -H "X-ClimateUmbral-Token: $ADMIN" https://climateumbral.eu/api/claims/<pe>/<pn>`
  (same for `/api/joins/<be>/<bn>`). Treat it like the ledger:
  secret. Empty or unset = moderation off. Auto-deploy only rebuilds
  when main moves, so apply an env change immediately with:
  `cd /opt/climateumbral && set -a && . /etc/default/climateumbral && set +a && docker compose -f "${COMPOSE:-deploy/docker-compose.prod.yml}" up -d`
- Logs: `docker compose -f deploy/docker-compose.prod.yml logs -f --tail 100`
- The API runs `-trust-proxy` behind Caddy so per-IP limits see real
  client IPs. Memory caps: api 256 MB, caddy 128 MB.
- Backup = copy `data/ledger.json` (0600, holds bearer tokens — treat
  as secret).
