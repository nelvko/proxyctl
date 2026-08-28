请为当前 Go CLI 项目实现 Shell Integration。项目 CLI 名称为 **`proxyctl`**。

核心目标：

> `proxyctl on` 需要能够修改**当前终端 Shell 的环境变量**；其他子命令保持普通 CLI 执行方式。

---

# 一、背景

当前项目是一个 Go 实现的 CLI，命令类似：

```text
proxyctl on
proxyctl off
proxyctl status
proxyctl sub ...
proxyctl kernel ...
```

其中 `proxyctl on` 除了执行 Go CLI 本身的业务逻辑之外，还需要向**当前终端 Shell**注入代理环境变量。

由于 Go CLI 作为子进程运行时，无法修改父 Shell 的环境变量，因此不能单纯依赖：

```go
os.Setenv(...)
```

实现。

采用以下方案：

```text
.bashrc
    │
    ▼
eval "$(proxyctl init bash)"
    │
    ▼
注册 proxyctl() Shell function
    │
    ├── proxyctl on
    │       │
    │       ▼
    │   command proxyctl on
    │       │
    │       ▼
    │   输出 export 命令
    │       │
    │       ▼
    │      eval
    │       │
    │       ▼
    │   修改当前 Shell 环境
    │
    └── 其他命令
            │
            ▼
       command proxyctl "$@"
            │
            ▼
          Go CLI
```

---

# 二、实现 `proxyctl init`

增加：

```bash
proxyctl init bash
```

该命令的作用不是初始化代理，而是输出 Bash Shell Integration 代码。

用户可以在 `.bashrc` 中：

```bash
eval "$(proxyctl init bash)"
```

`proxyctl init bash` 的 stdout 必须是合法的 Bash 代码，例如：

```bash
proxyctl() {
    case "$1" in
        on)
            eval "$(command proxyctl "$@")"
            ;;
        *)
            command proxyctl "$@"
            ;;
    esac
}
```

重点：

### 必须使用 `command proxyctl`

不能写：

```bash
proxyctl() {
    proxyctl "$@"
}
```

否则会递归调用当前 Shell function。

正确：

```bash
command proxyctl "$@"
```

这样可以绕过 Shell function，直接执行 PATH 中真正的 `proxyctl` 二进制。

---

# 三、`proxyctl on`

执行：

```bash
proxyctl on
```

实际上应该变成：

```bash
eval "$(command proxyctl on)"
```

真实的 Go CLI 负责：

1. 执行现有 `on` 业务逻辑。
2. 启动/检查代理服务。
3. 根据当前配置计算代理地址和端口。
4. 生成需要注入当前 Shell 的环境变量。
5. 业务成功后，通过 stdout 输出 Shell 命令。

例如：

```bash
export HTTP_PROXY=http://127.0.0.1:7890
export HTTPS_PROXY=http://127.0.0.1:7890
export ALL_PROXY=socks5://127.0.0.1:7890
```

具体变量和代理地址必须根据项目当前已有实现确定，**不要硬编码 `7890` 等值**。

---

# 四、stdout / stderr 必须严格区分

这是实现中的关键要求。

当执行：

```bash
eval "$(command proxyctl on)"
```

stdout 会被 Shell 捕获并执行。

因此：

### stdout

只能输出需要执行的 Shell 命令：

```bash
export HTTP_PROXY=...
export HTTPS_PROXY=...
export ALL_PROXY=...
```

### stderr

用于普通用户提示、日志、错误信息：

```text
Proxy enabled
Proxy service started
```

例如：

```go
fmt.Fprintln(os.Stderr, "Proxy enabled")
fmt.Println(`export HTTP_PROXY=http://127.0.0.1:7890`)
```

不能：

```go
fmt.Println("Proxy enabled")
fmt.Println(`export HTTP_PROXY=...`)
```

否则：

```bash
eval "$(command proxyctl on)"
```

会尝试把：

```text
Proxy enabled
```

当作 Shell 命令执行。

---

# 五、失败处理

必须保证：

```text
proxyctl on
    │
    ├── 业务失败
    │      ↓
    │   stderr 输出错误
    │      ↓
    │   stdout 不输出 export
    │
    └── 业务成功
           ↓
        stdout 输出 export
```

也就是说，如果代理服务启动失败：

```bash
proxyctl on
```

不能修改当前 Shell 的代理环境变量。

例如不能出现：

```text
启动代理失败

但同时：
export HTTP_PROXY=...
```

否则 `eval` 后会导致 Shell 环境处于错误状态。

同时必须正确返回非 0 exit code。

---

# 六、`proxyctl off`

检查项目当前 `off` 命令的语义。

如果 `off` 的职责包括关闭当前终端代理环境，则同样使用 Shell Integration：

```bash
proxyctl off
```

实际：

```bash
eval "$(command proxyctl off)"
```

Go stdout 输出：

```bash
unset HTTP_PROXY
unset HTTPS_PROXY
unset ALL_PROXY
unset http_proxy
unset https_proxy
unset all_proxy
```

如果项目当前 `off` 的语义并不需要修改当前 Shell 环境，则不要擅自改变现有行为。

---

# 七、普通命令

除了需要修改当前 Shell 状态的命令之外，其他命令必须直接执行真实的 Go CLI。

例如：

```bash
proxyctl status
proxyctl sub list
proxyctl kernel list
proxyctl version
```

应该等价于：

```bash
command proxyctl status
command proxyctl sub list
command proxyctl kernel list
command proxyctl version
```

**不能统一使用 eval。**

错误：

```bash
proxyctl() {
    eval "$(command proxyctl "$@")"
}
```

因为普通命令输出不是 Shell code，可能导致输出被错误地当成 Shell 命令执行。

---

# 八、Shell function 设计

第一阶段至少支持 Bash：

```bash
proxyctl init bash
```

生成：

```bash
proxyctl() {
    case "$1" in
        on)
            eval "$(command proxyctl "$@")"
            ;;
        *)
            command proxyctl "$@"
            ;;
    esac
}
```

如果当前项目已经存在 Shell 抽象，并且支持 Zsh/Fish，可以考虑同时支持：

```bash
proxyctl init zsh
proxyctl init fish
```

但不要为了实现 Bash 而大幅重构现有架构。

---

# 九、未来扩展

不要把设计写死成只有 `on`。

建议抽象出：

```text
需要修改当前 Shell 状态的命令
```

例如未来可能有：

```text
proxyctl on
proxyctl off
proxyctl env
proxyctl use
```

Shell function 可以类似：

```bash
proxyctl() {
    case "$1" in
        on|off|env|use)
            eval "$(command proxyctl "$@")"
            ;;
        *)
            command proxyctl "$@"
            ;;
    esac
}
```

具体哪些命令进入该列表，请结合项目当前 command 设计决定。

---

# 十、不要硬编码 proxyctl 路径

生成的 Shell function 中使用：

```bash
command proxyctl
```

不要生成：

```bash
/usr/local/bin/proxyctl
```

也不要把当前执行路径写死。

这样用户修改 PATH 或通过不同方式安装 `proxyctl` 后，Shell Integration 仍然可以工作。

---

# 十一、与现有 Shell 环境实现整合

在开始修改之前，请先检查项目：

1. CLI command 注册入口。
2. 当前 `on` command 实现。
3. 当前 `off` command 实现。
4. 当前代理环境变量实现。
5. Shell 相关代码。
6. `.bashrc` / `.zshrc` 修改逻辑。
7. 是否已经存在 `apply_rc`、Shell hook、proxy.env 等机制。
8. 当前 CLI 使用的框架。

如果已有 Shell integration 或环境变量相关代码，请优先复用和重构，不要重复实现。

尤其注意不要同时维护两套逻辑：

```text
旧 proxy.env 逻辑
+
新的 Shell Integration 逻辑
```

应该尽可能形成统一的数据源。

---

# 十二、`.bashrc` 集成

最终用户应该可以：

```bash
eval "$(proxyctl init bash)"
```

如果项目安装逻辑会自动修改 `.bashrc`，需要：

1. 不破坏用户已有内容。
2. 避免重复添加。
3. 最好能够检测已有：

   ```bash
   eval "$(proxyctl init bash)"
   ```
4. 不要每次执行安装都重复追加。

如果当前项目已经有 rc 文件处理逻辑，请复用现有实现。

---

# 十三、环境变量

请先检查项目现有的代理环境变量定义。

可能涉及：

```text
HTTP_PROXY
HTTPS_PROXY
ALL_PROXY
http_proxy
https_proxy
all_proxy
NO_PROXY
no_proxy
```

不要简单假设必须全部设置。

应以项目当前已有的代理设计为准。

代理地址也必须从当前配置动态获取，例如：

```text
http://127.0.0.1:<当前 HTTP proxy 端口>
socks5://127.0.0.1:<当前 SOCKS proxy 端口>
```

不要硬编码端口。

---

# 十四、参数透传

Shell function 必须正确处理参数：

```bash
proxyctl on
proxyctl on xxx
proxyctl status
proxyctl sub add "$URL"
```

必须使用：

```bash
"$@"
```

不能使用容易导致参数重新拆分的：

```bash
$@
```

---

# 十五、测试

请增加必要测试。

至少验证以下场景。

### 1. init bash

执行：

```bash
eval "$(proxyctl init bash)"
```

然后：

```bash
type proxyctl
```

应该得到：

```text
proxyctl is a function
```

### 2. 普通命令

```bash
proxyctl status
```

正常执行 Go CLI。

不能进入 eval。

### 3. on

```bash
proxyctl on
```

成功后：

```bash
env | grep -i proxy
```

能够看到正确的代理环境变量。

### 4. off

如果 `off` 负责清除当前 Shell 环境：

```bash
proxyctl off
```

之后：

```bash
env | grep -i proxy
```

不再存在对应代理变量。

### 5. on 失败

模拟代理服务启动失败。

验证：

```text
exit code != 0
```

并且：

```text
当前 Shell 环境不会被修改
```

### 6. 参数透传

验证：

```bash
proxyctl sub ...
```

参数完整传递到真实 Go CLI。

### 7. 递归问题

确认：

```bash
command proxyctl
```

可以绕过 Shell function。

---

# 十六、代码质量要求

请遵循当前项目已有代码风格和架构。

不要为了这个功能引入不必要的第三方依赖。

Shell 代码尽量保持简单、可读。

Go 代码中将：

```text
业务逻辑
```

与：

```text
Shell 输出逻辑
```

适当分离。

例如可以形成类似：

```go
type ShellEnv struct {
    HTTPProxy  string
    HTTPSProxy string
    AllProxy   string
}

func (e ShellEnv) ExportBash() string {
    ...
}
```

但具体结构请根据当前项目实际代码决定，不要求机械采用。

---

# 十七、实现流程

请严格按照以下流程执行：

### 第一步：分析

先分析当前项目：

```text
目录结构
CLI framework
command 注册方式
on/off 实现
Shell 相关代码
代理环境变量实现
配置结构
```

### 第二步：给出方案

在修改代码之前，先简要说明：

```text
1. 当前实现存在什么问题
2. Shell Integration 准备放在哪里
3. proxyctl init bash 如何实现
4. proxyctl on 如何输出 Shell code
5. stdout/stderr 如何处理
6. 哪些文件需要修改
7. 如何测试
```

### 第三步：实现

按照方案修改代码。

### 第四步：测试

至少执行：

```bash
go test ./...
```

以及项目已有的 lint / build / test 命令。

如果环境允许，再实际执行：

```bash
eval "$(proxyctl init bash)"

type proxyctl

proxyctl status

proxyctl on

env | grep -i proxy

proxyctl off

env | grep -i proxy
```

### 第五步：总结

最终说明：

```text
修改了哪些文件
每个文件修改的作用
Shell Integration 的调用流程
on/off 如何修改当前 Shell 环境
测试结果
是否存在遗留问题
```

---

# 最终验收标准

最终用户体验应该是：

```bash
$ eval "$(proxyctl init bash)"

$ proxyctl status
Proxy: stopped

$ proxyctl on
Proxy enabled

$ echo "$HTTP_PROXY"
http://127.0.0.1:7890

$ proxyctl status
Proxy: running

$ proxyctl off
Proxy disabled

$ echo "$HTTP_PROXY"

$ proxyctl sub list
...
```

其中：

```text
proxyctl on
    ↓
Shell function
    ↓
command proxyctl on
    ↓
Go CLI
    ↓
stdout 输出 export
    ↓
eval
    ↓
当前 Shell 环境发生变化
```

而：

```text
proxyctl status
    ↓
Shell function
    ↓
command proxyctl status
    ↓
Go CLI
    ↓
正常输出
```

**核心原则：只有需要修改当前 Shell 状态的命令才通过 `eval`，普通 `proxyctl` 子命令永远直接执行。**

