# baikal-vtodo-sync

一个用 Go 编写的 Baikal CalDAV 同步工具，用来把带明确时间信息的 `VTODO` 单向投影成目标日历中的 `VEVENT`，并附带提醒。

## 当前状态

当前版本已经完成：

- 项目骨架
- YAML 配置加载与校验
- 内部 todo / event 模型
- `VTODO -> VEVENT` 投影规则
- `REPORT calendar-query` 请求发送
- CalDAV 多状态 XML 解析
- 基于 ICS 文本的最小字段提取
- reconcile 创建 / 更新 / 删除主流程
- VEVENT 与 VALARM 生成
- 可通过 `go build ./...` 成功构建

当前版本的限制：

- ICS 解析目前是首版最小实现，只覆盖本项目需要的字段
- 暂未处理带 `TZID` 参数的日期时间
- 暂未处理 recurrence、子任务、复杂 VTODO 属性
- 暂未加入自动化测试

也就是说，现在已经不是纯骨架了，已经具备首版可运行主流程；下一步最适合接入真实 Baikal 账户做联调和修正兼容性细节。

## 运行方式

```bash
go run ./cmd/baikal-vtodo-sync -config config.yaml
```

## 配置示例

参考：`config.example.yaml`

## 设计说明

参考：`design.md`
