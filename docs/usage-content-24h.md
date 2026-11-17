# 管理员请求正文：最近 100 条纯内存窗口

## 行为与边界

- 管理员 → 使用记录 / 错误请求 → **查看对话**，按需查看原始输入、输出与可读对话。
- 默认每个应用进程仅保留最近 **100 条完成采集的请求**；第 101 条加入时淘汰最早加入的一条。异步计费引用释放后才完成发布，排序按发布顺序，而非请求开始时间。
- 同时限制已完成正文缓冲 **64 MiB**（按实际分配块容量计费）。容量不足时即使不到 100 条也提前淘汰。一个请求的多个计费 ID 共享一份正文，不重复占用条数 / 正文容量。
- 正文**不压缩、不写 PostgreSQL、不写 Redis、不写文件**。重启后清空；不同进程 / 副本不共享正文，详情请求需要路由到捕获该请求的实例。当前没有跨实例读取代理。
- 每个方向仍最多保存 2 MiB；超出只截断诊断内容，转发正文不截断。二进制媒体不保存；JSON 内嵌媒体仍受正文大小上限约束。
- 仍以采集开始时间计算最多 **24 小时** TTL。读取始终检查过期，后台每分钟回收过期条目。窗口不是“保证能查到最近 24 小时的每条请求”。
- 普通用户 / 观察员 / 受限管理密钥不能读取正文，查询始终以可信 API Key ID 隔离。仍保持 no-store、关闭 / 到期清空、文本安全展示，不把正文放入列表 DTO、导出或浏览器持久化缓存。
- 只改变临时对话正文：计费、用量记录、统计、模型请求和原有首字计时口径不变。手动诊断抓取 requestcapture、耗时观测 requesttiming 是独立功能，不受此开关控制。

## 性能实现

1. 请求读取 / 响应写出仅做有界复制；发布时把不可变块的所有权移入 FIFO + 请求 ID 索引，不再等待或执行 gzip / SQL。
2. 只在 Content-Type 实际改变时克隆和识别类型；达到正文限额后仍统计字节，但不为每个分片制造临时分配。
3. WriteString 转发仍走底层字符串写入，采集仅复制实际写出的前缀，不先把完整字符串转成字节数组；短写和错误保持不变。
4. 内部复用最多 64 个 16 KiB 块（1 MiB）；不使用无硬容量界限的 sync.Pool。复用仅暴露本次已覆盖的逻辑字节；仍被读取的块不得回收复用。
5. 进行中的采集另限 512 个会话 / 64 MiB 正文；超限提前跳过或截断采集，不阻塞或重发模型请求。
6. 同时最多 2 个正文读取者，限流覆盖**整个 HTTP JSON 序列化 / 写出**。已淘汰但仍被读取的原文继续计入窗口容量；若这些引用占满容量，只跳过新增正文，不超额扩张或等待。
7. 大于等于 128 Ki 个 JS 字符的详情在 Web Worker 解析；切换、关闭和过期会终止 worker 并拒绝旧结果。Worker 不可用时展示原文，不在主线程同步解析大正文。历史消息先展示 50 条，按需展开；原文格式化仅在原始模式执行。

**64 MiB 是已完成正文缓冲的上限，不是整个进程 RSS 上限。** 在途采集另有 64 MiB，复用池另有 1 MiB，两个读取者另有最多两份输入 + 输出字符串（默认合计不超过 8 MiB 逻辑内容）；HTTP JSON 转义 / 编码、元数据、其他业务内存和 GC 尚有开销。清理代表释放引用，不承诺操作系统立即归还物理内存或安全擦除。

## 配置

重启应用生效；部署时必须把环境变量传入实际运行容器 / 进程。

```yaml
gateway:
  usage_content:
    enabled: true
    max_records: 100
    max_memory_mib: 64
    max_body_bytes: 2097152
```

- GATEWAY_USAGE_CONTENT_ENABLED：默认 true；关闭后不采集新正文。重启本身会清空旧窗口。
- GATEWAY_USAGE_CONTENT_MAX_RECORDS：默认 100；0 使用默认值，允许 1–1000。
- GATEWAY_USAGE_CONTENT_MAX_MEMORY_MIB：默认 64；0 使用默认值，允许 1–1024。
- GATEWAY_USAGE_CONTENT_MAX_BODY_BYTES：默认 2097152；0 使用默认值，上限 8388608。单个采集若超过整个窗口容量则被跳过。
- 旧的 max_storage_mib / GATEWAY_USAGE_CONTENT_MAX_STORAGE_MIB 不再使用，不能把原先的磁盘 GiB 预算隐式变成内存预算。

详情响应提供 storage_mode=memory；stats 包含 records、max_records、retained_bytes、max_memory_bytes、buffered_bytes、pooled_bytes、read_bytes、saved、dropped、truncated 和 deleted。旧 queued / write_failures / cleanup_failures / storage_full 字段保持兼容，内存模式恒为 0 / false。

## 升级和回退注意

- 本次已将来源仓库 ranxi2001/sub2api 的 production 更新合并至本地，合并点为 e245f8982（包含 v2.21.1）。
- 新迁移 343_usage_content_memory_window.sql **删除旧的 usage_request_contents 和 usage_content_budget 临时表**；旧正文不迁入内存，也不能恢复。usage_logs、费用和交易表不修改。
- 部署需按正常迁移流程执行，并协调更新所有旧实例：旧版本不能继续依赖已删除的正文表。没有自动部署或生产数据库操作。
- 回退旧 SQL 正文实现时，除回退程序外还须重建这两张临时表（原迁移 338 的 schema），或关闭旧正文采集；不能假设迁移记录会自动重跑，也不能恢复已清理的原文。

## 验证记录（2026-11-13，本地）

- usagecontent 生命周期 / FIFO / 字节限额 / 读取钉住 / 过期 / 重启 / 跨 Key 隔离 / 多绑定去重 / 池复用 / 并发测试通过；竞态测试重复三次通过。
- middleware / handler 的原文、gzip 输入、preread、流式写出、短写、权限及取消读取测试，以及配置和迁移范围测试通过；usagecontent、middleware、admin handler 的选定竞态测试均通过。扩展构建曾遇 C 盘临时空间不足，改用 D 盘临时目录后重跑通过。
- 前后端构建、前端 TypeScript 检查通过；前端对话 / API / 内容解析 / i18n 共 24 项测试通过；包含 worker 关闭后拒收迟到结果、worker 降级和分页展开。另回归上游合并涉及的 SettingsView / PrioritySchedulingView / AccountQualityView，265 项测试通过；本次改动文件的 ESLint 检查通过。
- Windows / Ryzen 7 7840H / Go 1.27.4 本地基准：20 KiB 合计正文约 1.68–2.92 μs、1496 B / 请求；2 MiB 输入 + 2 MiB 输出、512 B 分片约 1.41–1.62 ms、222 次分配。
- 对比先前本地基准：小请求约 15.57–20.38 μs / 33512 B，大流约 8473 次分配。新基准包含内存发布 / 淘汰、使用预热复用池；旧基准只测复制 / 入队，不包括压缩和 SQL。它们不是生产端到端 A/B 结果，不能据此承诺首字提升比例。
- 生产验收仍需同模型、同负载对比采集开启 / 关闭的客户端 p95/p99 首正文和总耗时，并同时记录保存 / 丢弃比例、CPU、分配速率和 RSS。
