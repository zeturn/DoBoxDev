# DoBoxDev

DoBoxDev 是一个 Docker 沙盒管理工具，使用 Go Fiber + React 构建。它面向学习和演示场景，提供容器生命周期管理、实时状态查看和基础多用户鉴权。

> 注意：本项目会访问 Docker Engine。生产环境使用前，请先完成权限隔离、资源配额、网络策略和安全审计。

## 功能特性

### 用户认证
- JWT Token 无状态认证
- 用户注册和登录
- Bcrypt 密码加密
- 多用户隔离

### 容器管理
- 创建容器：从镜像创建，支持环境变量、端口映射、资源限制
- 启动/停止：容器的启动和优雅停止
- 暂停/恢复：暂停和恢复容器运行
- 资源限额：动态调整 CPU 和内存限制
- 网络路由：灵活的端口映射配置
- 删除容器：彻底删除容器

### 实时监控
- 容器状态实时更新
- CPU 使用率监控
- 内存使用率监控
- 网络流量统计
- 容器日志查看

### 现代化界面
- React + TypeScript + Vite
- 响应式设计
- 直观的操作体验
- 实时状态刷新

## 技术栈

### 后端
- Go Fiber
- GORM
- SQLite，可按需切换到 PostgreSQL
- Docker Engine API
- JWT + Bcrypt

### 前端
- React + TypeScript
- Vite
- TailwindCSS
- React Router
- Axios

## 项目结构

```text
DoBoxDev/
├── backend/                    # Go 后端
│   ├── cmd/server/             # 服务器入口
│   ├── internal/               # 内部包
│   │   ├── config/             # 配置管理
│   │   ├── models/             # 数据模型
│   │   ├── handlers/           # HTTP 处理器
│   │   ├── middleware/         # 中间件
│   │   ├── services/           # 业务逻辑
│   │   ├── database/           # 数据库连接
│   │   └── docker/             # Docker 服务封装
│   └── pkg/utils/              # 工具函数
├── frontend/                   # React 前端
│   ├── src/
│   │   ├── components/         # React 组件
│   │   ├── pages/              # 页面组件
│   │   ├── services/           # API 服务
│   │   ├── hooks/              # 自定义 Hooks
│   │   ├── types/              # TypeScript 类型
│   │   └── utils/              # 工具函数
└── README.md
```

## 快速开始

### 前置要求

- Go 1.25+
- Node.js 24+
- Docker Engine，且 Docker 服务正在运行

### 1. 克隆项目

```bash
git clone https://github.com/zeturn/DoBoxDev.git
cd DoBoxDev
```

### 2. 启动后端

```bash
cd backend
cp .env.example .env
go mod download
go run cmd/server/main.go
```

后端将在 `http://localhost:3000` 启动。

### 3. 构建 Project Sandbox 镜像

```bash
docker build -t dobox/code-sandbox:latest ./sandbox
```

Project sandbox 使用固定的 `dobox/code-sandbox:latest` 镜像和固定的 `/workspace` 工作区。该镜像预置常用开发工具，默认以 UID/GID `1000:1000` 的 `docode` 用户运行，并让 `/workspace` 归该用户所有。通过项目文件 API 写入的文件也会保留该 UID/GID，避免 agent 写入后变成 root-owned 文件。

### 4. 启动前端

```bash
cd frontend
npm install
npm run dev
```

前端将在 `http://localhost:5173` 启动。首次使用需要注册账户。

## 验证改动

后端：

```bash
cd backend
go test ./...
go vet ./...
```

前端：

```bash
cd frontend
npm ci
npm run lint
npm run build
```

仓库也提供 GitHub Actions CI，在 PR 和 `main` 分支推送时自动运行后端测试/静态检查和前端 lint/build。

## 配置

### 后端配置 (`backend/.env`)

```bash
PORT=3000
JWT_SECRET=your-secret-key-change-this-in-production
DB_PATH=./docode.db
DOCKER_HOST=unix:///var/run/docker.sock
CORS_ORIGINS=http://localhost:5173,http://localhost:3000
```

Windows 下 Docker Host 可设置为：

```bash
DOCKER_HOST=npipe:////./pipe/docker_engine
```

### 前端配置

```bash
VITE_API_URL=http://localhost:3000/api
```

## API 端点

### 认证
- `POST /api/auth/register` - 用户注册
- `POST /api/auth/login` - 用户登录
- `GET /api/auth/me` - 获取当前用户信息

### Project Sandbox / Agent 工具
- `POST /api/projects` - 创建项目并自动创建独立 sandbox、volume、network
- `GET /api/projects/:projectId` - 获取项目和 sandbox 摘要
- `DELETE /api/projects/:projectId` - 删除项目及其 sandbox 资源
- `POST /api/projects/:projectId/agent/sessions` - 创建 agent session
- `GET /api/projects/:projectId/agent/sessions` - 列出项目 agent sessions
- `GET /api/projects/:projectId/agent/tool-calls` - 列出项目 tool call 审计记录，支持 `agent_session_id` 和 `limit`
- `POST /api/projects/:projectId/exec` - 在项目 sandbox 中执行命令
- `POST /api/projects/:projectId/files/read` - 读取 workspace 文件，返回内容受后端大小上限约束并包含 `truncated`
- `POST /api/projects/:projectId/files/write` - 写入 workspace 文件
- `POST /api/projects/:projectId/files/list` - 列出 workspace 文件
- `POST /api/projects/:projectId/files/search` - 搜索 workspace 文件
- `GET /api/projects/:projectId/git/diff` - 获取 git diff
- `POST /api/projects/:projectId/git/status` - 获取 git status
- `POST /api/projects/:projectId/git/commit` - 提交 workspace 变更
- `GET /api/projects/:projectId/artifacts/archive` - 导出 workspace tar
- `GET /api/projects/:projectId/logs` - 获取 sandbox 日志

Project tool call 审计会记录工具名、状态、输入摘要、输出摘要、exit code 和错误信息。持久化输入会保留路径、命令等上下文，但会对 `content`、`content_base64` 和 `env` 这类大字段或敏感字段做大小记录和脱敏。命令输出、搜索/列表输出和项目文件读取都受后端大小上限保护，调用方应检查响应中的 `truncated`。

### 容器管理
- `GET /api/containers` - 列出所有容器
- `POST /api/containers` - 创建新容器
- `GET /api/containers/:id` - 获取容器详情
- `POST /api/containers/:id/start` - 启动容器
- `POST /api/containers/:id/stop` - 停止容器
- `POST /api/containers/:id/pause` - 暂停容器
- `POST /api/containers/:id/unpause` - 恢复容器
- `PUT /api/containers/:id/limits` - 更新资源限制
- `DELETE /api/containers/:id` - 删除容器
- `GET /api/containers/:id/logs` - 获取容器日志
- `GET /api/containers/:id/stats` - 获取容器统计信息

## 安全注意事项

1. 生产环境务必修改 `JWT_SECRET`，并通过 Secret 管理系统注入。
2. Docker Socket 权限等同于宿主机高权限入口，请限制可访问用户和部署环境。
3. 为可创建的容器设置 CPU、内存、网络和镜像来源限制，避免资源滥用。
4. Project sandbox 默认以非 root 用户 `1000:1000` 运行，并使用项目独立 bridge 网络；`network_mode=no_internet` 会创建 internal 网络，`host`、`container:...` 等原始 Docker 网络模式不允许由调用方传入。
5. 生产环境只允许可信 CORS 源，并启用 HTTPS。
6. 漏洞报告方式见 `SECURITY.md`。

## 生产部署

### 后端构建

```bash
cd backend
go build -o server cmd/server/main.go
./server
```

### 前端构建

```bash
cd frontend
npm run build
```

构建产物在 `frontend/dist/`。可以使用 Nginx 托管前端静态文件并反向代理后端 API。

## 常见问题

### Windows 上 Docker 连接问题

设置：

```bash
DOCKER_HOST=npipe:////./pipe/docker_engine
```

### 容器无法创建

1. 确保 Docker Engine 正在运行。
2. 检查镜像名称是否正确。
3. 查看后端日志了解详细错误。

### 前端无法连接后端

1. 检查后端是否正常运行。
2. 确认 `VITE_API_URL` 配置正确。
3. 检查后端 `CORS_ORIGINS` 是否包含前端地址。

## 贡献

欢迎提交 Issue 和 Pull Request。开发流程与检查清单见 `CONTRIBUTING.md`。

## 许可证

ISC License。详见 `LICENSE`。
