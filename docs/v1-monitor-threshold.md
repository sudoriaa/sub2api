# V1 渠道监控慢响应阈值

在 **系统设置 → 功能开关 → 渠道监控 → 慢响应变黄阈值（秒）** 调整。

- 默认 **6 秒**，支持 **1–90 秒的整数**。
- 只作用于 V1 的成功探活请求，统计完整响应耗时，不是首 Token 延迟。
- 耗时达到阈值时显示黄格；请求错误、空响应和答案校验失败仍显示红格。
- 保存后下一次定时检测或手动检测生效，不需要重启。
- 历史状态不重新计算；配额阈值、请求超时及 V2/V3 规则不变。
- 老数据库没有该设置时仍使用 6 秒；旧客户端省略该字段不会覆盖已保存值。

管理员设置 API 字段为 `channel_monitor_degraded_threshold_seconds`。读取和保存使用原有
`GET /api/v1/admin/settings`、`PUT /api/v1/admin/settings`，无需数据库迁移。

## Fork 镜像构建

该定制分支基于 v2.20.1。标准 `Dockerfile` 仍可完整构建；`deploy/Dockerfile.fork`
可以把带嵌入前端的新二进制叠加到当前固定版本运行环境上，保留其依赖、许可证及启动脚本。
在 Linux 或具备对应工具链的构建环境运行：

```bash
COMMIT=$(git rev-parse HEAD)
VERSION=2.20.1-custom
OUT=$(mktemp -d)
(cd frontend && pnpm install --frozen-lockfile && pnpm build)
(cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embed \
  -trimpath -ldflags="-s -w -X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildType=release" \
  -o "$OUT/sub2api" ./cmd/server)
cp deploy/Dockerfile.fork "$OUT/Dockerfile"
docker build --build-arg VERSION="$VERSION" --build-arg COMMIT="$COMMIT" \
  -t "sub2api-custom:$COMMIT" "$OUT"
```

更新服务器前备份 Compose、`.env`、应用数据和数据库。只替换 `sub2api` 服务镜像，
保留原环境变量、挂载、网络和端口。数据库和 Redis 不需要重建。
