# 新蜂资产管理平台 — 主机采集与执行 Agent

部署在被管理主机上的 Agent，为平台主机接入、状态采集与任务执行提供基础。仓库保留源自 AcePanel 的主机管理、Web 管理和 CLI 能力；平台接入相关代码位于 `internal/agent/`。

仓库：[coder-lulu/nb-agent](https://github.com/coder-lulu/nb-agent) · [平台工作区](https://github.com/coder-lulu/newbee)

## 获取代码

推荐通过完整工作区开发，保留兄弟模块目录及本地 `replace` 依赖。以下命令使用 Bash；Go 工作区要求 Go 1.25.1 或更高版本。

```bash
git clone --recurse-submodules https://github.com/coder-lulu/newbee.git
cd newbee/nb-agent
```

已有工作区执行 `git submodule update --init --recursive`。单独克隆模块时，需要自行补齐 `go.mod` 中的本地依赖路径。

## 目录导航

| 目录 | 用途 |
| --- | --- |
| `cmd/nb/` | Agent 主程序 |
| `cmd/cli/` | 管理命令行 |
| `internal/agent/` | 连接与执行组件 |
| `internal/bootstrap/`、`internal/service/` | 初始化与主机服务 |
| `pkg/` | 通用工具 |
| `web/` | Web 管理界面源码 |
| `docs/` | Agent 接口和中心接入设计 |

## 构建与配置

Agent 面向 Linux 主机，主程序启动时要求 root 权限。Makefile 使用 Bash，默认构建 `linux/amd64`，可通过 `GOARCH=arm64` 指定 ARM64。此模块未加入平台 `go.work`，因此在工作区内执行命令时设置 `GOWORK=off`。

```bash
GOWORK=off make agent
GOWORK=off make cli
GOWORK=off go test ./...
```

首次配置时将 `config.example.yml` 复制为 `config.yml`，设置随机的 32 字符 `app.key`、工作目录、HTTP 端口和 TLS 参数。配置加载优先读取 `/usr/local/etc/panel/config.yml`，不存在时读取当前目录的 `config.yml`。现有配置不要覆盖，真实密钥不要提交。

配置和主机运行环境准备完成后，在 Linux 上运行 `sudo ./nb-agent`。Agent 会操作主机服务与文件，请在独立测试主机验证接入流程后部署。仓库入口不提供尚未发布的在线安装脚本。

## 接入文档

- [Agent 接口规范](docs/agent-interface-spec.md)
- [中心端设计](docs/center-design.md)
- [代理端设计](docs/proxy-design.md)
- [构建命令说明](docs/make-readme.md)

## 许可证与来源

本仓库采用 [BSD-3-Clause](LICENSE)。上游为 [AcePanel（原 tnborg/panel）](https://github.com/tnborg/panel)，保留 AcePanel contributors 版权。第三方依赖遵循各自许可证，保留原有版权与许可声明。
