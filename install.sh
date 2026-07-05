#!/usr/bin/env bash
# Kroombox Backup Agent (KBA) — One-command Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/andiahmadnurmadani/kba/main/install.sh | bash
# ─────────────────────────────────────────────

set -uo pipefail
SELF="$(cd "$(dirname "$0")" 2>/dev/null && pwd || pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
CHECK="\xE2\x9C\x94"; CROSS="\xE2\x9C\x98"; BULLET="\xE2\x80\xA2"

info()  { echo -e "  ${CYAN}${BULLET}${NC} $1"; }
ok()    { echo -e "  ${GREEN}${CHECK}${NC} $1"; }
warn()  { echo -e "  ${YELLOW}${BULLET}${NC} $1"; }
fail()  { echo -e "  ${RED}${CROSS}${NC} $1"; }

# ── Detect OS ──
detect_os() {
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
}

detect_os

# ── Check what's installed ──
check_status() {
	local cmd=$1
	if command -v "$cmd" &>/dev/null; then
		echo -e "${GREEN}${CHECK}${NC}"
	else
		echo -e "${RED}${CROSS}${NC}"
	fi
}

get_version() {
	local cmd=$1
	command -v "$cmd" &>/dev/null && $cmd --version 2>/dev/null | head -1 || echo "-"
}

echo ""
echo -e "  ${BOLD}╔══════════════════════════════════════════════╗${NC}"
echo -e "  ${BOLD}║   Kroombox Backup Agent (KBA) Installer     ║${NC}"
echo -e "  ${BOLD}╚══════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${CYAN}System:${NC}  ${OS} (${ARCH})"
echo ""

# ── Pre-install Summary ──
echo -e "  ${BOLD}Installation Plan:${NC}"
echo ""

# Go
GO_STATUS=$(check_status go)
echo -e "  ${BULLET} Go language       ${GO_STATUS}    Build & compile KBA"
if command -v go &>/dev/null; then
	GO_VER=$(go version | awk '{print $3}')
	echo -e "                       ${YELLOW}${GO_VER} already installed${NC}"
fi

# Git
GIT_STATUS=$(check_status git)
echo -e "  ${BULLET} Git               ${GIT_STATUS}    Clone & version control"
if command -v git &>/dev/null; then
	GIT_VER=$(git --version | awk '{print $3}')
	echo -e "                       ${YELLOW}${GIT_VER} already installed${NC}"
fi

echo ""

# ── Backup Modules ──
echo -e "  ${BOLD}Backup Modules:${NC}"
echo ""

for tool in mysqldump pg_dumpall mongodump pm2 docker nginx; do
	status=$(check_status "$tool")
	case "$tool" in
		mysqldump)  DESC="MySQL/MariaDB databases" ;;
		pg_dumpall) DESC="PostgreSQL databases" ;;
		mongodump)  DESC="MongoDB databases" ;;
		pm2)        DESC="PM2 process manager" ;;
		docker)     DESC="Docker containers" ;;
		nginx)      DESC="Nginx configuration" ;;
	esac
	echo -e "  ${BULLET} $tool\t${status}\t${DESC}"
done

echo ""
echo -e "  ${BOLD}Target:${NC}"
echo -e "  ${BULLET} Binary:  ${CYAN}/usr/local/bin/kba${NC}"
echo -e "  ${BULLET} Config:  ${CYAN}/etc/kroombox/backup-agent/config.yaml${NC}"
echo -e "  ${BULLET} Backups: ${CYAN}/var/backups/kroombox/${NC}"
echo -e "  ${BULLET} DB:      ${CYAN}~/.kroombox/backup-agent.db${NC} (SQLite)"
echo ""

# ── Confirmation ──
if [ "${KBA_AUTO:-}" != "1" ]; then
	echo -e "  ${YELLOW}Press Enter to start installation, or Ctrl+C to cancel...${NC}"
	read -r </dev/tty || read -r || true
fi

echo ""
echo -e "  ${CYAN}━━━━━━━━━━━━━━━━━━━━━ Installing ─${NC}"
echo ""

# ── Step 1: Install missing core deps ──
if ! command -v git &>/dev/null; then
	info "Installing Git..."
	case "$OS" in
		linux)
			sudo apt-get update -qq && sudo apt-get install -y -qq git 2>/dev/null ||
			sudo yum install -y git 2>/dev/null ||
			sudo pacman -S --noconfirm git 2>/dev/null
			;;
		darwin)
			if command -v brew &>/dev/null; then brew install git; fi
			;;
	esac
	command -v git &>/dev/null && ok "Git installed"
fi

if ! command -v curl &>/dev/null; then
	info "Installing curl..."
	case "$OS" in
		linux) sudo apt-get install -y -qq curl 2>/dev/null || sudo yum install -y curl 2>/dev/null ;;
		darwin) brew install curl 2>/dev/null || true ;;
	esac
fi

# ── Step 2: Install Go ──
if ! command -v go &>/dev/null; then
	info "Installing Go 1.23.4..."
	GO_TAR="go1.23.4.${OS}-${ARCH}.tar.gz"
	curl -fsSL "https://go.dev/dl/${GO_TAR}" -o /tmp/go.tar.gz || {
		fail "Download failed — check internet connection"
		exit 1
	}
	sudo rm -rf /usr/local/go
	sudo tar -C /usr/local -xzf /tmp/go.tar.gz || {
		fail "Extract failed"
		exit 1
	}
	rm -f /tmp/go.tar.gz
	export PATH="$PATH:/usr/local/go/bin"
	echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc 2>/dev/null || true
	[ -f ~/.zshrc ] && echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.zshrc 2>/dev/null || true
	hash -r 2>/dev/null || true
	ok "Go $(/usr/local/go/bin/go version | awk '{print $3}')"
else
	ok "Go $(go version | awk '{print $3}')"
fi

# Ensure go is in PATH
export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"

# ── Step 3: Clone / Build ──
BUILD_DIR=""
if [ -f "$SELF/go.mod" ]; then
	BUILD_DIR="$SELF"
	info "Building from local source..."
else
	info "Cloning KBA repository..."
	BUILD_DIR="/tmp/kba-build"
	rm -rf "$BUILD_DIR"
	git clone --depth 1 https://github.com/andiahmadnurmadani/kba.git "$BUILD_DIR" 2>/dev/null || {
		fail "Clone failed"
		exit 1
	}
	ok "Repository cloned"
fi

cd "$BUILD_DIR"

info "Building KBA binary..."
# Offline build first, fallback to online
if GONOSUMCHECK='*' GONOSUMDB='*' GOFLAGS="-mod=mod" GOPROXY=off \
	go build -ldflags="-s -w" -o kba . 2>/dev/null; then
	:
elif go build -ldflags="-s -w" -o kba . 2>/dev/null; then
	:
else
	fail "Build failed! Try: cd $BUILD_DIR && GOPROXY=https://proxy.golang.org,direct go build"
	exit 1
fi
ok "KBA built ($(ls -lh kba | awk '{print $5}'))"

# ── Step 4: Install Binary ──
info "Installing binary..."
if [ -w /usr/local/bin ]; then
	cp kba /usr/local/bin/kba
	chmod +x /usr/local/bin/kba
else
	sudo cp kba /usr/local/bin/kba
	sudo chmod +x /usr/local/bin/kba
fi
ok "Installed: /usr/local/bin/kba"

# ── Step 5: Setup Config ──
info "Setting up configuration..."
sudo mkdir -p /etc/kroombox/backup-agent
if [ ! -f /etc/kroombox/backup-agent/config.yaml ]; then
	sudo tee /etc/kroombox/backup-agent/config.yaml > /dev/null << 'CONF'
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
	ok "Config created"
else
	ok "Config exists"
fi

# ── Step 6: Setup Backup Directory ──
info "Setting up backup directory..."
if sudo mkdir -p /var/backups/kroombox 2>/dev/null; then
	sudo chown "$(whoami)" /var/backups/kroombox 2>/dev/null || true
	ok "Backup dir: /var/backups/kroombox"
else
	mkdir -p "$HOME/backups/kroombox"
	sudo sed -i 's|destination:.*|destination: '"$HOME/backups/kroombox"'|' /etc/kroombox/backup-agent/config.yaml
	ok "Backup dir: $HOME/backups/kroombox (fallback)"
fi

# ── Step 7: Init SQLite Database ──
info "Initializing database..."
mkdir -p "$HOME/.kroombox"
/usr/local/bin/kba version &>/dev/null || true
ok "SQLite database ready"

# ── Step 8: Setup Service ──
case "$OS" in
	linux)
		if command -v systemctl &>/dev/null; then
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
		else
			warn "systemctl not found — use 'kba schedule' for cron setup"
		fi
		;;
	darwin)
		info "Creating launchd plist..."
		mkdir -p "$HOME/Library/LaunchAgents"
		cat > "$HOME/Library/LaunchAgents/com.kroombox.backup-agent.plist" << 'PLIST'
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
</dict>
</plist>
PLIST
		ok "LaunchAgent created"
		warn "Run: launchctl load ~/Library/LaunchAgents/com.kroombox.backup-agent.plist"
		;;
esac

# ── Step 9: Post-install Summary ──
echo ""
echo -e "  ${GREEN}${BOLD}╔══════════════════════════════════════════╗${NC}"
echo -e "  ${GREEN}${BOLD}║   Installation Complete!                  ║${NC}"
echo -e "  ${GREEN}${BOLD}╚══════════════════════════════════════════╝${NC}"
echo ""

# Detect & show services
echo -e "  ${BOLD}Detected Services:${NC}"
/usr/local/bin/kba detect 2>/dev/null | while IFS= read -r line; do
	echo "    $line"
done

echo ""
echo -e "  ${BOLD}Quick Start:${NC}"
echo ""
echo -e "    ${CYAN}kba backup${NC}              Run backup now"
echo -e "    ${CYAN}kba status${NC}              Show status"
echo -e "    ${CYAN}kba schedule${NC}            Setup automatic backup"
echo -e "    ${CYAN}kba schedule --daily 09:00${NC}  Daily at 09:00 WIB"
echo -e "    ${CYAN}kba schedule --cleanup 7${NC}    Auto-delete backups >7 days"
echo -e "    ${CYAN}kba logs${NC}                View backup history"
echo ""
echo -e "  ${BOLD}Need credentials?${NC}"
echo ""
echo -e "    ${YELLOW}MySQL:${NC}"
echo -e '      echo -e "[client]\\nuser=root\\npassword=YOUR_PASS" > ~/.my.cnf'
echo -e "      chmod 600 ~/.my.cnf"
echo ""
echo -e "    ${YELLOW}PostgreSQL:${NC}"
echo -e "      Set pg_hba.conf to 'local all all trust' or use .pgpass"
echo ""
echo -e "  ${BOLD}Schedule${NC}"
echo -e "    ${CYAN}kba schedule${NC}  (interactive) or ${CYAN}kba schedule --daily 10:00${NC}"
echo ""
echo -e "  ${BOLD}GitHub:${NC} ${CYAN}https://github.com/andiahmadnurmadani/kba${NC}"
echo ""
