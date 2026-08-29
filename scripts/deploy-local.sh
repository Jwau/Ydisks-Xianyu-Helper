#!/usr/bin/env bash
# deploy-local.sh 把本地工作区源码同步到服务器，在服务器上构建应用镜像并重启容器。
# 适用于 fork 自有部署：不经过 GitHub Actions、GHCR 或上游镜像。
#
# 一次性准备（服务器上）：
#   1. 部署目录（compose.yml + .env 所在目录）新建 docker-compose.override.yml：
#        services:
#          app:
#            image: xianyu-helper:local
#            pull_policy: never
#            build:
#              context: /opt/xianyu-src
#              dockerfile: Dockerfile.debian13
#   2. 确认 ssh 主机别名可直接登录（~/.ssh/config）。
#
# 日常部署：./scripts/deploy-local.sh <ssh主机别名> [服务器源码目录] [服务器部署目录]
set -euo pipefail

# server 是 SSH 主机别名或 user@host。
SERVER="${1:?用法: deploy-local.sh <ssh主机别名> [服务器源码目录] [服务器部署目录]}"
# remote_src 是服务器上存放源码的目录，构建上下文指向它。
REMOTE_SRC="${2:-/opt/xianyu-src}"
# remote_deploy 是服务器上 compose.yml 所在的部署目录。
REMOTE_DEPLOY="${3:-/opt/xianyu}"

echo "==> 同步源码到 $SERVER:$REMOTE_SRC"
# --delete 保证服务器源码与本地工作区完全一致；排除本地构建产物与开发数据。
rsync -az --delete \
  --exclude .git \
  --exclude node_modules \
  --exclude frontend/node_modules \
  --exclude frontend/coverage \
  --exclude data \
  --exclude dist \
  ./ "$SERVER:$REMOTE_SRC/"

echo "==> 服务器上构建镜像并重启应用容器"
# 构建失败时不会执行重启；up -d 只重建镜像变化的服务，数据卷不受影响。
ssh "$SERVER" "cd '$REMOTE_DEPLOY' && docker compose build app && docker compose up -d"

echo "==> 完成。查看日志：ssh $SERVER \"cd $REMOTE_DEPLOY && docker compose logs -f app\""
echo "    健康检查：ssh $SERVER 'curl -fsS http://127.0.0.1:59188/health'"
