#!/bin/bash
# Initialize /data/ztrade-mcp deployment directory
# Run once on the server before first deployment

set -e

DEPLOY_DIR="/data/ztrade-mcp"

echo "Initializing deployment directory: $DEPLOY_DIR"

mkdir -p "$DEPLOY_DIR/configs"
mkdir -p "$DEPLOY_DIR/strategies"

# Copy docker-compose.yml
cp docker-compose.yml "$DEPLOY_DIR/docker-compose.yml"

# Copy DB init scripts
cp init.sql "$DEPLOY_DIR/init.sql"
cp init-readonly.sh "$DEPLOY_DIR/init-readonly.sh"

# Copy default config if no config exists
if [ ! -f "$DEPLOY_DIR/configs/ztrade.yaml" ]; then
  cp configs/ztrade.yaml "$DEPLOY_DIR/configs/ztrade.yaml"
  echo "Default ztrade.yaml copied to $DEPLOY_DIR/configs/ztrade.yaml"
  echo "Please edit it to set your exchange API keys and auth tokens."
else
  echo "Config already exists at $DEPLOY_DIR/configs/ztrade.yaml, skipping."
fi

echo "Done. Place your config at $DEPLOY_DIR/configs/ztrade.yaml."
echo "Tip: set MYSQL_READONLY_USER / MYSQL_READONLY_PASSWORD in .env before first startup."
echo "  cd $DEPLOY_DIR && docker compose up -d"
