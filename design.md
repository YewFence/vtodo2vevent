# CalDAV VTODO -> VEVENT 单向同步设计

## 背景

`tasks.org` 在部分 Android 系统上受后台限制影响，待办提醒可能无法稳定触发。Vikunja、Radicale、Baikal 等服务可以通过 CalDAV 暴露 `VTODO`，但系统日历客户端通常不会把 `VTODO` 直接显示为日历事件。因此需要一个中间同步工具，把带有明确时间含义的 `VTODO` 投影成目标日历中的 `VEVENT`，并依靠系统日历自身提醒能力完成通知。

默认事件后端选择 Radicale。Radicale 部署轻，集合路径简单，数据以文件形式落盘，适合作为这个同步器的目标事件日历。

## 目标

- 从源 CalDAV 待办集合读取 `VTODO`
- 将带明确时间点 / 时间段的任务单向同步到目标 CalDAV 事件日历
- 为生成的事件附带统一提醒
- 保持重复执行时幂等
- 在任务完成、删除或失去时间信息时删除对应事件

## 非目标

- 不做 `VEVENT -> VTODO` 反向同步
- 不试图让客户端直接显示 `VTODO`
- 不在首版实现复杂优先级提醒策略
- 不在首版处理无具体时间的纯日期任务

## 同步范围

首版只处理：

1. `DUE` 存在且带具体时间的任务
2. 同时存在 `DTSTART` 与 `DUE` 的任务

首版不处理：

- 仅有日期、没有时间的任务
- 没有任何时间字段的普通待办

## 映射规则

### 1. 时间段任务

当 todo 同时拥有：
- `DTSTART`
- `DUE`

则映射为：
- `VEVENT.DTSTART = VTODO.DTSTART`
- `VEVENT.DTEND = VTODO.DUE`

### 2. 定点截止任务

当 todo 只有：
- `DUE` 且包含具体时间

则映射为：
- `VEVENT.DTSTART = VTODO.DUE`
- `VEVENT.DTEND = VTODO.DUE + 默认持续时间`

默认持续时间通过配置控制，首版建议 5 分钟。

### 3. 字段映射

- `SUMMARY`：使用 todo 标题
- `DESCRIPTION`：保留 todo 描述，并追加源 todo UID
- `UID`：使用稳定派生规则，避免重复创建
- `VALARM`：写入统一提醒

## 稳定映射机制

为了实现幂等更新与删除，目标事件必须能稳定定位回源 todo。

方案：

- 使用原始 `VTODO.UID` 作为源标识
- 在生成事件时写入：
  - `X-SOURCE-TODO-UID:<todo uid>`
- 事件 `UID` 使用固定派生值，例如：
  - `vtodo-sync-{sha1(todo uid)}`

## 删除规则

以下情况删除对应目标事件：

- todo 被标记为已完成
- todo 从源集合中消失
- todo 不再满足同步条件（例如时间字段被移除）

## 配置项

当前实现把源待办集合和目标事件集合拆开配置，二者都以 CalDAV 访问：

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

`source` 可以指向 Vikunja 的 `/dav/projects/<project id>` 待办集合，`target` 指向 Radicale、Baikal、Nextcloud Calendar 等支持 `VEVENT` 的事件日历集合。

## 项目结构

```text
vtodo2vevent/
├─ cmd/vtodo2vevent/main.go
├─ internal/
│  ├─ caldav/client.go
│  ├─ config/config.go
│  ├─ ics/alarm.go
│  ├─ model/event.go
│  ├─ model/todo.go
│  ├─ sync/reconcile.go
│  └─ transform/projector.go
├─ config.example.yaml
├─ design.md
└─ go.mod
```

## 实现步骤

1. 初始化 Go 模块与配置加载
2. 实现内部 todo / event 模型
3. 实现投影逻辑
4. 实现通用 CalDAV 读写
5. 实现 reconcile 创建 / 更新 / 删除
6. 增加 dry-run 与日志输出
7. 用 Radicale 验证幂等性
8. 支持源和目标使用不同 CalDAV 服务
