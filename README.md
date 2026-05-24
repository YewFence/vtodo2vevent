# vtodo2vevent

一个用 Go 编写的 CalDAV 同步工具，用来把源待办集合里带明确时间信息的 `VTODO` 单向投影成目标日历集合里的 `VEVENT`，并附带提醒。

## 运行方式

```bash
go run ./cmd/vtodo2vevent -config config.yaml
```

如果 `config.yaml` 里没有配置 `sync.poll_interval`，程序会执行一次同步后退出。

如果配置了 `sync.poll_interval`，程序会常驻运行，启动后立即同步一次，然后按轮询间隔持续同步。

## 配置示例

参考：`config.example.yaml`

配置分为源待办集合和目标事件集合，两边都使用标准 CalDAV 访问。源可以是 Vikunja、Radicale、Baikal 或其他支持 `VTODO` 的服务，目标需要是支持 `VEVENT` 的日历集合。

```yaml
source:
  type: "caldav"
  base_url: "https://vikunja.example.com"
  username: "user"
  password: "tk_xxx"
  todo_collection: "/dav/projects/12"

target:
  type: "caldav"
  base_url: "https://radicale.example.com"
  username: "user"
  password: "pass"
  event_collection: "/user/task-reminders/"

sync:
  timezone: "Asia/Shanghai"
  due_only_duration_minutes: 5
  alarm_minutes_before: 10
  dry_run: false
  poll_interval: "1m"
```

持续运行由 `sync.poll_interval` 控制。

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

1. 复制 `config.example.yaml` 为 `config.yaml`
2. 把 `config.yaml` 里的源和目标 CalDAV 地址、账号和集合路径改成你自己的配置
3. 保留 `sync.poll_interval`，这样容器会持续同步而不是跑完就退出

Vikunja 的 CalDAV 路径通常在 `/dav/projects/<project id>`，它适合作为 `source`。目标 `target` 需要选择真正支持 `VEVENT` 的 CalDAV 日历集合，例如 Radicale、Baikal 或 Nextcloud Calendar。

## 设计说明

参考：`design.md`
