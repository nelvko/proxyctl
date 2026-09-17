# 项目结构调整记录

2026-09-13。对应结构扫描提出的八项建议；保留命令入口、业务编排、持久化、订阅、内核与终端界面的职责分工，后续按文末复核结果合并包。

| 问题 | 最终实现 | 主要位置 |
| --- | --- | --- |
| 路径分散、卸载删除范围过宽 | config 统一默认配置/数据路径；解析失败不回退到工作目录；卸载前验证完整目标集，仅递归删除该内核的默认目录 | config/paths.go、kernel/manage.go、app/kernels.go |
| 锁依赖命令名字串 | app.WithStateLock 管理锁生命周期；CLI 定义处使用私有 mutation，TUI 按操作使用 MutateProfileMetadata / MutateRuntimeProfiles 在锁内重新装配 | app/state_lock.go、cmd/execution.go、ui/subscription_operations.go |
| 全局 HTTP client、镜像和静默开关 | 操作持有 Downloader；应用层按当前内核/环境选择传输；镜像快照与警告去重只属于本实例 | download/downloader.go、app/downloads.go |
| 布尔装配参数隐藏能力差异 | LoadProfileMetadata 与 LoadRuntimeProfiles 明确区分；Add/Use/Update/BeginEdit 提前检查 Runtime | app/subscriptions.go、subscription/service.go |
| 业务与终端交互混合 | 应用/更新结果使用有类型的状态；ui 负责详情与结果文本、外部编辑器执行；CLI writer 与 TUI 警告收集器接收下载事件 | ui/、cmd/execution.go、subscription/service.go |
| env 解析具体内核格式 | Mihomo 适配器解析 YAML；统一返回 kernel.Endpoints；env.ForProxy 生成 shell 环境 | kernel/endpoints.go、env/environment.go、subscription/download.go |
| 查询结果共享 headers map | Profile.Clone 用于 Get/List/Selected 的输出和 Add/Set 的输入 | config/subscriptions.go、subscription/service.go、subscription/metadata.go |
| 构建依赖机器目录 | unisvc 固定到远程可获取的 8a83285bcd9d；go.mod 不再 replace ../unisvc；本地联调用 go.work | go.mod、Makefile、make/orbstack.mk、scripts/smoke.sh |

## 行为与边界

- 命令名、别名与用户配置文件格式保持现有约定，不增加原先暂缓的订阅功能。
- 主配置与订阅元数据仍分别加载。元数据查询和修改不要求主配置可读或内核已安装。
- 事务从读取最新状态之前开始，覆盖下载、外部编辑器、提交及失败回滚。领域对象不重复加锁；新调用入口应使用 app.WithStateLock，避免嵌套事务。
- 元数据服务不能执行需要校验/应用的操作。删除没有安装内核的遗留订阅仍沿用清理元数据的语义。
- 卸载自定义配置目录时保留该目录；自定义二进制按单文件删除。共享或其他内核目录一律拒绝，所有删除目标先完成校验，避免部分删除后才发现错误。
- 普通 CLI 的进度只在终端输出，警告使用命令的 stderr writer。TUI 使用自己的 spinner，在操作结束时展示收集的警告；内核及 HTTP 包不自行输出。
- 内核版本查询和资源下载使用同一显式 client。自动代理按每次操作的服务和环境状态选择，不保留前一次内核地址。
- 编辑器进程及 stdio 属于 ui，订阅服务只持有临时编辑会话、校验和原子提交。
- 下载器使用显式依赖，不增加后台服务、定时更新、JSON 命令或多内核新实现。

## 验证

新增/迁移回归覆盖：

- 共享根/订阅目录/其他内核目录拒绝删除，拒绝符号链接父目录；拒绝时不提前删除有效目标；正常卸载和重复卸载保持可用。
- 默认路径拒绝保留名称、目录穿越以及缺失/相对 HOME。
- 事务等待后读取最新订阅状态；装配失败释放锁。
- 元数据服务调用运行操作返回 ErrNoRuntime，headers 的查询/写入副本相互独立。
- 下载实例之间的 client 与镜像隔离，代理条件变化不保留旧地址；警告按操作去重。
- Mihomo 的 mixed/http/socks 端口优先级、认证、IPv6 和错误端口测试随解析器迁入 kernel。
- CLI writer 不混入管道进度；TUI 警告保留并脱敏；详情默认值及本地来源提示保留。

基础验证入口为 `GOWORK=off make verify`；并执行 race 测试和真实 CLI/TUI 的隔离配置冒烟。涉及真实服务安装、卸载或网络代理切换的 OrbStack E2E 单独运行，不包含在通用 verify 中。


## 包边界复核与精简

2026-09-13。按实际职责、复用关系和依赖方向复核，而不是按命令名字或文件数量拆包。

| 原结构 | 最终结构 | 决策依据 |
| --- | --- | --- |
| cmd、cmd/kernel、cmd/sub | cmd 组装命令树；cmd/kernel 与 cmd/sub 分别负责内核和订阅命令 | 包按代码职责组织，命令树按用户使用方式组织；内核命令仍直接注册在根命令下 |
| internal/cli | 删除；Cobra 适配留在各命令包内，事务复用 app，Reporter 归 ui/progress.go | CLI 适配属于命令入口，进度属于终端展示 |
| internal/ui、internal/tui | 单一 internal/ui | CLI 和全屏界面共用表单、详情与编辑器；全屏视图、操作、表单按文件组织 |
| internal/display | internal/redact | 只有脱敏与文本清理；底层和界面均使用，不能并入依赖业务层的 ui |
| internal/httpx | internal/download | 核心是 Downloader、内容传输、镜像和重试；不是通用 HTTP API 客户端；更新也是下载能力的调用者 |
| httpx/Ungzip | kernel/archive.go 的私有 ungzip | 只由内核安装使用，属于安装过程 |

`download` 的名字不限制它只能被安装或添加调用。订阅更新的来源重新获取、内核升级的版本文件及资源获取，都复用它；版本比较、落盘切换、重启和回滚仍由 subscription/kernel/app 负责。不会为 update 再新建一份网络实现。

下载入口由原来的包级 Download / DownloadVia 与 Downloader.Download 收敛为实例方法 `Downloader.Fetch`，请求选项显式传入；私有单次请求函数负责重试循环中的一次尝试。保留选定 transport 的实例隔离、超时、TLS、重定向、响应头回调及进度行为。

界面侧导出 `RunSubscriptions` 供命令入口启动管理器；警告收集类型改为私有。`subscription_view.go` 保存布局与样式，`subscription_model.go` 保存模型和状态转换。Cobra 依赖仅存在于 cmd 及其命令子包，app/subscription/kernel/download/redact 不依赖 ui。

此前包合并阶段保留 cmd/kernel 和 cmd/sub，相比整理前包总数减少 2 个；该阶段未改变命令和配置格式。本轮配置模型及运行装配调整见下文。


### 命令包边界修正（2026-09-17）

此前将所有命令统一到 cmd，过于强调减少包数量。恢复 cmd/sub 与 cmd/kernel，各自管理相关命令定义、参数、补全和交互选择。根命令通过 sub.NewCommand 与 kernel.NewCommands 统一注册；内核命令仍使用 proxyctl install、upgrade、start 等顶层形式。代码包边界与 CLI 命令层级不必一一对应；internal/kernel 继续负责内核实现。

子包内保留少量私有 Cobra 回调适配与终端判断，锁入口只有 app.WithStateLock 一份，回滚由各业务操作负责，展示仍使用 ui。不恢复 internal/cli，不向 app 注入 Cobra 依赖，也不通过父包回调或接口传递这些小型辅助函数。命令行为和其余包边界保持不变。


## 职责与命名复核实施（2026-09-17）

本轮按最终职责调整，不保留发布前的兼容层。命令名、别名和参数不变；订阅配置有明确变更：`Profile.Update / UpdateConfig / update:` 改为 `Profile.Download / DownloadConfig / download:`。调度功能继续暂缓，模型删除 enable、interval、cron；严格解码拒绝旧节点、调度字段及未知字段，不自动迁移用户文件。

| 调整 | 结果与原因 |
| --- | --- |
| 内核启动配置 | Mihomo.ServiceSpec 提供启动命令和参数，app 安装流程不再拼装 Mihomo 的 -d/-f |
| 开启代理流程 | app/proxy.go 的 StartProxy 负责解析端点、启动及等待可用；cmd/on.go 只处理 shell 与输出 |
| 删除订阅装配 | app.RemoveProfile 仅在强制删除当前选中订阅时装配内核，保留无内核时清理遗留订阅的行为 |
| CLI/TUI 能力一致 | TUI 启动只加载元数据；修改与删除未选中订阅使用 MutateProfileMetadata，下载、应用和内容编辑使用 MutateRuntimeProfiles |
| 锁名称 | Transaction 改为 WithStateLock；仅提供互斥，不承诺自动提交或回滚；state_lock.go 保存这一边界 |
| 状态名称 | SelectedKernel、SelectedName、Selected 和 Entry.Selected 表示选中；外部服务接口 IsActive 仍表示运行 |
| 重复 API | subscription.Use 统一返回应用结果，Update 统一接收 DownloadOptions 并返回逐项结果；移除 UseResult、UpdateResults、UpdateWithOptions 及错误值专用包装；删除无生产调用的 Mihomo.InboundURL |
| 代理客户端 | KernelClient 改为 ProxyClient，表达按 HTTP/SOCKS URL 构建客户端的实际能力 |

文件按职责组织，包边界保持 cmd/kernel、cmd/sub、app、config、kernel、subscription、download、ui、redact、env：

- subscription/service.go 保留服务定义、查询及能力检查；增删、应用、编辑、更新、存储分别归 add.go、remove.go、apply.go、edit.go、update.go、storage.go；元数据变更和名称校验归 metadata.go。
- kernel/mihomo.go 保留适配器及校验；GeoData、发布资产与版本、配置端点解析分别归 mihomo_geodata.go、mihomo_release.go、mihomo_config.go；endpoints.go 仅保存通用端点类型。
- download/runtime.go 拆为 downloader.go、client.go、mirror.go，传输逻辑继续留在 download.go。
- cmd/kernel/kernel.go 仅注册命令，服务控制和安装交互选择分别归 service.go、select.go。
- ui 的订阅模型、视图、异步操作统一命名为 subscription_model.go、subscription_view.go、subscription_operations.go。
- 原 optimize_test.go、semantics_test.go 按订阅功能归入测试文件，cmd 的订阅集成测试归 sub_test.go。

新增回归覆盖：旧配置/未知字段拒绝、下载设置完整往返、服务启动参数、启动就绪与失败不启动、按删除目标加载状态、TUI 元数据操作不依赖损坏的主配置。

本轮验证：`GOWORK=off make verify`、`GOWORK=off go test -race ./...` 全部通过；调整前后 23 项命令帮助输出完全一致。测试使用隔离配置和本地监听端口，未操作真实内核服务。
