
   主要编译命令

  | 命令         | 功能       | 输出文件              |
  |------------|----------|-------------------|
  | make agent | 编译主服务器程序 | nb-agent          |
  | make cli   | 编译CLI工具  | nb-cli            |
  | make all   | 编译全部程序   | nb-agent + nb-cli |

  ️ 完整功能列表

  核心编译

  - make agent - 编译主程序
  - make cli - 编译CLI工具
  - make all - 编译全部
  - make quick - 快速编译主程序（agent别名）

  跨平台编译

  - make agent-amd64 - 编译AMD64主程序
  - make agent-arm64 - 编译ARM64主程序
  - make cli-amd64 - 编译AMD64 CLI
  - make cli-arm64 - 编译ARM64 CLI
  - make build-all-arch - 编译所有架构

  开发工具

  - make dev - 开发模式编译（带竞态检测）
  - make run - 编译并运行主程序
  - make fmt - 代码格式化
  - make vet - 代码检查
  - make lint - 代码规范检查

  测试相关

  - make test - 运行测试
  - make test-race - 带竞态检测的测试
  - make test-coverage - 生成覆盖率报告

  依赖管理

  - make deps - 下载依赖
  - make deps-update - 更新依赖

  发布构建

  - make release - 发布版本（多架构+压缩）
  - make frontend - 构建前端

  系统管理

  - make install - 安装到系统（需root权限）
  - make uninstall - 卸载系统安装
  - make clean - 清理构建文件
  - make clean-all - 清理全部文件

  开发流程

  - make check - 运行格式化、检查和测试
  - make pre-commit - 预提交检查
  - make help - 显示帮助信息

   特性亮点

  版本信息注入

  Makefile自动注入完整的版本信息：
  - Git版本号、提交哈希
  - 构建时间、Go版本
  - 构建用户和主机名

  智能检测

  - 自动检测Git信息
  - 检查工具是否安装（UPX、golangci-lint等）
  - Root权限检查

  彩色输出

  - 清晰的状态信息
  - 成功/警告标识
  - 结构化的帮助信息

   使用示例

  # 编译主程序
  make agent

  # 编译CLI工具  
  make cli

  # 编译全部
  make all

  # 查看所有可用命令
  make help

  # 发布构建（多架构）
  make release

  # 开发模式编译和测试
  make dev
  make test

  # 安装到系统
  sudo make install

  所有命令都经过测试验证，可以正常使用
