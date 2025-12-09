# Cloudflare Tunnel Deployment Guide

This document covers the full lifecycle of exposing `api.twentcode.com` through a Cloudflare Tunnel and wiring it to the Go API.

## Prerequisites

1. `cloudflared` installed locally or on the host machine.
   - macOS/Linux: `brew install cloudflare/cloudflare/cloudflared`
   - Linux: download from https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation
2. Logged into Cloudflare: `cloudflared login` opens your browser; select the zone that owns `api.twentcode.com`.

## Tunnel Setup

1. Create a tunnel (once):
   ```bash
   cloudflared tunnel create api-demo
   ```
   - Save the generated `Tunnel ID` and credentials file (`~/.cloudflared/<tunnel-id>.json`).
2. Configure `~/.cloudflared/config.yml` (or `/etc/cloudflared/config.yml` on Linux):

   ```yaml
   tunnel: <tunnel-id>
   credentials-file: ~/.cloudflared/<tunnel-id>.json

   ingress:
     - hostname: api.twentcode.com
       service: http://localhost:8080
     - service: http_status:404
   ```

3. DNS routing in Cloudflare:
   - Create a CNAME for `api.twentcode.com` pointing to `<tunnel-id>.cfargotunnel.com`.
   - Ensure the record is proxied (orange cloud).

## Running the Tunnel and API Together

1. Export production-ready environment variables (see `.env.example` for defaults):
   ```bash
   export BASE_URL=https://api.twentcode.com
   export APP_ENV=production
   export ALLOWED_HOSTS=api.twentcode.com
   export CORS_ORIGINS=https://api.twentcode.com
   export JWT_SECRET=your-strong-secret
   export JWT_REFRESH_SECRET=your-other-secret
   ```
2. Start your Go server (for example with Makefile targets):
   ```bash
   make run
   ```
3. Launch the tunnel in its own terminal or background process:
   ```bash
   ./scripts/run_cloudflare_tunnel.sh
   ```
   You can also register the script with systemd or `launchd` for auto-start.

## Monitoring & Health

- Tail the tunnel logs: `cloudflared tunnel run api-demo --loglevel debug`.
- Use Cloudflare Zero Trust dashboard to inspect active connections.
- Confirm the API is healthy: `curl https://api.twentcode.com/api/v1/health` once the tunnel is live.

## Optional Automation

- The accompanying shell script (`scripts/run_cloudflare_tunnel.sh`) wraps the tunnel command and can be run manually or from init tooling.
- For resilience, run `cloudflared tunnel run api-demo` via systemd with `Restart=on-failure` and ensure it runs after Docker/Go service is ready.
