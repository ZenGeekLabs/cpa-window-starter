# CPA Window Starter · 账号定时请求

**按你的时间安排，让 CPA 自动向选中的账号发送一次简短请求。**

简体中文 · [English](README.md) · [下载版本](https://github.com/ZenGeekLabs/cpa-window-starter/releases) · [反馈问题](https://github.com/ZenGeekLabs/cpa-window-starter/issues)

这是一个 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 原生插件，支持 CPA 中已有的 **GPT / Codex** 和 **Gemini / Antigravity（反重力）** 账号。选好账号、模型、时区和每天的时间点，插件就会按计划逐个发送简短请求。

> 请求成功不等于新额度窗口已经启动。额度与重置规则由上游服务决定，定时请求会正常消耗账号额度。

## 功能

- **两套独立计划**：Codex 和 Antigravity 分别设置账号、模型、时区、时间点和自动触发开关。
- **自定义每日时间**：最多 24 个 `HH:MM` 时间点，支持搜索全球 IANA 时区和夏令时。默认上海时区的 `07:00 / 13:00 / 19:00` 可以自由修改。
- **直接使用 CPA 账号**：通过原生接口指定账号发送请求；已禁用或已删除的账号会跳过。
- **从 CPA 选择模型**：显示所选账号共同支持的模型；Antigravity 计划显示 Gemini 模型。
- **按套餐管理账号**：根据 CPA 明确提供的信息分为 Plus、Team / Business、Pro 等；没有套餐信息时显示“未知”。
- **查看每个账号的结果**：记录触发时间和状态；有可靠上游响应头时显示 Codex 五小时窗口的实际重置时间。单个账号失败不影响其余账号。
- **跟随 CPA 界面**：支持中英文和浅色 / 深色，直接跟随 CPA 的语言与外观设置。
- **后台执行**：保存后可以关闭网页，CPA 进程需持续运行。停机期间错过的时间点不会补发。

## 安装

### 通过插件商店

官方商店收录正在提交，默认商店暂不保证能够搜到。收录完成前，可以在 CPA 中添加本仓库的插件源：

```text
https://raw.githubusercontent.com/ZenGeekLabs/cpa-window-starter/main/registry.json
```

对应配置如下。请合并到已有的 `plugins` 配置中，保留其他插件和插件源：

```yaml
plugins:
  enabled: true
  dir: plugins
  store-sources:
    - https://raw.githubusercontent.com/ZenGeekLabs/cpa-window-starter/main/registry.json
```

刷新商店后，找到 **CPA Window Starter · 账号定时请求**，安装并启用。官方收录合并后，也可以直接从默认商店安装。

### 手动安装

从 [最新 Release](https://github.com/ZenGeekLabs/cpa-window-starter/releases/latest) 下载对应 ZIP 和 `checksums.txt`：

| CPA 所在机器 | 安装包后缀 | 包内文件 |
| --- | --- | --- |
| macOS，Apple Silicon | `darwin_arm64.zip` | `cpa-window-starter.dylib` |
| Linux，x86_64 / glibc | `linux_amd64.zip` | `cpa-window-starter.so` |

按 **运行 CPA 的机器** 选择，与浏览器所在设备无关。本版未提供 Windows、Intel Mac、Linux ARM64 或 musl / Alpine 构建。

1. 用 macOS 的 `shasum -a 256 <安装包>` 或 Linux 的 `sha256sum <安装包>`，核对 `checksums.txt` 中对应的 SHA-256。
2. 解压动态库到 CPA 配置的插件目录。更新已有插件时，先备份原文件。
3. 在 CPA 中启用插件功能和本插件；完整配置示例见 [config.example.yaml](config.example.yaml)。
4. 在插件管理页加载或重新加载插件，再打开其菜单入口。如果你的 CPA 版本需要重启，请安排在没有请求运行时操作。

需要支持原生插件以及 `host.auth.list`、`host.model.execute` 回调的 CPA 版本。集成工作已在 macOS CPA 8.0.10 和 Linux CPA 8.0.13 上进行；不代表所有旧版本或平台均已验证。

## 开始使用

1. 在已登录的 CPA 管理页面打开插件。
2. 切换到 **GPT / Codex** 或 **Gemini / Antigravity**。
3. 勾选账号，从列表选择请求模型。
4. 设置时区和每天的时间点，打开“自动触发”，点击 **保存计划**。
5. 如需立即验证，点击 **立即触发一次**，再查看各账号的执行结果。这会发送真实请求。

新安装默认没有选中账号，因此不会自动发出请求；Antigravity 的自动触发也默认关闭。两类账号的计划分别保存。

插件复用同源 CPA 页面兼容的“记住登录”会话，不单独要求再填管理密钥。若没有可复用的会话，页面会提供 CPA 原生插件设置入口。账号 Token 由 CPA 管理，插件通过账号 ID 和宿主回调发起请求。

## 关于额度窗口

每次请求要求模型简短回复 `OK`，仍属于正常模型请求，会消耗相应的 Token 和请求额度。

对于“首次使用后开始计时”的上游规则，定时请求可能让空闲账号进入新窗口；它无法改变已经开始的窗口、补充用完的额度或绕过周限额。**07:00 发出请求，并不保证 12:00 重置。** 只有收到可识别的上游五小时窗口响应头时，插件才显示 Codex 重置时间，否则保持未知。

Gemini / Antigravity 直接使用 CPA 现有的反重力认证文件发送请求。请求成功不能用于推断其额度恢复机制，本插件不承诺 Gemini 采用五小时周期。

## 保存与运行

计划保存在 CPA 的插件配置中。执行记录默认保存到 `plugins/state/cpa-window-starter.json`，可通过 `state_file` 修改；Antigravity 使用追加 `.antigravity` 后缀的独立文件。请保持配置和记录目录可写，并在重启或容器重建后保留这些文件。

自动任务会在请求发出前记录本次执行，避免重启后对同一时间点重复发送。错过或中断的任务不会自动补发；“立即触发一次”会主动发起一次新请求。

插件以原生动态库形式运行在 CPA 进程中，无需单独的守护进程、浏览器扩展或另一套账号登录。

## 开发与本地演示

需要 Go 1.24 或更新版本、C 编译器、支持 `node:test` 的 Node.js，以及用于打包 / ABI 检查的 Python 3。macOS 还需要 Xcode Command Line Tools。

```sh
make test
make build
python3 tests/abi_smoke.py dist/darwin_arm64/cpa-window-starter.dylib --work-dir .build/abi
```

在 Linux 上进行 ABI 检查时换成 `.so` 路径。原生 Linux 构建使用 `sh scripts/build.sh linux amd64`；从 macOS 交叉编译可使用 `ZIG=/path/to/zig sh scripts/build.sh linux amd64`，目标为 GNU / glibc。

```sh
make demo
```

打开 <http://127.0.0.1:18418/v0/resource/plugins/cpa-window-starter/status?demo=1>。演示使用虚拟账号、模型和响应，不调用真实上游，也不消耗账号额度；其中的重置时间为模拟数据。

构建两个平台后，运行 `python3 scripts/package.py`，在 `release/` 获得安装包和 `checksums.txt`。验证范围见 [发布验证说明](docs/release-verification.md)，版本变化见 [CHANGELOG](CHANGELOG.md)。

## 开源许可与图标说明

项目代码采用 [MIT](LICENSE) 许可，© 2026 ZenGeekLabs / 禅极科技。

OpenAI、Gemini 名称和图标用于识别相关服务，权利归各自所有者，不属于本项目 MIT 授权范围，也不表示官方认可或合作关系。来源说明：[OpenAI](docs/openai-asset-source.md)、[Gemini](docs/gemini-asset-source.md)。
