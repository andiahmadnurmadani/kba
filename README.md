# Kroombox Backup Agent (KBA)

Lightweight daemon untuk backup server otomatis dengan **auto-discovery service**.

## Fitur

- **Auto-detection**: MySQL, PostgreSQL, MongoDB, PM2, Nginx, Docker, SSL, Git, Cron
- **Multi-OS**: Linux (Ubuntu/Debian) + macOS (Intel/ARM)
- **Multi-user**: Satu binary bisa dipakai untuk backup semua user
- **Animasi**: Progress bar ala \`docker pull\`
- **Storage**: Local, NAS, Duplicati, SFTP, S3 (v2)
- **Manifest**: JSON report tiap backup
- **Restore**: Selective restore per service

## Quick Install

### Linux / macOS

```bash
# Clone
git clone https://github.com/kroombox/kroombox-backup-agent.git
cd kroombox-backup-agent

# Install otomatis
chmod +x install.sh
./install.sh
```

Atau tanpa clone (Go wajib terinstall):

```bash
go install github.com/kroombox/kroombox-backup-agent@latest
```

### Manual

```bash
# Build
go build -o kba .

# Install binary
sudo cp kba /usr/local/bin/kba
sudo chmod +x /usr/local/bin/kba

# Config
sudo mkdir -p /etc/kroombox/backup-agent
sudo cp config.yaml /etc/kroombox/backup-agent/config.yaml

# Backup dir
sudo mkdir -p /var/backups/kroombox
sudo chown $USER /var/backups/kroombox

# Systemd (Linux only)
sudo cp kroombox-backup-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now kroombox-backup-agent.timer

# Launchd (macOS only)
cp com.kroombox.backup-agent.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.kroombox.backup-agent.plist
```

## Usage

### CLI

```bash
kba backup          # Run backup (dengan animasi)
kba detect          # Deteksi service
kba status          # Status JSON (buat Panel)
kba restore <dir>   # Restore dari backup
kba version         # Versi
```

### Hasil Backup

```
Pull from minibox
─────  layers: [mysql, mongodb, pm2, nginx, ssl, git, cron, docker]

 ✔ mysql        [==============================] 100% Pull complete 7s
 ✔ mongodb      [==============================] 100% Pull complete 5s
 ✔ pm2          [==============================] 100% Pull complete 5s
 ✔ nginx        [==============================] 100% Pull complete 4s
 ✔ save         [==============================] 100% Pull complete <1s

 digest: sha256:kba-2026-07-05
 status: ✔ downloaded (191.0MB)
```

### Struktur Backup

```
/var/backups/kroombox/
  2026-07-05/
    manifest.json    ← JSON report
    mysql/           ← mysqldump per database
    mongodb/         ← mongodump
    pm2/             ← ~/.pm2 + pm2 list
    nginx/           ← config + version
    git/             ← .env, package.json, Dockerfile
    ssl/             ← letsencrypt
    cron/            ← crontab
    docker/          ← compose files + image/volume list
```

## Config

\`\`\`yaml
server:
  name: ""  # auto (hostname)

backup:
  mysql: auto
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
\`\`\`

Config dicari otomatis dari:
1. \`./config.yaml\` (current dir)
2. \`/etc/kroombox/backup-agent/config.yaml\` (system)
3. \`~/.kroombox/backup-agent.yaml\` (user)

## Requirement

| Tool         | Linux              | macOS              |
|-------------|--------------------|--------------------|
| mysqldump   | \`apt install mysql-client\` | \`brew install mysql-client\` |
| mongodump   | \`apt install mongodb-database-tools\` | via mongosh tools |
| pg_dumpall  | \`apt install postgresql-client\` | \`brew install postgresql\` |
| pm2         | \`npm install -g pm2\` | \`npm install -g pm2\` |

## Multiple User

KBA bisa dipakai oleh banyak user:

```bash
# Setiap user punya config sendiri
~/.kroombox/backup-agent.yaml

# Atau pake config berbeda via flag
kba backup --config /path/to/custom/config.yaml
```

## Integrasi dengan Kroombox Panel

Panel cukup panggil:

\`\`\`bash
kba status
\`\`\`

Output JSON:

\`\`\`json
{
  "status": "healthy",
  "hostname": "minibox",
  "last_backup": "2026-07-05",
  "services": ["mysql", "mongodb", "pm2", "nginx"],
  "versions": {"mysql": "8.4.10", "pm2": "7.0.3"}
}
\`\`\`

## License

MIT © Kroombox
