# baikal-vtodo-sync

一个用 Go 编写的 Baikal CalDAV 同步工具，用来把带明确时间信息的 `VTODO` 单向投影成目标日历中的 `VEVENT`，并附带提醒。

## 运行方式

```bash
go run ./cmd/baikal-vtodo-sync -config config.yaml
```

如果 `config.yaml` 里没有配置 `sync.poll_interval`，程序会执行一次同步后退出。

如果配置了 `sync.poll_interval`，程序会常驻运行，启动后立即同步一次，然后按轮询间隔持续同步。

## 配置示例

参考：`config.example.yaml`

新增的持续运行配置：

```yaml
sync:
  poll_interval: "1m"
```

把它留空或删除，就会退回单次执行模式。

## Compose Sidecar

仓库已经附带 `compose.yaml` 和 `Dockerfile`，可以直接作为 sidecar 服务运行：

```bash
docker compose up -d --build
```

运行前准备：

- 复制 `config.example.yaml` 为 `config.yaml`
- 把 `config.yaml` 里的服务地址、账号和集合路径改成你自己的
- 保留 `sync.poll_interval`，这样容器会持续同步而不是跑完就退出

## 设计说明

参考：`design.md`
