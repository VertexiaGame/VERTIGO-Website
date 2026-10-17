set -euo pipefail

DEPLOY_PATH="${DEPLOY_PATH:?DEPLOY_PATH is required}"
SERVICE_NAME="${SERVICE_NAME:?SERVICE_NAME is required}"
HEALTH_URL="${HEALTH_URL:?HEALTH_URL is required}"
RELEASE_SHA="${RELEASE_SHA:-}"

if [ ! -f "$DEPLOY_PATH/release-new.tar.gz" ]; then
  echo "::error::Release archive is missing from $DEPLOY_PATH"
  exit 1
fi

has_prev=false
if [ -f "$DEPLOY_PATH/vertexia-frontend" ]; then
  cp "$DEPLOY_PATH/vertexia-frontend" "$DEPLOY_PATH/vertexia-frontend.prev"
  has_prev=true
fi

rm -f "$DEPLOY_PATH/vertexia-frontend"

if ! tar -xzf "$DEPLOY_PATH/release-new.tar.gz" -C "$DEPLOY_PATH"; then
  echo "::error::Failed to extract release into $DEPLOY_PATH"
  if [ "$has_prev" = "true" ]; then
    cp "$DEPLOY_PATH/vertexia-frontend.prev" "$DEPLOY_PATH/vertexia-frontend"
    chmod +x "$DEPLOY_PATH/vertexia-frontend"
  fi
  exit 1
fi

chmod +x "$DEPLOY_PATH/vertexia-frontend"

supervisorctl_bin() {
  if command -v supervisorctl >/dev/null 2>&1; then
    command -v supervisorctl
  elif [ -x /www/server/panel/pyenv/bin/supervisorctl ]; then
    echo /www/server/panel/pyenv/bin/supervisorctl
  fi
}

restart_service() {
  local supctl
  supctl="$(supervisorctl_bin)"
  if [ -n "$supctl" ] && "$supctl" restart "$SERVICE_NAME"; then
    return 0
  elif [ "$(id -u)" = "0" ]; then
    systemctl restart "$SERVICE_NAME"
  else
    sudo -n systemctl restart "$SERVICE_NAME"
  fi
}

if ! restart_service; then
  echo "::error::Failed to restart '$SERVICE_NAME' for commit $RELEASE_SHA"
  echo "::error::try 'sudo systemctl restart $SERVICE_NAME'"
  if [ "$has_prev" = "true" ]; then
    rm -f "$DEPLOY_PATH/vertexia-frontend"
    cp "$DEPLOY_PATH/vertexia-frontend.prev" "$DEPLOY_PATH/vertexia-frontend"
    chmod +x "$DEPLOY_PATH/vertexia-frontend"
    restart_service || true
  fi
  exit 1
fi

ok=false
for i in $(seq 1 30); do
  if curl -fsS -o /dev/null --max-time 2 "$HEALTH_URL"; then
    ok=true
    break
  fi
  sleep 1
done

if [ "$ok" != "true" ]; then
  echo "::error::Health check on $HEALTH_URL failed for commit $RELEASE_SHA"
  echo "::error::Rolling back to the previous release"
  if [ "$has_prev" = "true" ]; then
    rm -f "$DEPLOY_PATH/vertexia-frontend"
    cp "$DEPLOY_PATH/vertexia-frontend.prev" "$DEPLOY_PATH/vertexia-frontend"
    chmod +x "$DEPLOY_PATH/vertexia-frontend"
    restart_service || true
  else
    echo "::error::no previous release available to restore"
  fi
  exit 1
fi

rm -f "$DEPLOY_PATH/release-new.tar.gz"
echo "Deploy of commit $RELEASE_SHA is healthy at $DEPLOY_PATH"
