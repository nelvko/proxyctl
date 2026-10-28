# TUI 优化实施状态

更新：2026-10-28。按现有架构渐进修改，不调整内核生命周期、系统服务或 TUN 行为。

## 现状与方案

项目使用 Bubble Tea v2.1.0、Bubbles v2.2.1、Lip Gloss v2.0.6 和 Huh v2.0.3。
Dashboard 由单个模型分发页面、异步快照和操作消息；独立订阅 TUI 使用
Bubbles List 与 Huh 表单。继续沿用这两条入口和已有业务操作函数。

原有问题是颜色与中英文文案分散、焦点与鼠标命中区域缺少一致性，以及订阅
列表尺寸仅在渲染副本上更新。新增组件只负责展示、交互状态与坐标映射：列表
筛选、分页、文本输入、滚动和表单校验继续由现有依赖承担。

主题和语言共用原有 config.yaml 及状态锁。预览保留在 UI 模型中，确认时重新
读取最新配置并保存对应字段，取消时恢复原状态。语言以模型 Locale 显式传递，
避免运行时全局语言状态。新增 UI 文案先进入消息目录，再接入界面。

## 已实现

| 优先级 | 内容 | 主要模块 |
| --- | --- | --- |
| P0 | 语义 Theme、文字层级、统一边框/选中/焦点颜色、尺寸约束 | `internal/ui/theme.go`、Dashboard 与订阅主题适配 |
| 优先基础 | 中英文目录、环境与配置回退、语言实时预览/确认/取消、格式参数检查 | `internal/i18n`、`internal/config/language.go`、`dashboard_locale.go` |
| P1 | 五款内置主题、外观设置、预览与持久化、恢复默认 | `dashboard_appearance.go`、`internal/config/theme.go` |
| P1 | Dropdown、鼠标点击/悬停/滚轮、Button、共享命中区域 | `dropdown.go`、`button.go`、`components.go` |
| P2 | 部分自定义颜色覆盖、严格导入/导出、Auto/Dark/Light、颜色降级 | `theme.go`、`terminal.go`、`cmd/theme.go` |
| P2 | 可搜索选择器与可滚动通用弹窗 | `select_modal.go`、`modal.go` |
| 交互补齐 | 独立订阅 TUI 鼠标工具栏、列表点击/滚轮、详情/确认/错误/帮助弹窗 | `subscription_interactions.go`、`subscription_model.go` |
| i18n 迁移 | Dashboard、选择组件、订阅 TUI 控件/详情/结果、订阅表单 | `catalog_dashboard.go`、`catalog_subscriptions.go`、`profile_form.go` |

Dropdown 采用稳定的推开式布局；通用 Modal 使用 Lip Gloss 原生字符画布合成。
二者没有使用 ANSI 字符串裁切覆盖。没有新增第三方依赖。

独立订阅 TUI 保留 a/Enter/u/s/m/e/dd/?/q 等快捷键；Tab 可进入工具栏。
鼠标移除使用默认取消的确认弹窗，并在执行时检查目标元数据是否变化；仍遵守
当前订阅删除保护。Huh 表单的鼠标“上一步/下一步”发送原生导航按键，选择和
确认字段提供对应操作按钮，保留库的验证逻辑。文字输入继续使用键盘。

## 验证与后续计划

单元/交互回归覆盖解析回退、锁内保存、主题和语言预览取消、导入导出、禁用
选项、筛选分页命中、滚轮、焦点恢复、小窗口、Unicode 网格和失败表单草稿。
终端实测的命令、平台与限制见 [终端验证记录](tui-validation.md)。

- [ ] 逐模块迁移其余 CLI 帮助、内核选择器及诊断文案。领域错误使用 typed
  error/result 或稳定错误码在展示边界翻译，避免业务层依赖语言目录。
- [ ] Windows Terminal 实际输入、颜色与 Unicode 渲染验证；交叉编译不能代替。
- [ ] macOS 外观保存的隔离 PTY 场景；原生只读检查不能证明设置写入流程。
- [ ] 按实际重复使用需求再抽象 Tabs、Toast、Status Badge 等组件。当前使用
  现有布局、状态提示和 Button；不为候选清单增加未使用的组件。

更多说明：[主题配置](themes.md)、[i18n 评估与迁移边界](i18n.md)。
