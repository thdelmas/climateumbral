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

- Update: `cd /opt/climateumbral && git pull && docker compose -f deploy/docker-compose.prod.yml up -d --build`
- Logs: `docker compose -f deploy/docker-compose.prod.yml logs -f --tail 100`
- The API runs `-trust-proxy` behind Caddy so per-IP limits see real
  client IPs. Memory caps: api 256 MB, caddy 128 MB.
- Backup = copy `data/ledger.json` (0600, holds bearer tokens — treat
  as secret).
