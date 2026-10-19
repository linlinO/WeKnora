# 模型地址白名单更新

- 2026-10-14 20:24，按用户要求在 192.168.20.15 的 `/opt/data/WeKnora/.env` 中向 `SSRF_WHITELIST_EXTRA` 追加精确地址 `192.168.20.109`。
- 原有条目完整保留：searxng、qdrant、milvus、weaviate、doris-fe、doris-be、minio、192.168.20.181、58.250.250.47、192.168.20.99。
- 修改前备份：`/opt/data/WeKnora/backups/ssrf-20261014-2vWnNd/env.before`。备份包含环境变量，仅保存在服务器受限目录中。
- Compose 配置校验通过，重建启动 app 以加载新环境变量；镜像保持 `weknora-app:clarification-20261014-r4`。
- 已确认运行容器包含新增条目，应用日志确认加载白名单；应用 healthy、重启计数 0、网页 HTTP 200、企业微信连接恢复。
- 本次只验证白名单与应用健康，未调用新模型推理接口。用户可回到模型配置再次点击“测试连接”。
