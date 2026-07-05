
<p align="center">
  <img src="https://img.shields.io/badge/version-1.0.0-blue?style=flat-square" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Linux-FCC624?style=flat-square&logo=linux&logoColor=black" alt="Linux">
  <img src="https://img.shields.io/badge/macOS-000000?style=flat-square&logo=apple" alt="macOS">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License">
</p>

<h1 align="center">🛡️ Kroombox Backup Agent (KBA)</h1>

<p align="center">
  <b>Lightweight, auto-discovery backup agent for Linux & macOS.</b><br>
  Backup MySQL, PostgreSQL, MongoDB, PM2, Nginx, SSL, Docker, Cron & Git —<br>
  with SQLite tracking, schedule, and auto-cleanup.
</p>

<p align="center">
  
</p>

---

## ✨ Features

| Feature | Description |
|---------|-------------|
| 🔍 **Auto-Detection** | Detects installed services automatically — no config needed |
| 🗄️ **Multiple Databases** | MySQL, PostgreSQL, MongoDB (full dump per service) |
| ⚙️ **Config Backup** | Nginx, PM2, SSL, Docker, Git config files, Crontab |
| 🗓️ **Scheduling** | systemd (Linux) / launchd (macOS) / Cron with timezone support |
| 🧹 **Auto-Cleanup** | SQLite-backed retention policy — deletes old backups |
| 📊 **Status & Logs** | `kba status` shows system info, services, last backup, schedule |
| 🖥️ **Cross-Platform** | Works on Ubuntu, Debian, macOS (Intel & Apple Silicon) |
| 🚀 **One-Command Install** | Installs Go + builds + configures in one go |

---

## 🚀 Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/andiahmadnurmadani/kba/main/install.sh | bash
```

> Shows installation plan → asks confirmation → installs everything automatically.

### Non-interactive (for scripts):
```bash
KBA_AUTO=1 curl -fsSL https://raw.githubusercontent.com/andiahmadnurmadani/kba/main/install.sh | bash
```

---

## 📋 Requirements

| Tool | Required? | For |
|------|-----------|-----|
| Go 1.23+ | ✅ Auto-installed | Build KBA |
| Git | ✅ Auto-installed | Clone repo |
| `mysqldump` | ⚠️ Optional | MySQL backup |
| `mongodump` | ⚠️ Optional | MongoDB backup |
| `pg_dumpall` | ⚠️ Optional | PostgreSQL backup |
| `pm2` | ⚠️ Optional | PM2 process backup |
| `docker` | ⚠️ Optional | Docker config backup |

> Missing tools = skipped automatically. KBA still runs fine without them.

---

## 🎯 Usage

### Backup Now

```bash
kba backup
```



```
Backup from minibox
─────  layers: [mysql, mongodb, pm2, nginx, ssl, git, cron, docker]

 ✔ mysql        [==============================] 100% Backup done  4s
 ✔ mongodb      [==============================] 100% Backup done  3s
 ✔ pm2          [==============================] 100% Backup done  2s
 ✔ nginx        [==============================] 100% Backup done  2s
 ✔ save         [==============================] 100% Backup done  1s

 status: ✔ backed up (192.4MB)
```

### Check Status

```bash
kba status
```

```
  ┌────────────────────────────────────────────────────┐
  │ System Info                                        │
  ├────────────────────────────────────────────────────┤
  │ Hostname: minibox                                  │
  │ OS:       Linux  5.15.0-181-generic                │
  └────────────────────────────────────────────────────┘

  Service                Version
  ─────────────────────  ───────────────────────────
  ✓ mysql       mysqldump Ver 8.4.10 ...
  ✓ mongodb     mongodump version: 100.17.0...
  ✓ pm2         7.0.3
  ✓ nginx

  ┌────────────────────────────────────────────────────┐
  │ Last Backup (2026-07-05)                           │
  ├────────────────────────────────────────────────────┤
  │ Date:     2026-07-05                               │
  │ Size:     192.4MB                                  │
  │ Backed up: nginx, pm2, git, mongodb, mysql         │
  └────────────────────────────────────────────────────┘

  Current backup schedule:
    Last run:  Sun 2026-07-05 05:17:11 UTC
    Status:    active (systemd timer)
    Schedule:  *-*-* 10:00:00 Asia/Jakarta
```

### Schedule Automatic Backup

```bash
# Interactive wizard
kba schedule

# Quick setup — daily at 09:00 WIB
kba schedule --daily 09:00

# With custom timezone
kba schedule --daily 10:00 --tz Asia/Makassar

# View current schedule
kba schedule --list

# Remove schedule
kba schedule --remove
```

### Cleanup Old Backups

```bash
# Delete backups older than 7 days
kba schedule --cleanup 7

# Delete backups older than 30 days
kba schedule --cleanup 30
```

```
Cleaning backups older than 7 days...
  ✓ Removed 2026-06-28 (147.2MB)
  ✓ Removed 2026-06-27 (43.5MB)

  Cleaned up 2 backups (190.7MB freed)
```

### View Logs

```bash
kba logs
```

```
=== Recent Backup Logs ===

│ backup_20260705.log [Jul 05 05:00]:
  INFO: 2026/07/05 05:00:10 [mysql] done (148.6MB)
  INFO: 2026/07/05 05:00:11 [mongodb] done (43.5MB)
  INFO: 2026/07/05 05:00:13 [pm2] done (76.9KB)
  INFO: 2026/07/05 05:00:16 Backup completed!

=== Recent Backups ===
  2026-07-05/
    { "hostname": "minibox", "backup_date": "2026-07-05", "size": "192.4MB" }
```

### Restore

```bash
# Restore from a backup directory
kba restore /var/backups/kroombox/2026-07-05

# Restore specific services only
kba restore /var/backups/kroombox/2026-07-05 mysql nginx
```

---

## 🔐 Credentials Setup

### MySQL

Create `~/.my.cnf` for passwordless backup:

```ini
[client]
user=root
password=your_password
```

```bash
chmod 600 ~/.my.cnf
```

### PostgreSQL

Set trust authentication in `pg_hba.conf`, or use `~/.pgpass`:

```
localhost:5432:*:postgres:your_password
```

### MongoDB

MongoDB local (no auth) works out of the box. For auth:

```bash
mongodump --username user --password pass --authenticationDatabase admin
```

---

## 📁 Backup Structure

```
/var/backups/kroombox/
├── 2026-07-05/
│   ├── manifest.json      ← backup metadata (JSON)
│   ├── mysql/
│   │   └── all.sql        ← mysqldump --all-databases
│   ├── mongodb/
│   │   └── innovillage/   ← mongodump per DB
│   ├── pm2/
│   │   ├── dump.pm2       ← PM2 process list
│   │   └── pm2_list.json
│   ├── nginx/
│   │   ├── nginx.conf     ← Nginx config
│   │   └── sites-enabled/
│   ├── ssl/
│   │   └── letsencrypt/   ← SSL certs (if exists)
│   ├── git/
│   │   └── project.env    ← .env, package.json, Dockerfile
│   ├── cron/
│   │   └── crontab.txt    ← Crontab entries
│   └── docker/
│       ├── compose/       ← Docker Compose files
│       ├── images.txt
│       └── volumes.txt
└── ...
```

---

## 🗄️ Database (SQLite)

KBA uses SQLite to track backups — no separate DB server needed.

```bash
~/.kroombox/backup-agent.db
```

**Tables:**
| Table | Description |
|-------|-------------|
| `backups` | Records every backup run (path, date, size, services) |
| `config` | Stores settings (destination path, schedule config) |

---

## ⚙️ Configuration

```yaml
server:
  name: ""              # auto (uses hostname)

backup:
  mysql: auto           # auto / disabled
  postgres: auto
  mongodb: auto
  nginx: auto
  pm2: auto
  docker: auto
  ssl: auto
  git: auto
  cron: auto

storage:
  type: local
  destination: /var/backups/kroombox
```

Config locations (auto-discovered in order):
1. `./config.yaml` (current directory)
2. `/etc/kroombox/backup-agent/config.yaml` (system-wide)
3. `~/.kroombox/backup-agent.yaml` (user)

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────┐
│                    CLI (main.go)                     │
│   backup  │  restore  │  status  │  schedule  │ logs │
└───────────┴───────────┴──────────┴─────────────┴─────┘
        │           │           │              │
        ▼           ▼           ▼              ▼
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐
│ Backup   │ │ Restore  │ │ Detect   │ │ Scheduler    │
│ Engine   │ │ Engine   │ │ Engine   │ │ systemd/cron │
└────┬─────┘ └──────────┘ └────┬─────┘ └──────┬───────┘
     │                          │              │
     ▼                          ▼              ▼
┌─────────────────────────────────────────────────────┐
│  Modules                                              │
│  MySQL │ PostgreSQL │ MongoDB │ PM2 │ Nginx │ Docker  │
│  SSL │ Git │ Cron                                     │
└───────────────────────────────────────────────────────┘
     │                          │
     ▼                          ▼
┌──────────┐            ┌──────────────┐
│ Storage  │            │ SQLite DB    │
│ Provider │            │ (~/.kroombox)│
└──────────┘            └──────────────┘
```

---

## 🖥️ Supported Platforms

| Platform | Architectures | Service Manager |
|----------|--------------|-----------------|
| Ubuntu 22.04+ | amd64, arm64 | systemd / cron |
| Debian 12+ | amd64, arm64 | systemd / cron |
| macOS (Intel) | amd64 | launchd / cron |
| macOS (Apple Silicon) | arm64 | launchd / cron |

---

## 📦 All Commands

| Command | Description |
|---------|-------------|
| `kba backup` | Run backup now |
| `kba status` | Show system, services, schedule |
| `kba detect` | Detect services & check credentials |
| `kba schedule` | Interactive schedule wizard |
| `kba schedule --daily 09:00` | Quick daily schedule (WIB) |
| `kba schedule --list` | Show current schedule |
| `kba schedule --remove` | Remove schedule |
| `kba schedule --cleanup 7` | Delete backups >7 days |
| `kba logs` | View backup logs |
| `kba restore <dir>` | Restore from backup |
| `kba version` | Show version |

---

## 🔧 Development

```bash
git clone https://github.com/andiahmadnurmadani/kba.git
cd kba
go build -ldflags="-s -w" -o kba .
sudo cp kba /usr/local/bin/kba
```

### Project Structure

```
kba/
├── main.go              ← CLI entrypoint
├── go.mod               ← Go module
├── install.sh           ← One-command installer
├── config.yaml.example  ← Example config
├── backup/              ← Backup orchestrator
├── restore/             ← Restore engine
├── scheduler/           ← Schedule wizard + systemd/cron
├── detect/              ← Auto-detection engine
├── modules/             ← Backup modules (MySQL, PG, Mongo, etc.)
├── storage/             ← Local storage provider
├── manifest/            ← Backup manifest (JSON)
├── progress/            ← Progress bar animation
├── db/                  ← SQLite database
├── tui/                 ← Terminal UI (menus, tables, boxes)
└── logs/                ← Logging
```

---

## 📄 License

MIT © Kroombox

<p align="center">
  <a href="https://github.com/andiahmadnurmadani/kba">
    <img src="https://img.shields.io/github/stars/andiahmadnurmadani/kba?style=social" alt="Stars">
  </a>
  <a href="https://github.com/andiahmadnurmadani/kba/issues">
    <img src="https://img.shields.io/github/issues/andiahmadnurmadani/kba?style=social" alt="Issues">
  </a>
</p>
