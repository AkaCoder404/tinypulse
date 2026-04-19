#!/bin/bash

# TinyPulse Deployment Script
# This script sets up TinyPulse as a systemd service on a Linux VPS.
# Run this script with sudo: sudo ./setup_systemctl.sh

set -e

# Configuration
APP_NAME="tinypulse"
INSTALL_DIR="/opt/${APP_NAME}"
BIN_PATH="${INSTALL_DIR}/${APP_NAME}"
DB_PATH="${INSTALL_DIR}/uptime.db"
ENV_PATH="${INSTALL_DIR}/.env"
SERVICE_PATH="/etc/systemd/system/${APP_NAME}.service"

# Ensure the script is run as root
if [ "$EUID" -ne 0 ]; then
  echo "❌ Please run this script as root (sudo ./setup_systemctl.sh)"
  exit 1
fi

# 1. Create installation directory
echo "📁 Creating installation directory at ${INSTALL_DIR}..."
mkdir -p "${INSTALL_DIR}"

# 2. Check for the binary in the current directory, or download it
if [ -f "./${APP_NAME}" ]; then
  echo "📦 Found local binary. Installing to ${BIN_PATH}..."
  cp "./${APP_NAME}" "${BIN_PATH}"
else
  echo "🌐 Local binary not found. Downloading the latest Linux AMD64 release from GitHub..."
  
  # You should replace 'AkaCoder404' with your actual GitHub username before using this in production!
  REPO="AkaCoder404/tinypulse"
  LATEST_URL=$(curl -s https://api.github.com/repos/${REPO}/releases/latest | grep "browser_download_url.*linux-amd64" | cut -d '"' -f 4)
  
  if [ -z "$LATEST_URL" ]; then
    echo "❌ Could not find a linux-amd64 release for ${REPO}."
    echo "Please download the binary manually or update the REPO variable in this script."
    exit 1
  fi
  
  wget -O "${BIN_PATH}" "${LATEST_URL}"
fi

chmod +x "${BIN_PATH}"

# 4. Prompt for secure password
echo ""
echo "🔒 TinyPulse uses HTTP Basic Auth to protect your dashboard."
read -sp "Enter a secure password for the 'admin' user: " ADMIN_PASS
echo ""

# 5. Create the .env file
echo "🔑 Creating environment file at ${ENV_PATH}..."
cat <<EOF > "${ENV_PATH}"
TINYPULSE_ADDR=:8080
TINYPULSE_DB=${DB_PATH}
TINYPULSE_PASSWORD=${ADMIN_PASS}
EOF
chmod 600 "${ENV_PATH}"

# 6. Create the systemd service file
echo "⚙️ Creating systemd service file at ${SERVICE_PATH}..."
cat <<EOF > "${SERVICE_PATH}"
[Unit]
Description=TinyPulse Uptime Monitor
After=network.target

[Service]
Type=simple
EnvironmentFile=${ENV_PATH}
ExecStart=${BIN_PATH} -db ${DB_PATH}
WorkingDirectory=${INSTALL_DIR}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 7. Enable and start the service
echo "🚀 Reloading systemd and starting ${APP_NAME}..."
systemctl daemon-reload
systemctl enable "${APP_NAME}"
systemctl restart "${APP_NAME}"

# 8. Check status
echo ""
echo "✅ TinyPulse has been installed and started!"
echo ""
systemctl status "${APP_NAME}" --no-pager | head -n 10
echo ""
echo "==================================================================="
echo "🔔 IMPORTANT DEPLOYMENT REMINDERS"
echo "==================================================================="
echo "TinyPulse is now running internally on port 8080."
echo ""
echo "To access it from the internet, you must configure your network:"
echo ""
echo "1. INTERNAL FIREWALL (e.g., UFW):"
echo "   If you want to access port 8080 directly, run: sudo ufw allow 8080/tcp"
echo ""
echo "2. CLOUD PROVIDER FIREWALL (AWS, DigitalOcean, etc):"
echo "   Ensure port 8080 is open in your cloud console's Security Groups."
echo ""
echo "3. REVERSE PROXY (Highly Recommended):"
echo "   If you use Caddy, Nginx, or Traefik, DO NOT open port 8080."
echo "   Instead, configure your proxy to point a domain (status.example.com)"
echo "   to 'localhost:8080' to enable secure HTTPS automatically."
echo "==================================================================="
