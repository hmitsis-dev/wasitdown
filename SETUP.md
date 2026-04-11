# wasitdown.dev — Production Setup Guide

Full walkthrough for deploying the stack:
- **VPS** — runs PostgreSQL + GitHub Actions self-hosted runner
- **GitHub Actions** — scrapes data, generates static site
- **Cloudflare Pages** — serves the static site

---

## Local Development

Use `docker-compose.local.yml` — has postgres, scraper, generator, and nginx all wired up.

```bash
cp .env.example .env
# Edit .env — set POSTGRES_PASSWORD

# Start everything
docker compose -f docker-compose.local.yml up -d postgres

# Run scraper once (fetches incidents)
docker compose -f docker-compose.local.yml run --rm scraper

# Generate the site
docker compose -f docker-compose.local.yml run --rm generator

# Serve at http://localhost:8080
docker compose -f docker-compose.local.yml up -d web
```

`docker-compose.yml` (no suffix) is the production file — postgres only, localhost-bound. Don't use it locally.

---

## 1. VPS

Any provider works (Hetzner, DigitalOcean, Vultr, etc.). Minimum specs: 1 vCPU, 1GB RAM, 20GB disk, Ubuntu 22.04 LTS.

### Initial server setup

```bash
# SSH into your VPS as root
ssh root@YOUR_VPS_IP

# Create a non-root user
adduser wasitdown
usermod -aG sudo wasitdown

# Copy your SSH key to the new user
rsync --archive --chown=wasitdown:wasitdown ~/.ssh /home/wasitdown

# Switch to new user for everything below
su - wasitdown
```

### Firewall

```bash
sudo ufw allow OpenSSH
sudo ufw enable
# Only SSH is open. Postgres stays on localhost — never exposed.
```

---

## 2. Docker

Install Docker and Docker Compose on the VPS.

```bash
# Install Docker
curl -fsSL https://get.docker.com | sudo sh

# Add your user to the docker group (no sudo needed)
sudo usermod -aG docker wasitdown

# Apply group change without logging out
newgrp docker

# Verify
docker --version
docker compose version
```

---

## 3. PostgreSQL (Docker on VPS)

Clone the repo and start only the postgres service.

```bash
# Clone the repo
git clone https://github.com/YOUR_USERNAME/wasitdown.git
cd wasitdown

# Create .env from example
cp .env.example .env
# Edit .env — set a strong POSTGRES_PASSWORD
nano .env

# Start only postgres, bound to localhost
docker compose up -d postgres
```

The `docker-compose.yml` binds postgres to `127.0.0.1:5432` only — it is never reachable from the internet.

Verify it's running:

```bash
docker compose ps
docker compose logs postgres
```

---

## 4. Go

The self-hosted runner needs Go installed to run the scraper and generator.

```bash
# Download Go 1.22
wget https://go.dev/dl/go1.22.4.linux-amd64.tar.gz

# Install
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc and ~/.profile for runner service)
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.bashrc

# Verify
go version
```

---

## 5. GitHub Actions Self-Hosted Runner

This runs on your VPS. It picks up jobs from your GitHub repo and executes them locally — giving the scraper direct localhost access to postgres.

### Register the runner

1. Go to your GitHub repo
2. **Settings → Actions → Runners → New self-hosted runner**
3. Select **Linux** and **x64**
4. Follow the commands GitHub shows you. They look like this:

```bash
# Create a directory for the runner
mkdir -p ~/actions-runner && cd ~/actions-runner

# Download (use the exact URL GitHub gives you — it includes a version token)
curl -o actions-runner-linux-x64.tar.gz -L https://github.com/actions/runner/releases/download/vX.X.X/actions-runner-linux-x64-X.X.X.tar.gz

# Extract
tar xzf ./actions-runner-linux-x64.tar.gz

# Configure (use the exact token GitHub gives you)
./config.sh --url https://github.com/YOUR_USERNAME/wasitdown --token YOUR_TOKEN
```

When prompted:
- Runner group: press Enter (default)
- Runner name: `wasitdown-vps` (or anything)
- Labels: press Enter (default — adds `self-hosted`)
- Work folder: press Enter (default `_work`)

### Run as a service (survives reboots)

```bash
# Install as systemd service
sudo ./svc.sh install

# Start it
sudo ./svc.sh start

# Verify it's running
sudo ./svc.sh status
```

The runner will now appear as **Online** in GitHub → Settings → Actions → Runners.

---

## 6. GitHub Secrets

Go to your repo → **Settings → Secrets and variables → Actions → New repository secret**.

Add these three secrets:

| Secret | Value |
|---|---|
| `DATABASE_URL` | `postgres://wasitdown:YOUR_POSTGRES_PASSWORD@localhost:5432/wasitdown?sslmode=disable` |
| `CLOUDFLARE_API_TOKEN` | (see Cloudflare section below) |
| `CLOUDFLARE_ACCOUNT_ID` | (see Cloudflare section below) |

`DATABASE_URL` uses `localhost` because the runner runs on the same machine as postgres.

---

## 7. Cloudflare Pages

### Create the Pages project

1. Log in to [dash.cloudflare.com](https://dash.cloudflare.com)
2. Go to **Workers & Pages → Create → Pages → Connect to Git**
3. Connect your GitHub account and select the `wasitdown` repo
4. Set build settings:
   - **Build command**: leave blank (GitHub Actions handles building)
   - **Build output directory**: `dist`
5. Click **Save and Deploy** — this first deploy will be empty, that's fine

### Get your Account ID

In the Cloudflare dashboard URL: `dash.cloudflare.com/ACCOUNT_ID/...` — copy that ID.

Or go to **Workers & Pages → Overview** — it's shown in the right sidebar.

### Create an API Token

1. Go to **My Profile → API Tokens → Create Token**
2. Use the **Edit Cloudflare Workers** template, or create a custom token with:
   - Permission: **Cloudflare Pages — Edit**
   - Account: your account
3. Copy the token — you only see it once

Add both values as GitHub secrets (`CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`).

---

## 8. First Run

Trigger the workflows manually to verify everything works:

```bash
# On the VPS — verify postgres is up
docker compose ps

# On the VPS — verify Go is available to the runner
# (runner runs as a service user, check PATH is set)
sudo systemctl status actions.runner.*
```

In GitHub → **Actions**:
1. Run **"Run Scraper"** manually → should complete green
2. Run **"Generate & Deploy Static Site"** manually → should deploy to Cloudflare Pages

Check your Cloudflare Pages URL to confirm the site is live.

---

## Maintenance

### View postgres logs
```bash
cd ~/wasitdown
docker compose logs -f postgres
```

### Restart postgres
```bash
docker compose restart postgres
```

### Update the runner
```bash
cd ~/actions-runner
sudo ./svc.sh stop
# Download and extract new version, re-run config.sh with a new token from GitHub
sudo ./svc.sh start
```

### Pull repo changes on VPS (for migrations)
```bash
cd ~/wasitdown
git pull
docker compose restart postgres  # migrations run on scraper startup, not needed here
# The scraper runs via GitHub Actions — it auto-applies migrations on next run
```

### Backups
```bash
# Dump the database
docker exec wasitdown-postgres-1 pg_dump -U wasitdown wasitdown > backup_$(date +%Y%m%d).sql
```
