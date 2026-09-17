## optimize

> 更新日期：2026-09-12。按用户授权完成实现，新增名称和实现取舍交由最终 review。
> 详细变更与验证见 [optimize review](docs/optimize-review.md)。feat 保留为后续规划。
> 内核仍是默认对象，操作位于顶层；订阅使用 sub 分组；on/off 管理当前 shell，start/stop/restart 管理共享服务。

- [x] **O1：添加后的下一步提示。** 没有选中订阅时提示 `sub use <name>` 和 `on`，不自动使用。已有选择或显式 `--use` 时不重复引导。名称安全转义。
- [x] **O2：保留校验失败文件。** `sub add/update --keep-failed` 将完整但未通过校验的下载保留在配置目录的 `diagnostics/`；目录 0700、文件 0600，输出实际路径且明确未应用。默认清理并提示重试选项；空文件、网络中断、取消不保留。
- [x] **O3：响应头默认名称。** 同一次 GET 读取 Content-Disposition，标准库解析 filename*/filename；显式名称优先，然后响应文件名、本地文件名/来源域名、自动名称。清理路径/控制字符/配置扩展名并限制自动名称长度；自动重名追加序号，显式重名报错。更新不改名。
- [x] **O4：按职责拆分 env。** 因 B3 需要共用端点解析，已把解析放入 `internal/kernel/endpoints.go`，`shell.go` 保留 shell 检测和环境语句渲染；没有只为统一字面做文件改名。
- [x] **O5：use 空参数。** 顶层 `use [name]` 在终端选择已安装内核，标记当前项；脚本缺名称时提示 list/use，无已安装内核时提示 install，未知名称列出已安装项。保留 selected 表示持久化选择。
- [x] **O6a：TUI 帮助顺序。** 常用操作优先，编辑/移除放入完整帮助，窄终端提供包含 q 的紧凑提示。
- [x] **O6b：v select。** 项目初始化时显式将 Quit 绑定为 q，移除重复退出帮助；v 不再误导为选择/退出。未更换、修改或 fork bubbles 依赖。
- [x] **O6c：来源搜索。** 优先匹配名称，再匹配来源主机名；不搜索路径/token/query。域名命中不套用标题字符高亮，操作目标仍由实际选中项决定。
- [x] **O7：帮助分组。** 保留 Kernel Management / Service Control / Shell Integration，评估后无需调整。
- [x] **O8：下载路由。** `--download-via env|direct|kernel` 明确互斥路由，kernel 显式优先于环境代理；内核未运行或入站不可用则报错，不静默回退。支持 HTTP/SOCKS、监听地址和认证；`--use` 仍只负责添加后选择并应用。
- [x] **O9：订阅移除入口。** 正式名 `sub remove`，别名 rm/delete/del；`--force` 的停止服务和删除已应用配置语义保留。
- [x] **O10：元数据修改。** `sub modify <name>` 支持 --name/--url/--user-agent/--timeout/--download-via/--insecure/--header/--remove-header。仅保存元数据，不下载、不应用；支持清空 UA、false 和默认超时。同步当前选择引用，缓存使用与名称无关的稳定随机文件名。
- [x] **O11：ls 别名。** 保留 sub ls、subscription ls 及相关别名测试。
- [x] **O12：更新范围。** CLI 无参数只更新当前选择，--all 更新全部，与名称参数互斥。没有当前选择则给出明确错误；TUI u 继续更新高亮项。
- [x] **O13：查看元数据。** `sub show [name]` 默认查看当前选择，展示缓存、超时、路由、UA、TLS 设置和 header 名称。普通输出隐藏来源凭据，--show-url 显式展示完整来源；CLI/TUI 共用 ProfileDetails 展示，后续 JSON 留作 feat。
- [x] **O14：结果和过程反馈。** 更新逐项返回 updated/unchanged/failed，应用时区分重启与保持停止；应用失败明确缓存留待重试。start/stop 显示 already 状态，服务操作在终端显示进行中提示。CLI/TUI 消费同一应用和更新结果。
- [x] **O15：动态补全。** use/uninstall/upgrade 补全已安装内核，sub use/update/edit/remove/modify/show 补全已登记名称；只读配置，不加载服务、不下载、不弹交互。

## bug

- [x] **B1：配置字段不生效。** Headers/UserAgent/SkipCert 已接入实际请求，TLS 设置不污染共享客户端；跨来源重定向不转发自定义 header。未实现的调度字段有非默认值时明确报错。
- [x] **B2：取消贯通。** CLI context 贯通订阅下载、编辑器、内核校验、geodata 和版本元数据请求；版本探测也有调用方 context 和超时。TUI 忙碌时 Ctrl+C 取消当前操作并等待完成；退出信号经过相同路径，提交/回滚期间不提前释放锁。unisvc 的服务操作接口没有 context，已经进入的服务调用仍同步等其返回，再完成提交/回滚。
- [x] **B3：内核端点解析不一致。** shell 导出、订阅下载和内核自动代理共用 env.Resolve。修正认证、IPv6、通配地址和端口校验，支持仅 SOCKS 的入站；显式内核路由不会被环境变量覆盖。
- [x] **B4：普通输出泄露 URL 凭据。** list/TUI/on 提示/status/错误使用统一脱敏，隐藏 userinfo、路径、query、fragment。env/on 的可执行 shell 输出仍保留真实值；show --show-url 是明确的查看入口。
- [x] **B5：跨进程写入丢失。** CLI 写操作与 TUI 单次操作共用 OS 文件锁，从读取到提交/回滚完整串行化；拿锁后重新加载状态。查询、补全和空闲 TUI 不占锁。当前有意将下载/编辑器纳入事务锁，避免引入乐观提交与冲突恢复协议；并发写命令会等待，可取消。
- [ ] sub 新 add 订阅但未 use 订阅时，执行 on 子命名会报错提示   Read kernel config: open /home/nelvko/.config/proxyctl/mihomo/config.yaml: no such file or directory.
## feat
- [ ] 后期支持 install /use 多个 kernel 时，怎么管理订阅？例如当前 use 的mihomo，且有 2 个对应订阅。若再 install 并 use clash /singbox 后，再执行 sub 时，应该展示 0 个，还是也展示 mihomo 的2个订阅。也需要考虑后期多个内核来回切换时的 sub 的订阅展示情况，不要仅考虑初次 install 并 use 别的内核的情况。
什么方案比较好？sub add 时指定是哪个内核的？或者直接强隔离？通过--all 来显示全部的？
- [ ] 目前仅支持 user 域的 systemd unit，是否建议可安装系统级的 unit
- [ ] i18n 推荐怎么实现？推荐使用哪个依赖库？
- [ ] sub list 新增 json 等格式化输出的选项
- [ ] 新增个 类似mixin 的功能，可以为单个/所有订阅进行覆写
- [ ] 定时更新订阅的可行性
- [ ] install支持三方的魔改 clash 内核
- [ ] 挖掘 shell hook 可以新增的功能，例：进入某目录时自动开启/关闭代理

## Clash Verge 订阅管理对照评估

> 评估日期：2026-09-07；实现验证更新：2026-09-12。按用户要求先处理优化与 bug，feat 暂缓；完成状态与本轮范围见下表。
> 依据：用户截图、本机 Clash Verge 的实际界面、官方文档和 v2.5.2 标签源码。通过 macOS 辅助功能接口打开订阅页与 NEW 表单，切换 Remote / Local，随后 CANCEL；未保存表单或触发订阅更新。标签版本用于固定源码依据，不据此推断本机版本。

### 命令名称与职责

现有 `add / list / show / modify / edit / use / update / remove` 可以保留。Clash Verge 将“编辑信息”和“编辑文件”分为两个入口，恰好支持当前 `modify` 管元数据、`edit` 管配置内容的划分；无需为凑 `get/set` 对称而改名。仅保留 `modify`，不注册 `set` 别名，避免与 `show`、`use` 混淆。[订阅菜单源码](https://github.com/clash-verge-rev/clash-verge-rev/blob/v2.5.2/src/components/profile/profile-item.tsx)

| Clash Verge 功能 | 当前 proxyctl | 建议 |
| --- | --- | --- |
| URL 快速导入 / NEW | `sub add <url>` / `sub add` 表单 | 已补高级请求设置 |
| Remote / Local | HTTP(S) 与 `file://`，但都保存来源并可更新 | 明确本地副本与关联文件的区别，见 CV8 |
| Name / Descriptions | 名称已有，描述缺失 | 增加可清空的 `--description` |
| 编辑信息 / 编辑文件 | `modify` / `edit`，TUI m/e | 已补齐对应入口 |
| User Agent / Timeout | add/modify 持久设置，update 单次覆盖 | 已统一参数语义 |
| System Proxy / Clash Proxy | `--download-via env/direct/kernel` | 已统一为互斥路由 |
| 允许无效证书 | CLI/TUI 已支持 TLS 设置 | `--insecure` 显式启用，默认校验证书 |
| 更新间隔 / 自动更新 | 有配置字段，当前明确拒绝启用 | 调度必须有独立执行机制，见 CV7 |
| 流量 / 到期 / 更新时间 | 尚无持久化的对应元数据 | 从同次请求响应头提取并缓存展示 |
| 单项刷新 / 更新全部 | `update [name...]` / `update --all` | 已覆盖；支持单次请求设置 |
| 全局 / 单订阅覆写 | feat 中已有想法 | 优先实现声明式覆写，见 CV9 |
| 查看运行配置 / 重新激活 | show 只看元数据；use 可重新应用 | 补预览能力，重新应用仍复用 use |
| 批量操作 / 二维码 / 文件管理器 | 仅部分有对应能力 | 批量已有 CLI 基础，桌面专属交互暂缓 |

### 本轮范围与状态

| 范围 | 类型 | 状态 |
| --- | --- | --- |
| CV1 下载路由及临时覆盖 | 优化现有下载设置 | 已完成 |
| CV2 TLS/header 管理入口 | 补齐已有底层能力 | 已完成 |
| CV3 默认值及实际展示 | 优化/提示修正 | 已完成；保持默认 10s 和 Go UA，产品 UA 策略暂缓 |
| CV5 表单恢复、CLI/TUI 操作一致性 | bug/体验优化 | 已完成；含并发修改冲突检查 |
| CV8 现有 file:// 的关联来源说明 | 提示修正 | 已完成；独立副本模型暂缓 |
| CV4、CV6～CV12 的新增能力 | feat | 暂缓；包括描述、状态统计、调度、独立副本、覆写、预览、JSON 等 |

详细实现与验证见 [review 第 7 节](docs/optimize-review.md#7-clash-verge-对照中的优化与-bug2026-09-07)。以下保留评估依据，已完成项的落实说明优先于原拟议方案。

### 建议任务

优先级：P1 是现有行为缺口或基础模型问题；P2 是有明确价值的扩展；P3 是可选便利功能。已勾选项的参数已实现；未勾选项仍是拟议接口，不能按已支持用法执行。

- [x] **CV1 · P1：用互斥路由替代单一代理布尔值，并支持本次更新覆盖。**
  落实：已统一 --download-via 枚举，移除 --via-kernel；update 临时覆盖路由/超时/UA/TLS/header，保持订阅持久设置不变。
  评估时 `--via-kernel=false` 只是恢复环境代理，无法表达“即使配置了环境代理也直连”。建议 add/modify/update 使用 `--download-via env|direct|kernel`：env 遵循 HTTP(S)_PROXY/NO_PROXY；direct 禁用应用层显式代理；kernel 强制经运行中的受管内核，不可用即报错。direct 不承诺绕过操作系统 TUN/VPN。不要把环境变量代理命名为 system，它并不等于 macOS 系统代理或 PAC。
  add/modify 保存设置；update 只覆盖本次调用，未传参数时继承订阅值；`--all` 的显式覆盖作用于本次全部目标。统一规则也适用于 timeout/UA/TLS。已用 `--download-via` 取代 `--via-kernel`，无需另开 `update-direct`、`update-proxy` 子命令。show 展示已配置策略，不把静态配置误称为已验证的实际连接路线。
  验收重点：direct 确实不使用环境代理；kernel 不静默回退；临时覆盖不写回配置；file 来源不使用网络设置。参考表单源码中两个开关的相互取消逻辑，以及菜单中的普通更新/代理更新入口：[表单](https://github.com/clash-verge-rev/clash-verge-rev/blob/v2.5.2/src/components/profile/profile-viewer.tsx)、[菜单](https://github.com/clash-verge-rev/clash-verge-rev/blob/v2.5.2/src/components/profile/profile-item.tsx)。

- [x] **CV2 · P1/P2：补齐 TLS 与请求头的管理入口。**
  落实：add/modify/update 已支持 --insecure 与重复 --header；modify/update 支持 --remove-header。显式 false、header 大小写与冲突处理已有验证。
  P1：add/modify 提供 `--insecure`，`modify --insecure=false` 恢复订阅下载的证书校验；update 同名参数仅覆盖本次。show 明确其只影响 HTTPS 订阅请求，不影响节点的 TLS 配置。保持默认校验，不因普通连接失败自动关闭校验。
  P2：Headers 已有底层实现，可增加可重复的 `--header 'Name: value'` 和 modify 的 `--remove-header Name`；按不区分大小写的名称更新/删除，专用 `--user-agent` 与通用 header 的冲突明确报错。值不进入普通输出，继续保持跨来源重定向的保护。先支持当前模型的一键一值，暂不引入复杂的任意 HTTP 请求编辑器。

- [x] **CV3 · P2：明确超时与 User-Agent 的有效默认值。**
  落实：默认超时集中为 10s，0 恢复默认；总预算覆盖下载和重试，慢响应/取消已验证。默认 UA 保持 Go-http-client/1.1，并与详情展示一致；产品 UA 与格式兼容策略属于 feat，暂缓。
  保留带单位的 `--timeout 30s` / `1m`，不用 UI 的无单位数字。`0` 统一表示恢复默认，不表示无限等待；说明预算覆盖整个下载及其重试，不含后续内核配置校验。当前默认 10s，截图占位值为 60 秒；仅凭截图不能判断最佳值，先用慢响应、重试和取消场景验证，再决定是否统一调整到 60s，避免 add/modify/show 各有一份默认常量。
  UA 未配置时，show 应显示实际默认值与“未自定义”，而非空白。评估提供带真实版本的 proxyctl UA；对必须识别 Clash 格式的来源使用明确的兼容策略，不直接冒用 clash-verge 的产品版本。保持用户自定义和清空入口，格式兼容性结合原 feat 的多内核问题处理。[官方文档对 UA 与响应头的说明](https://www.clashverge.dev/guide/url_schemes.html)

- [ ] **CV4 · P2：描述作为独立元数据，不塞进名称。**
  add/modify 增加 `--description`，空字符串可清空；show 展示，TUI 详情展示并可按描述搜索。名称继续作为命令选择标识，描述适合记录用途、套餐或来源备注。更新不覆盖用户填写的名称与描述。避免为普通备注同时引入 description、remark、comment 三套同义字段。

- [x] **CV5 · P1：TUI 的新增、修改与失败恢复形成完整流程。**
  落实：高级请求表单、s 详情、m 元数据修改和终端中的 modify 无 flag 表单已实现。失败/取消操作保留草稿，成功/取消表单清理草稿；并发修改检查防止覆盖较新配置。
  评估时新增表单只有 URL 和名称，不能配置 UA/超时/路由；TUI 也没有 modify/show 对应入口。建议常用字段直接展示，高级请求设置放入可展开区；Remote/Local 根据来源显示适用项。增加详情和元数据修改操作，保留现有 e 编辑配置内容，帮助文案明确区分。
  原代码中 `addSubmittedMsg` 在实际 Add 成功前调用 `resetAddForm()`，下载或校验失败会丢失已填内容。应保留草稿，失败后返回原表单和具体错误，允许修正后重试；成功或主动取消才清理。CLI 的 `sub modify <name>` 无 flag 时可在终端打开同一表单，非终端仍要求显式参数。与 CLI 共享参数校验和业务调用。

- [ ] **CV6 · P1：持久化订阅状态，补流量、到期与更新时间。**
  从下载的同次响应读取 `subscription-userinfo`（upload/download/total 为字节，expire 为时间戳）与可选 `profile-web-page-url`。用量为上传与下载之和；缺失、非法值和 total=0 不擅自解释为零用量、已过期或无限套餐。它们属于订阅生态约定，并非每个来源都会提供。[响应头文档](https://www.clashverge.dev/guide/url_schemes.html)
  区分最近尝试、最近成功检查、内容变化、应用结果；内容 unchanged 也可能有更新的配额和检查时间。选中状态不等于服务正在运行，缓存版本不等于已应用版本。list 保持简洁，show/TUI 详情展示完整信息，默认查询不联网。
  建议只在完整响应并通过配置校验后提交本次内容及响应元数据；下载/校验失败保留上次成功信息并标记过期，另存脱敏的最近失败结果。应用失败独立记录，不能倒退为“下载失败”。首次失败没有成功时间；无对应响应头明确显示未知。状态数据与用户可编辑的下载设置分开建模，便于 JSON 与后续调度共用。

- [ ] **CV7 · P2：细化原 feat 的定时更新，开关、间隔、执行器分开。**
  拟用 add/modify 的 `--auto-update` 与 `--interval 6h`：显式开启才执行，关闭保留间隔；设置间隔本身不自动开启。用户间隔优先于服务端建议；`profile-update-interval` 响应头单位是小时，与 Verge 表单分钟不同，必须统一转换并保留来源信息。服务端建议不自动覆盖用户值或替用户开启调度。
  当前 CLI 运行即退出，保存两个字段不等于实现定时更新。结合项目已有 systemd 支持，优先评估用户级 timer 定期唤起一次“处理到期订阅”的任务，复用下载、锁、校验和应用流程；暂不额外维持常驻调度 daemon，也不同时支持 interval 与 cron 两套配置。实现时另行明确后台入口与安装/移除命令，不能把普通 `update --all` 当成只处理已启用订阅的调度任务。
  show 区分订阅启用状态、执行器可用状态和下次到期；补最小间隔、失败退避、错过任务后的补跑及进程互斥。非当前订阅只更新缓存，当前订阅应用仍保持原服务运行状态。Verge 的 timer 对缺失 allow_auto_update 使用 true 回退，这一细节不直接照搬到 proxyctl：[调度源码](https://github.com/clash-verge-rev/clash-verge-rev/blob/v2.5.2/src-tauri/src/core/timer.rs)。

- [ ] **CV8 · P1：区分远程订阅、本地副本与关联本地文件。**
  本轮仅落实提示修正：CLI 帮助、详情和 TUI 明示 file:// 关联原路径，更新会重新读取；HTTP 选项不用于 file 读取。独立副本、路径导入和类型模型仍属于 feat，未实现。
  实机 Local 表单只保留名称、描述和 CHOOSE FILE；官方文档说明导入文件会复制到 profiles，原文件删除不影响副本。当前 `file://` 会保存来源，update 再读取原文件，属于关联来源而非独立副本。[本地导入说明](https://www.clashverge.dev/guide/profile.html)
  建议 `sub add ./config.yaml` / 绝对路径导入独立副本，继续保留 `file://` 表达可再次读取的关联文件，并在帮助、show、TUI 明示两者差别；统一由 add 处理，无需再增语义重叠的 import。独立副本没有远程更新，显式 update 应提示无需更新，--all 跳过并报告 skipped；移动原文件不影响 edit/use。
  来源类型与内核配置格式分开建模，不把 local/remote 当成 mihomo/sing-box 的替代分类。空白本地配置涉及草稿与可应用状态，暂缓支持；不让空文件通过现有校验。若未来产品核心扩展为通用配置管理，再一并评估 sub 是否应升级为 profile，目前不只为对齐 GUI 标题更名。

- [ ] **CV9 · P1：优先落实原 feat 的覆写，解决更新覆盖本地编辑。**
  当前 edit 的本地更改会被 update 替换。借鉴全局与单订阅扩展，先做声明式 YAML 覆写：来源内容 → 当前格式的全局覆写 → 单订阅覆写 → 最终校验 → 应用；原始下载与覆写分别保存，更新不破坏用户规则。全局覆写应按配置格式限定适用范围，避免未来把 Mihomo 字段合并进 sing-box 配置。[扩展机制文档](https://www.clashverge.dev/guide/extend.html)
  命令分组建议 `sub override`，支持显式 `--global` 或一个订阅名称，两者互斥；无目标不隐式编辑全局。编辑入口可采用 `sub override edit <name>` / `sub override edit --global`，其余查看和清除操作在实现时保持相同作用域规则。修改覆写默认只保存并校验，需要显式 use 应用。
  先明确对象、列表和删除字段的合并规则：标量覆盖、对象合并；列表默认整体替换，规则前置/后置用显式操作表达。避免无约定地按索引混合节点/规则。JavaScript 扩展及节点、策略组、规则的独立可视化编辑器暂缓，先让一套覆写与校验流程可解释、可预览。

- [ ] **CV10 · P2：补内容预览与只下载更新，明确三种配置。**
  show 继续看元数据；拟增加 `sub preview [name]` 查看缓存经覆写后的待应用内容，`--diff` 与受管内核当前应用文件比较，明确它不等于内核 API 返回的实时运行状态。没有应用文件时显示“尚未应用”，不能报成空差异。输出配置可能包含节点凭据，只在显式预览时输出，不混入普通列表或操作结果。
  update 可增加 `--no-apply`：下载、校验并保存缓存，暂不影响已应用配置；后续 `sub use <name>` 应用。默认行为继续沿用当前 update 语义。重新应用当前项也复用 use，不再增 reactivate/reload 等同义入口。与 CV6 联动显示“待应用”，与 CV9 共用最终配置生成逻辑。

- [ ] **CV11 · P2：JSON 与批量操作共用清晰的结果模型。**
  细化原 feat 的格式化输出：先做 `sub list/show --json`，字段覆盖来源类型、描述、设置、已知状态，未知值用 null；默认仍隐藏完整来源凭据与 header 值。机器字段保持稳定，时间使用明确时区的时间戳；与人类可读的“几分钟前”分开。JSON 模式不混入进度文案。
  update 已支持多个名称和 --all，不需新增 batch 子命令；后续可提供逐项结构化结果，部分失败继续处理其余目标且退出码非零。TUI 多选可在此基础上扩展，先做更新；批量移除需逐项保留当前订阅保护规则，不能为了 GUI 对齐扩大 force 的含义。

- [ ] **CV12 · P3：补输入便利性，桌面专属功能按需做。**
  可增加 `sub add --url-stdin` 读取单个 URL，避免用户必须把订阅凭据写进 shell 历史；与位置参数互斥，明确不是从 stdin 导入配置正文。桌面剪贴板按钮无需单独转化为 CLI 子命令。
  可在同一 add 解析器接收 `clash://install-config?url=...`，仅解码一次并校验内部来源，不因此注册系统 URL 协议或自动 use。主页链接先在详情中展示，仅接受 HTTP(S)，不因接收到响应头而打开浏览器。二维码分享、文件管理器跳转、拖拽排序、脚本控制台和完整图形化规则编辑暂缓，它们不是当前 CLI 的优先缺口。

后续 feat 建议顺序：CV6 状态记录、CV8/CV9 来源与覆写边界，再做 CV10；CV4/CV11 可独立推进，CV7 依赖请求策略和状态记录，CV12 按真实需求选做。原 feat 的多内核、覆写、定时更新、JSON 条目继续保留，以上对应项是细化方案，不算重复完成。


## structure

- [x] **S1：config / app 职责与文件组织。** 主配置统一为 config.Config + Load/Save；公共路径与存储从具体配置文件拆出；app 的内核、服务、订阅装配按职责拆文件。界面代码归入 internal/ui，与 CLI 共用 app 的订阅装配入口；订阅业务不再依赖交互实现。app.Load 保持纯加载，下载依赖由操作实例持有。磁盘配置与命令语义保持不变，详见 [review 第 8 节](docs/optimize-review.md#8-config--app-职责整理2026-09-12)。


以下对应项目结构扫描的八项建议，实施细节见 [架构调整记录](docs/architecture-review.md)。

- [x] **S2：统一内核路径及卸载归属。** 默认路径由 config 解析；卸载先校验全部目标，再停止服务和删除；拒绝共享根、其他内核目录和重定向的符号链接路径。
- [x] **S3：统一事务入口。** app.WithStateLock 管理锁；CLI 在命令定义处声明私有 mutation，TUI 使用 app.MutateRuntimeProfiles，不再根据命令名推断事务。
- [x] **S4：去除全局下载状态。** Downloader 按操作持有 client、镜像和通知回调；应用层选择自动代理，元数据和资源共用已选 transport。
- [x] **S5：明确订阅服务能力。** LoadProfileMetadata / LoadRuntimeProfiles 代替布尔参数；需要内核的操作在开始时返回 ErrNoRuntime。
- [x] **S6：展示与业务分离。** typed application/update 状态交给 ui 渲染；详情、编辑器进程执行移到 ui；CLI 使用命令 writer，TUI 收集警告，底层不直接写终端。
- [x] **S7：内核适配器负责代理端点解析。** Mihomo YAML 解析归 kernel，Runtime 暴露 ProxyEndpoints；env 只负责 shell 环境表示与生成。
- [x] **S8：订阅状态所有权。** Get/List/Selected 返回可独立编辑的 headers；Add/Set 保存独立副本，调用者修改不能绕过业务入口。
- [x] **S9：独立构建。** 固定可远程获取的 unisvc 提交，移除 ../unisvc replace；通用 Makefile 与可选 OrbStack 流程分离，默认 make 仅构建，verify 不依赖 Docker/OrbStack。


- [x] **S10：按实际职责精简包边界。** 内核与订阅命令分别归 cmd/kernel、cmd/sub，根 cmd 组装命令树，删除 internal/cli；ui/tui 合为 internal/ui，进度与表单共用，视图按文件拆分。display 改为 redact，httpx 改为 download；下载入口统一为 Downloader.Fetch，解压归回 kernel。更新仍复用 download，版本/缓存/重启等业务留在 app/subscription/kernel。决策依据见 [包边界复核](docs/architecture-review.md#包边界复核与精简)。

- [x] **S11：恢复订阅命令组边界（2026-09-14）。** cmd/sub 通过 NewCommand 注册，专属参数、补全及交互选择留在组内；少量适配函数保持私有，事务复用 app.WithStateLock。修正为减少包数量而取消清晰命令分组的问题，其余精简结果保留。

- [x] **S12：恢复内核命令包边界（2026-09-17）。** cmd/kernel 通过 NewCommands 向根命令注册安装、卸载、升级、列表、选择及服务控制命令；补全与交互选择归包内，internal/kernel 保留内核实现职责。代码包按职责划分，CLI 仍保持顶层内核命令。

- [x] **S13：职责、配置模型与文件命名复核（2026-09-17）。** 内核启动参数归适配器，开启代理及删除订阅装配归 app；TUI 元数据操作不再依赖内核。Transaction 改 WithStateLock，选中状态统一 Selected，订阅 Use/Update 入口收敛。下载模型改 DownloadConfig/download，移除未实现调度字段并严格拒绝未知字段；大文件及历史批次测试按职责拆分。详见架构调整记录本轮实施章节。
