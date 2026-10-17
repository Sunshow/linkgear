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
