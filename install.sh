#!/usr/bin/env bash
# Kroombox Backup Agent (KBA) — Installer v1.0.0
# Linux (x86_64, aarch64) + macOS (Intel, Apple Silicon)
# ─────────────────────────────────────────────

set -uo pipefail
# Note: no 'set -e' - we handle errors manually

BIN_NAME="kba"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/kroombox/backup-agent"
BACKUP_DIR="/var/backups/kroombox"
KBA_VERSION="1.0.0"
SELF="$(cd "$(dirname "$0")" && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

info()  { echo -e "${CYAN}[..]${NC} $1"; }
ok()    { echo -e "${GREEN}[✔]${NC} $1"; }
warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
fail()  { echo -e "${RED}[✘]${NC} $1"; }

# ── Pre-flight ──
preflight() {
	echo -e "\n${BOLD}Kroombox Backup Agent v${KBA_VERSION} — Installer${NC}"
	echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

	# OS detection
	case "$(uname -s)" in
		Linux*)  OS="linux" ;;
		Darwin*) OS="darwin" ;;
		*)       fail "Unsupported OS: $(uname -s)"; exit 1 ;;
	esac
	case "$(uname -m)" in
		x86_64|amd64) ARCH="amd64" ;;
		aarch64|arm64) ARCH="arm64" ;;
		*) fail "Unsupported arch: $(uname -m)"; exit 1 ;;
	esac
	info "Detected: ${OS} (${ARCH})"

	# Required tools
	info "Checking prerequisites..."
	local missing=0
	for cmd in curl tar git; do
		if ! command -v "$cmd" &>/dev/null; then
			fail "Missing: $cmd"
			missing=1
		fi
	done

	# Go is optional (we'll install if missing)
	if ! command -v go &>/dev/null; then
		warn "Go not found — will install"
	fi

	# Sudo check
	if ! command -v sudo &>/dev/null; then
		warn "sudo not found — some steps may fail"
	fi

	return $missing
}

# ── Install Go ──
install_go() {
	if command -v go &>/dev/null && [ "$(go version | head -c14)" = "go version go1" ]; then
		ok "Go $(go version | awk '{print $3}')"
		return 0
	fi

	warn "Installing Go 1.23.4..."
	local tarball="go1.23.4.${OS}-${ARCH}.tar.gz"
	curl -fsSL "https://go.dev/dl/${tarball}" -o /tmp/go.tar.gz || {
		fail "Download failed (no internet?)"
		return 1
	}
	sudo rm -rf /usr/local/go
	sudo tar -C /usr/local -xzf /tmp/go.tar.gz || {
		fail "Extract failed"
		return 1
	}
	rm -f /tmp/go.tar.gz
	export PATH="$PATH:/usr/local/go/bin"
	echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc 2>/dev/null || true
	if [ -f ~/.zshrc ]; then
		echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.zshrc 2>/dev/null || true
	fi
	ok "Go $(/usr/local/go/bin/go version | awk '{print $3}')"
}

# ── Build ──
build_kba() {
	local src="$1"
	info "Building KBA..."
	cd "$src"

	# Try online build first, fallback to offline
	if go build -ldflags="-s -w" -o "$BIN_NAME" . 2>/dev/null; then
		ok "Build complete ($(ls -lh $BIN_NAME | awk '{print $5}'))"
		return 0
	fi

	# Offline mode (GOPROXY=off)
	warn "Online build failed — trying offline mode..."
	GONOSUMCHECK='*' GONOSUMDB='*' GOFLAGS="-mod=mod" GOPROXY=off 		go build -ldflags="-s -w" -o "$BIN_NAME" . 2>/dev/null && {
		ok "Build complete (offline) ($(ls -lh $BIN_NAME | awk '{print $5}'))"
		return 0
	}

	fail "Build failed!"
	return 1
}

# ── Install Binary ──
install_binary() {
	info "Installing to ${INSTALL_DIR}/${BIN_NAME}..."
	if [ -w "$INSTALL_DIR" ]; then
		cp "$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
		chmod +x "$INSTALL_DIR/$BIN_NAME"
	else
		sudo cp "$BIN_NAME" "$INSTALL_DIR/$BIN_NAME" 2>/dev/null || {
			warn "Need sudo to install to ${INSTALL_DIR}"
			sudo cp "$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
		}
		sudo chmod +x "$INSTALL_DIR/$BIN_NAME"
	fi
	ok "Installed: $(which $BIN_NAME 2>/dev/null || echo "$INSTALL_DIR/$BIN_NAME")"
}

# ── Config ──
setup_config() {
	if [ -f "$CONFIG_DIR/config.yaml" ]; then
		ok "Config exists: $CONFIG_DIR/config.yaml"
		return 0
	fi

	info "Creating config..."
	sudo mkdir -p "$CONFIG_DIR"
	sudo tee "$CONFIG_DIR/config.yaml" > /dev/null << 'CONF'
server:
  name: ""
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
CONF
	ok "Config: $CONFIG_DIR/config.yaml"
}

# ── Backup Directory ──
setup_backup_dir() {
	sudo mkdir -p "$BACKUP_DIR" 2>/dev/null || {
		warn "Cannot create $BACKUP_DIR"
		local home_backup="$HOME/backups/kroombox"
		mkdir -p "$home_backup"
		BACKUP_DIR="$home_backup"
		warn "Using: $BACKUP_DIR"
		# Update config
		sudo sed -i "s|destination:.*|destination: $BACKUP_DIR|" "$CONFIG_DIR/config.yaml" 2>/dev/null || true
		return 0
	}
	sudo chown "$(whoami)" "$BACKUP_DIR" 2>/dev/null || true
	ok "Backup dir: $BACKUP_DIR"
}

# ── Service ──
setup_service() {
	case "$OS" in
		linux)
			if ! command -v systemctl &>/dev/null; then
				warn "systemctl not found — skipping service (use cron instead)"
				return 0
			fi
			info "Installing systemd service..."
			sudo tee /etc/systemd/system/kroombox-backup.service > /dev/null << 'SVC'
[Unit]
Description=Kroombox Backup Agent
After=network.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/kba backup
User=root
Nice=19
IOSchedulingClass=idle
SVC
			sudo systemctl daemon-reload 2>/dev/null || true
			ok "Service: kroombox-backup.service"
			;;
		darwin)
			info "Installing launchd plist..."
			mkdir -p ~/Library/LaunchAgents
			cat > ~/Library/LaunchAgents/com.kroombox.backup-agent.plist << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.kroombox.backup-agent</string>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/local/bin/kba</string>
		<string>backup</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>3</integer>
		<key>Minute</key>
		<integer>0</integer>
	</dict>
	<key>Nice</key>
	<integer>19</integer>
	<key>LowPriorityIO</key>
	<true/>
	<key>StandardOutPath</key>
	<string>/tmp/kroombox-backup.log</string>
	<key>StandardErrorPath</key>
	<string>/tmp/kroombox-backup.err</string>
</dict>
</plist>
PLIST
			ok "LaunchAgent: ~/Library/LaunchAgents/com.kroombox.backup-agent.plist"
			warn "Load with: launchctl load ~/Library/LaunchAgents/com.kroombox.backup-agent.plist"
			;;
	esac
}

# ── Post-install hints ──
post_install_hints() {
	echo ""
	echo -e "${YELLOW}━━ Post-Install Hints ──${NC}"

	# MySQL
	if command -v mysql &>/dev/null; then
		if [ ! -f ~/.my.cnf ]; then
			echo -e "  ${YELLOW}MySQL detected — create ~/.my.cnf for passwordless backup:${NC}"
			echo "    [client]"
			echo "    user=root"
			echo "    password=YOUR_PASSWORD"
		else
			ok "MySQL: ~/.my.cnf found"
		fi
	fi

	# PostgreSQL
	if command -v psql &>/dev/null; then
		echo -e "  ${YELLOW}PostgreSQL detected — set pg_hba.conf or ~/.pgpass for backup${NC}"
	fi

	# MongoDB
	if command -v mongod &>/dev/null || command -v mongosh &>/dev/null; then
		# Check if running
		if ! pgrep -x mongod >/dev/null 2>&1; then
			if [ "$OS" = "linux" ]; then
				echo -e "  ${YELLOW}MongoDB: sudo systemctl start mongod${NC}"
			else
				echo -e "  ${YELLOW}MongoDB: brew services start mongod-community${NC}"
			fi
		else
			ok "MongoDB is running"
		fi
	fi

	# PM2
	if command -v pm2 &>/dev/null; then
		ok "PM2: $(pm2 --version 2>/dev/null || echo 'installed')"
	else
		echo -e "  ${YELLOW}PM2: npm install -g pm2${NC}"
	fi

	# ── Summary ──
	echo ""
	echo -e "${BOLD}✔ KBA installed successfully!${NC}"
	echo ""
	echo -e "  ${CYAN}kba backup${NC}       — Run backup"
	echo -e "  ${CYAN}kba status${NC}       — Show status"
	echo -e "  ${CYAN}kba detect${NC}       — Detect services"
	echo -e "  ${CYAN}kba schedule${NC}     — Schedule backups"
	echo -e "  ${CYAN}kba schedule --daily 09:00${NC}  — Quick daily WIB"
	echo -e "  ${CYAN}kba schedule --cleanup 7${NC}    — Auto-cleanup"
	echo ""
	echo -e "  Config:  ${BOLD}$CONFIG_DIR/config.yaml${NC}"
	echo -e "  Backups: ${BOLD}$BACKUP_DIR${NC}"
	echo -e "  Logs:    ${BOLD}./logs/${NC}"
	echo -e "  DB:      ${BOLD}~/.kroombox/backup-agent.db${NC}"
	echo ""
}

# ── Main ──
main() {
	preflight || exit 1
	install_go || exit 1
	build_kba "$SELF" || exit 1
	install_binary
	setup_config
	setup_backup_dir
	setup_service
	post_install_hints
}

main "$@"
