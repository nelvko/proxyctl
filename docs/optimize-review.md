# Optimize 实现 review

本轮完成 todo.md 中 O1–O15 与关联 B1–B5。O4 按实际职责拆分，O7 保留现有帮助分组，O11 保留已实现的别名；feat 中的多内核订阅归属、调度、i18n、JSON 等长期功能没有混入本轮。

## 1. 对外命令与参数

| 入口 | 最终行为 |
| --- | --- |
| `use [name]` | 无参数且有终端时选择已安装内核；脚本缺参数时给出明确提示。 |
| `sub add [url]` | 保持只添加；无当前订阅时提示 use/on。省略名称时从响应或来源推导。 |
| `sub add <url> --use` | 添加后应用，结果区分服务重启与继续停止。 |
| `sub add <url> --download-via=kernel` | 必须通过运行内核下载，并保存路由偏好；支持认证和 SOCKS，优先于环境代理。 |
| `sub add/update --keep-failed` | 保留完整但校验失败的候选内容，打印实际诊断文件路径。 |
| `sub add --user-agent` | 设置此次及后续更新使用的 HTTP User-Agent。 |
| `sub update` | 只更新当前选中的订阅；没有当前项则报错。 |
| `sub update name...` | 更新明确指定的项；重复名称只处理一次。 |
| `sub update --all` | 显式更新全部，与名称互斥。 |
| `sub modify <name>` | 修改名称、来源、UA、超时、下载路由、TLS 或请求头；不会下载、应用或重启。 |
| `sub show [name]` | 查看元数据，默认当前项；`--show-url` 才展示完整来源。 |
| `sub remove [name]` | 移除登记和缓存；保留 rm/delete/del 别名。force 语义保留。 |

下载路由统一为 `--download-via env|direct|kernel`，不再保留旧布尔参数。元数据修改仅使用 `modify`，不再注册 `set` 别名；`subscription`、`ls` 和移除别名继续作为输入便利保留。

## 2. 下载与文件身份

`internal/subscription/download.go` 负责来源读取、名称推导、请求配置和诊断文件。`internal/download/download.go` 在原有重试/限时/进度机制上接受请求参数及响应头回调，没有额外发 HEAD 或复制下载流程。

名称优先级为显式名称、Content-Disposition、来源文件名/域名、自动名称。响应名经路径清理、控制字符移除、扩展名剥离和长度限制；自动名称冲突追加序号，显式名称冲突直接失败。filename* 与 filename 由标准库 mime 解析。

新缓存文件使用独立的随机稳定名称，重命名只更新元数据和当前选择引用。之后重新添加旧名称不会覆盖原订阅缓存。修改来源后保留旧有效内容，直到下一次下载和校验成功。

失败诊断文件位于应用配置目录的 diagnostics/，只保留完整且校验失败的内容。目录 0700、文件 0600；不登记、不应用，用户自行删除。空内容、读取失败、网络中断、取消不保留；内核无法执行与内核已执行但拒绝配置也分别处理。更新失败不会覆盖原缓存；应用失败时保留已验证的新缓存以便重试。

## 3. 端点、请求设置和展示

`internal/kernel/endpoints.go` 共用解析实际端点，`shell.go` 专注 shell 渲染。通过内核下载与 shell 导出读取相同的端口、地址和认证；IPv6 使用正确的 host:port 编码，仅 SOCKS 的内核也可作为下载代理。

`--download-via=kernel` 表示必选路由，不静默直连、不自动启动服务。默认下载仍遵循环境代理规则。UA、Headers、SkipCert 实际作用于请求，TLS 配置按请求隔离；跨来源重定向会去掉配置的自定义请求头。调度字段尚无实现，下载/修改元数据时遇到非默认值会报错。

`internal/redact` 集中处理人类可读 URL 和错误脱敏，并保留 error unwrap。隐藏用户名密码、路径中的 token、query 和 fragment；终端仍能辨认来源主机。实际 shell 变量内容不脱敏。`show` 仅显示 header 名称，不打印其值。

## 4. TUI、取消与事务

TUI 搜索先匹配名称，再匹配来源域名；域名命中不拿它的字符索引高亮标题。Enter/u/a 是常用动作，s/m/e/dd 在完整帮助中，分别查看详情、修改元数据、编辑配置、移除。项目主动将 list 的 Quit 绑定为 q，v 不再有误导含义；无需改依赖。

CLI 下载、校验、编辑和内核元数据请求继承调用方 context。TUI 的 Ctrl+C 取消忙碌操作，保持 busy 直到完成；系统退出信号也经过取消和等待路径。已经进入提交/回滚的事务会收尾后再解锁。

`internal/config/lock*.go` 使用 OS 文件锁；CLI 在加载状态前加锁，TUI 每次操作拿锁后重新加载状态。查询、补全和空闲 TUI 不持锁。当前选择简单的完整事务串行化，下载与编辑器也在锁内；代价是其他写命令需要等待，但不会用旧状态覆盖新状态。锁等待可取消，进程结束由 OS 释放，不依赖删除锁文件。

服务管理依赖 unisvc 当前没有 context 的接口，所以已发出的服务操作同步等待返回；没有用后台 goroutine 假装操作已取消。系统服务的真实启停仍需要在 Linux/systemd 环境验证。

## 5. 建议 review 顺序

1. `cmd/sub/add.go`、`update.go`、`modify.go`、`show.go`、`remove.go`：命令语义及帮助。
2. `internal/subscription/download.go`、`metadata.go`、`service.go`：请求参数、缓存身份、失败回滚和结果。
3. `cmd/execution.go`、`internal/config/lock*.go`、`internal/ui/subscription_operations.go`：锁范围与取消边界。
4. `internal/kernel/endpoints.go`、`internal/download/download.go`、`internal/redact`：代理端点、请求隔离和脱敏。
5. `internal/ui/subscription_model.go`：搜索、按键、结果反馈和等待完成。

## 6. 验证

针对性测试覆盖：响应头名称/中文/冲突/显式名称、请求头与 UA、认证内核代理与环境覆盖、TLS 隔离、跨来源重定向、诊断文件权限和清理、取消不提交、重命名后重用旧名称、元数据保存失败回滚、更新范围、只读补全、跨进程锁等待及并发添加、TUI 按键/取消/域名高亮、IPv6 解析，以及 bash/zsh/fish 提示命令的参数还原。

最终验证通过：

- `go test -race ./...`
- `go vet ./...`
- `GOOS=linux go vet ./...`
- `go build -o proxyctl .`（已更新本地 macOS arm64 二进制）
- `GOOS=linux GOARCH=amd64 go build`（Linux amd64 交叉构建通过）
- `git diff --check`
- 使用全新临时配置目录运行真实 CLI：添加、名称推导、元数据修改、查看、更新范围、诊断文件及移除别名通过。
- 使用 PTY 启动真实 TUI：成功渲染订阅，展开帮助，v 保持界面，q 正常退出。

没有访问真实订阅源，也没有启动或停止本机系统服务。真实 Linux/systemd 的运行环境验收尚未执行。

## 7. Clash Verge 对照中的优化与 bug（2026-09-07）

本次实现 CV1/CV2/CV3/CV5，并澄清 CV8 的关联文件语义；描述、配额与时间状态、调度、独立副本、覆写、预览、JSON、批量 TUI 和导入便利功能仍留作 feat。

- `cmd/sub/options.go` 统一下载参数：add/modify 持久保存，update 对每个目标临时覆盖，显式 0/false/空 UA 与未传参数分开处理。路由枚举带补全，旧 --via-kernel 已移除。请求头按名称替换或删除，拒绝同一头的重复/冲突输入。
- `internal/subscription/options.go` 共用默认值、校验和配置覆盖；覆盖会复制请求头 map，不改变原配置。默认超时仍为 10s，默认 UA 保持 Go-http-client/1.1，未引入产品 UA 或格式适配策略。
- `internal/subscription/download.go` 按路由建立独立 HTTP 客户端；direct 不读取显式代理策略，env 使用标准环境代理，kernel 强制走受管内核。HTTPS 校验开关仅作用于订阅下载。
- `internal/subscription/details.go` 为 CLI/TUI 共用展示；区分默认 UA、header 提供的 UA 和配置路由，不探测运行状态。file 来源明确标为关联本地文件，不再显示不适用的 HTTP/TLS 设置。
- 新增/修改表单支持高级请求设置；TUI 使用 s 查看、m 修改，e 仍编辑内容。失败后重新建立可编辑表单，保留原始名称、URL 和请求设置，避免把失败请求推导出的名称当作用户输入。CLI 交互 add 也允许保留输入重试。
- TUI 编辑元数据时不持锁；提交后拿锁读取最新配置并核对打开表单时的记录，发现冲突则拒绝覆盖。原有下载、校验、提交与应用事务边界不变。

验证新增覆盖直连/代理路由、TLS false 恢复验证、UA 默认值、总下载预算、临时覆盖不落盘、header 大小写处理、失败表单恢复和并发编辑冲突。未增加依赖。

2026-09-12 收尾验证通过：全量 `go test -race ./...`、`go vet ./...`、本地构建和 `git diff --check`。使用隔离配置与假内核运行实际 CLI/PTY，确认临时请求设置不落盘，详情查看、元数据改名、失败后草稿恢复、高级设置页面、取消和退出均正常。Linux vet 与 amd64 交叉构建已在本轮前段通过；未运行真实 systemd 服务验收。

## 8. config / app 职责整理（2026-09-12）

保留持久化数据与应用流程两层，主配置类型改为 `config.Config`，读写入口改为 `config.Load` / `config.Save`；`app.App` 保留作为已加载状态与应用流程入口。磁盘 YAML 字段、文件位置与命令语义不变。

| 原位置 | 新位置与职责 |
| --- | --- |
| config/app.go | config/config.go：主配置模型与读写；公共原子写入移到 storage.go |
| config/sub.go | config/subscriptions.go：订阅模型与读写；公共目录与文件路径移到 paths.go |
| app/app.go 中的内核流程 | app/kernels.go：安装、卸载、升级、切换与回滚 |
| app/app.go 中的服务控制 | app/service.go：start/stop/restart |
| CLI / TUI 各自创建订阅服务 | app/subscriptions.go：共用 LoadProfileMetadata / LoadRuntimeProfiles 与 App.Profiles 装配 |
| subscription/tui*.go | tui/subscriptions.go、profile_form.go、operations.go 及对应测试 |
| 表单文件中的来源校验 | subscription/validation.go：CLI/TUI 共用业务校验 |

TUI 自己持有交互 context，只调用订阅服务的公开 API；不再访问服务内部字段，也不再重复加载主配置、设置镜像、创建内核。查询直接读取订阅状态，写操作仍先拿锁再经应用层装配，编辑器与提交/回滚仍处于原事务范围内。

`app.Load()` 移除设置全局 HTTP 镜像的副作用。需要下载或校验的应用流程显式配置镜像；install 的镜像设置仍随配置保存，upgrade 的覆盖只保留在本次内存快照中。原有 HTTP 客户端机制没有在本次重构中整体重写。

新增验证覆盖加载不修改下载环境、元数据操作不读取损坏的主配置、TUI 在拿锁后读取最新状态，以及装配失败释放锁。原有测试与 TUI 行为测试随代码迁移。

本次验证通过：`go test -race ./...`、macOS/Linux `go vet ./...`、本地构建、Linux amd64 交叉构建、`git diff --check`；隔离配置下的真实 CLI/PTY 验证覆盖临时设置不落盘、详情、元数据改名、失败草稿恢复、高级设置、取消和退出。未调用真实系统服务。


## 9. 后续架构边界整理（2026-09-13）

第 8 节的包职责划分继续保留。装配入口进一步拆为 `LoadProfileMetadata` / `LoadRuntimeProfiles`，全局下载配置替换为实例依赖，事务由 `app.WithStateLock` 管理。路径安全、展示边界、端点解析、状态复制和构建环境的完整变更见 [架构调整记录](architecture-review.md)。本次不实现 Clash Verge 对照评估中暂缓的 feat。
