# linkgear Docker 使用指南

风格对齐 [agentgear](https://github.com/Sunshow/agentgear)：多阶段 Go 构建、Alpine 运行、非 root（UID 1000）。

## 构建

```bash
docker build -t sunshow/linkgear:latest .
```

## 使用 Compose（Postgres）

```bash
export LINKGEAR_PUBLIC_URL=https://linkgear.example.com
docker compose up -d
```

- App: http://localhost:8080
- Docs: http://localhost:8080/docs/bot-integration.md
- Health: http://localhost:8080/healthz

## 仅运行镜像（SQLite）

```bash
docker run -d --name linkgear \
  -p 8080:8080 \
  -e LINKGEAR_PUBLIC_URL=http://localhost:8080 \
  -e LINKGEAR_DSN=sqlite:///app/data/linkgear.db \
  -v "$(pwd)/data:/app/data" \
  sunshow/linkgear:latest
```

## 环境变量

| 变量 | 说明 | 默认（镜像内） |
|------|------|----------------|
| `LINKGEAR_DSN` | `sqlite://…` 或 `postgres://…` | `sqlite:///app/data/linkgear.db` |
| `LINKGEAR_HTTP_ADDR` | 监听 | `:8080` |
| `LINKGEAR_PUBLIC_URL` | 对外 URL（加入链接 / 文档链接） | `http://localhost:8080` |
| `LINKGEAR_DELIVERY_WORKERS` | 出站并发 | `8` |
| `LINKGEAR_DELIVERY_POLL` | pending 扫描间隔 | `2s` |

第一期**不使用 Redis**（DB outbox + 进程内 worker）。

## 注意

- 宿主机 `./data` 需可被 UID 1000 写入：`sudo chown -R 1000:1000 ./data`
- `LINKGEAR_PUBLIC_URL` 必须是 bot 能访问到的地址，否则邀请文案里的 join URL / docs 链接会错

## Jenkins

根目录 `Jenkinsfile` 推送到 Docker Hub：`sunshow/linkgear:latest` 与 `:<git-short>-<YYYYMMDD>`。建议任务名 `pipeline-build-linkgear`。

## 生产部署（docker-compose.prod.yml）

与开发用 `docker-compose.yaml`（本地 build）不同，生产文件**默认拉取** Docker Hub 镜像 `sunshow/linkgear:latest`，不本地构建。风格对齐 agentgear（`restart` + `healthcheck`），并附带 Postgres 与命名卷。

```bash
cp .env.example .env
# 编辑 .env：改 POSTGRES_PASSWORD、LINKGEAR_DSN（密码保持一致）、LINKGEAR_PUBLIC_URL
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml ps
curl -fsS http://127.0.0.1:8080/healthz
```

Compose 会自动加载同目录下的 `.env` 做变量插值，无需再写 `--env-file`。

- App 默认映射：`${LINKGEAR_HOST_PORT:-8080}:8080`
- 数据卷：`linkgear_pg_data`（Postgres）
- 凭据与对外 URL：见 `.env.example`（复制为 `.env`，勿提交）
- 镜像标签：`LINKGEAR_IMAGE_TAG`（默认 `latest`；亦可设 Jenkins 打的 `<short-sha>-<YYYYMMDD>`）

**外部 Postgres：** 在 `.env` 写好 `LINKGEAR_DSN=postgres://…`，然后只起应用：

```bash
docker compose -f docker-compose.prod.yml up -d --no-deps linkgear
```

第一期仍**不使用 Redis**。
