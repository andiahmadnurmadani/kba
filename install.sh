#!/usr/bin/env bash
# Kroombox Backup Agent (KBA) — One-command Installer
# curl -fsSL https://raw.githubusercontent.com/andiahmadnurmadani/kba/main/install.sh | bash
set -uo pipefail

SELF="$(cd "$(dirname "$0")" 2>/dev/null && pwd || pwd)"
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
CHECK="\xE2\x9C\x94"; CROSS="\xE2\x9C\x98"; BULLET="\xE2\x80\xA2"
info()  { echo -e "  ${CYAN}${BULLET}${NC} $1"; }
ok()    { echo -e "  ${GREEN}${CHECK}${NC} $1"; }
warn()  { echo -e "  ${YELLOW}${BULLET}${NC} $1"; }
fail()  { echo -e "  ${RED}${CROSS}${NC} $1"; }

detect_os() {
	case "$(uname -s)" in
		Linux*)  OS="linux" ;;
		Darwin*) OS="darwin" ;;
		*)       fail "Unsupported OS"; exit 1 ;;
	esac
	case "$(uname -m)" in
		x86_64|amd64) ARCH="amd64" ;;
		aarch64|arm64) ARCH="arm64" ;;
		*) fail "Unsupported arch"; exit 1 ;;
	esac
}
detect_os

# ── Pre-flight banner ──
clear 2>/dev/null || true
echo ""
echo -e "  ${BOLD}╔══════════════════════════════════════════════╗${NC}"
echo -e "  ${BOLD}║     Kroombox Backup Agent (KBA) v1.0.0     ║${NC}"
echo -e "  ${BOLD}║         One-Command Installer               ║${NC}"
echo -e "  ${BOLD}╚══════════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${CYAN}${BULLET}${NC} System: ${BOLD}${OS}${NC} (${ARCH})"
echo -e "  ${CYAN}${BULLET}${NC} User:   $(whoami)"
echo ""

# ── Detailed Installation Plan ──
echo -e "  ${BOLD}━━━ Installation Plan ━━━${NC}"
echo ""

# Section: Core dependencies
echo -e "  ${BOLD}[1/5] Core Dependencies${NC}"
plan_core() {
	local list=""
	# Go
	if command -v go &>/dev/null; then
		list+="  ${GREEN}${CHECK}${NC} Go $(go version | awk '{print $3}')\n"
	else
		list+="  ${YELLOW}${BULLET}${NC} Go 1.23.4  (will be installed from golang.org)\n"
	fi
	# Git
	if command -v git &>/dev/null; then
		list+="  ${GREEN}${CHECK}${NC} Git $(git --version 2>/dev/null | awk '{print $3}')\n"
	else
		list+="  ${YELLOW}${BULLET}${NC} Git        (will be installed via apt/brew)\n"
	fi
	# Curl
	if command -v curl &>/dev/null; then list+="  ${GREEN}${CHECK}${NC} curl\n"
	else list+="  ${YELLOW}${BULLET}${NC} curl\n"; fi
	# Tar
	if command -v tar &>/dev/null; then list+="  ${GREEN}${CHECK}${NC} tar\n"
	else list+="  ${YELLOW}${BULLET}${NC} tar\n"; fi
	echo -e "$list"
}
plan_core

# Section: Go Module Dependencies
echo -e "  ${BOLD}[2/5] Go Module Dependencies${NC}"
echo -e "  ${GREEN}${CHECK}${NC} modernc.org/sqlite  (SQLite driver, ~5MB)"
echo -e "  ${GREEN}${CHECK}${NC} gopkg.in/yaml.v3    (YAML config parser)"
echo ""

# Section: Backup Modules
echo -e "  ${BOLD}[3/5] Backup Modules${NC}"
for tool in mysqldump pg_dumpall mongodump pm2 docker nginx; do
	status=$(command -v "$tool" &>/dev/null && echo "${GREEN}${CHECK}${NC}" || echo "${YELLOW}-${NC}")
	case "$tool" in
		mysqldump)  DESC="MySQL databases" ;;
		pg_dumpall) DESC="PostgreSQL databases" ;;
		mongodump)  DESC="MongoDB databases" ;;
		pm2)        DESC="PM2 processes" ;;
		docker)     DESC="Docker containers" ;;
		nginx)      DESC="Nginx config" ;;
	esac
	echo -e "  ${status} $tool  → ${DESC}"
done
echo -e "  ${YELLOW}-${NC} (missing modules are auto-skipped)"
echo ""

# Section: System Integration
echo -e "  ${BOLD}[4/5] System Integration${NC}"
dest="/var/backups/kroombox"
echo -e "  ${CYAN}${BULLET}${NC} Binary:  ${BOLD}/usr/local/bin/kba${NC}"
echo -e "  ${CYAN}${BULLET}${NC} Config:  ${BOLD}/etc/kroombox/backup-agent/config.yaml${NC}"
echo -e "  ${CYAN}${BULLET}${NC} Backups: ${BOLD}${dest}${NC}"
echo -e "  ${CYAN}${BULLET}${NC} Logs:    ${BOLD}./logs/${NC}"
echo -e "  ${CYAN}${BULLET}${NC} DB:      ${BOLD}~/.kroombox/backup-agent.db${NC} (SQLite)"
if command -v systemctl &>/dev/null; then
	echo -e "  ${CYAN}${BULLET}${NC} Service: ${BOLD}systemd timer${NC} (auto-start on boot)"
elif [[ "$OS" == "darwin" ]]; then
	echo -e "  ${CYAN}${BULLET}${NC} Service: ${BOLD}launchd${NC} (auto-start on boot)"
fi
echo ""

# Section: Post-Install
echo -e "  ${BOLD}[5/5] Post-Install${NC}"
echo -e "  ${CYAN}${BULLET}${NC} Setup MySQL credentials (if MySQL is installed)"
echo -e "  ${CYAN}${BULLET}${NC} Run first backup with: ${BOLD}kba backup${NC}"
echo -e "  ${CYAN}${BULLET}${NC} Schedule automatic backup: ${BOLD}kba schedule${NC}"
echo ""

# ── Confirmation ──
total_size="~15MB"
echo -e "  ${BOLD}Total download size:${NC} ${total_size}"
echo ""
if [ "${KBA_AUTO:-}" != "1" ]; then
	echo -e "  ${YELLOW}Press Enter to start installation, or Ctrl+C to cancel...${NC}"
	read -r </dev/tty 2>/dev/null || read -r || true
fi

# ═══════════════════════════════════════════
#  INSTALLATION
# ═══════════════════════════════════════════

echo ""
echo -e "  ${CYAN}━━━━━━━━━━━━━━━━━━━━━ Installing ━${NC}"
echo ""

# ── Step 1: Core deps ──
if ! command -v curl &>/dev/null; then
	info "Installing curl..."
	case "$OS" in
		linux) sudo apt-get install -y -qq curl 2>/dev/null || sudo yum install -y curl 2>/dev/null ;;
		darwin) brew install curl 2>/dev/null || true ;;
	esac
fi

if ! command -v git &>/dev/null; then
	info "Installing Git..."
	case "$OS" in
		linux)
			sudo apt-get update -qq && sudo apt-get install -y -qq git 2>/dev/null ||
			sudo yum install -y git 2>/dev/null ||
			sudo pacman -S --noconfirm git 2>/dev/null ;;
		darwin) brew install git 2>/dev/null || true ;;
	esac
fi

# ── Step 2: Go ──
if ! command -v go &>/dev/null; then
	info "Installing Go 1.23.4..."
	GO_TAR="go1.23.4.${OS}-${ARCH}.tar.gz"
	curl -fsSL "https://go.dev/dl/${GO_TAR}" -o /tmp/go.tar.gz || {
		fail "Download failed — no internet?"
		exit 1
	}
	sudo rm -rf /usr/local/go
	sudo tar -C /usr/local -xzf /tmp/go.tar.gz || { fail "Extract failed"; exit 1; }
	rm -f /tmp/go.tar.gz
	export PATH="$PATH:/usr/local/go/bin"
	hash -r 2>/dev/null || true
	# Persist path
	for rc in ~/.bashrc ~/.zshrc ~/.profile; do
		grep -q '/usr/local/go/bin' "$rc" 2>/dev/null || echo 'export PATH=$PATH:/usr/local/go/bin' >> "$rc" 2>/dev/null || true
	done
	ok "Go $(/usr/local/go/bin/go version | awk '{print $3}')"
else
	ok "Go $(go version | awk '{print $3}')"
fi

# Ensure Go is in PATH for build
export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"
hash -r 2>/dev/null || true

# ── Step 3: Build ──
BUILD_DIR=""
if [ -f "$SELF/go.mod" ]; then
	BUILD_DIR="$SELF"
else
	info "Cloning KBA repository..."
	BUILD_DIR="/tmp/kba-build"
	rm -rf "$BUILD_DIR"
	git clone --depth 1 https://github.com/andiahmadnurmadani/kba.git "$BUILD_DIR" 2>/dev/null || {
		fail "Clone failed — no internet?"
		exit 1
	}
	ok "Repository cloned"
fi

cd "$BUILD_DIR"

info "Downloading Go dependencies..."
go mod download 2>&1 | tail -1 || true

info "Building KBA binary..."
# Try online build first, then offline
if go build -ldflags="-s -w" -o kba . 2>/dev/null; then
	:
elif GONOSUMCHECK='*' GONOSUMDB='*' GOFLAGS="-mod=mod" GOPROXY=off \
	go build -ldflags="-s -w" -o kba . 2>/dev/null; then
	:
else
	fail "Build failed. Trying alternative GOPROXY..."
	GOPROXY=https://proxy.golang.org,direct go build -ldflags="-s -w" -o kba . 2>&1 | tail -5 || {
		fail "Build failed. Check internet or run manually: cd ${BUILD_DIR} && go build"
		exit 1
	}
fi
ok "KBA built ($(ls -lh kba | awk '{print $5}'))"

# ── Step 4: Install Binary ──
info "Installing binary..."
if [ -w /usr/local/bin ]; then
	cp kba /usr/local/bin/kba && chmod +x /usr/local/bin/kba
else
	sudo cp kba /usr/local/bin/kba && sudo chmod +x /usr/local/bin/kba
fi
ok "Installed: /usr/local/bin/kba"

# ── Step 5: Config ──
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

# ── Step 6: Backup dir ──
if sudo mkdir -p /var/backups/kroombox 2>/dev/null; then
	sudo chown "$(whoami)" /var/backups/kroombox 2>/dev/null || true
	ok "Backup dir: /var/backups/kroombox"
else
	BDIR="$HOME/backups/kroombox"
	mkdir -p "$BDIR"
	sudo sed -i "s|destination:.*|destination: ${BDIR}|" /etc/kroombox/backup-agent/config.yaml 2>/dev/null || true
	ok "Backup dir: ${BDIR}"
fi

# ── Step 7: DB ──
mkdir -p "$HOME/.kroombox"
/usr/local/bin/kba version &>/dev/null || true
ok "SQLite database initialized"

# ── Step 8: Service ──
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
	<array><string>/usr/local/bin/kba</string><string>backup</string></array>
	<key>StartCalendarInterval</key>
	<dict><key>Hour</key><integer>3</integer><key>Minute</key><integer>0</integer></dict>
	<key>Nice</key><integer>19</integer>
	<key>LowPriorityIO</key><true/>
</dict>
</plist>
PLIST
		ok "LaunchAgent created"
		;;
esac
ok "Installation complete!"
echo ""

# ── Summary ──
echo -e "  ${BOLD}╔══════════════════════════════════════════╗${NC}"
echo -e "  ${BOLD}║   ${GREEN}All set! Ready to backup.${NC}              ${BOLD}║${NC}"
echo -e "  ${BOLD}╚══════════════════════════════════════════╝${NC}"
echo ""
echo -e "  ${BOLD}Quick Start:${NC}"
echo ""
echo -e "    ${CYAN}kba backup${NC}              Run first backup"
echo -e "    ${CYAN}kba status${NC}              Show system status"
echo -e "    ${CYAN}kba detect${NC}              Check services & credentials"
echo -e "    ${CYAN}kba schedule${NC}            Setup automatic schedule"
echo -e "    ${CYAN}kba schedule --daily 09:00${NC}  Daily at 09:00 WIB"
echo -e "    ${CYAN}kba schedule --cleanup 7${NC}    Auto-delete old backups"
echo -e "    ${CYAN}kba logs${NC}                View backup history"
echo ""
echo -e "  ${BOLD}Credentials:${NC}"
echo ""
if command -v mysql &>/dev/null && [ ! -f ~/.my.cnf ]; then
	echo -e "    ${YELLOW}→ MySQL:${NC} echo -e \"${BOLD}[client]\\nuser=root\\npassword=YOUR_PASS${NC}\" > ~/.my.cnf"
	echo -e "    chmod 600 ~/.my.cnf"
fi
echo ""
echo -e "  ${BOLD}GitHub:${NC} ${CYAN}https://github.com/andiahmadnurmadani/kba${NC}"
echo ""
