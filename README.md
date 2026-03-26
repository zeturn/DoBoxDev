# Docker 沙盒管理工具

一个功能完整的 Docker 容器管理系统，使用 Go Fiber + React 构建。支持容器的全生命周期管理、实时监控和多用户鉴权。

## ✨ 功能特性

### 🔐 用户认证
- JWT Token 无状态认证
- 用户注册和登录
- Bcrypt 密码加密
- 多用户隔离

### 🐳 容器管理
- **创建容器**: 从镜像创建，支持环境变量、端口映射、资源限制
- **启动/停止**: 容器的启动和优雅停止
- **暂停/恢复**: 暂停和恢复容器运行
- **资源限额**: 动态调整 CPU 和内存限制
- **网络路由**: 灵活的端口映射配置
- **删除容器**: 彻底删除容器

### 📊 实时监控
- 容器状态实时更新
- CPU 使用率监控
- 内存使用率监控
- 网络流量统计
- 容器日志查看

### 🎨 现代化界面
- React + TypeScript + Ant Design
- 响应式设计
- 直观的操作体验
- 实时状态刷新

## 🏗️ 技术栈

### 后端
- **框架**: Go Fiber (高性能 Web 框架)
- **ORM**: GORM
- **数据库**: SQLite (可轻松切换到 PostgreSQL)
- **容器管理**: Docker Engine API
- **认证**: JWT + Bcrypt

### 前端
- **框架**: React 18 + TypeScript
- **构建工具**: Vite
- **UI 库**: Ant Design
- **样式**: TailwindCSS
- **路由**: React Router
- **HTTP 客户端**: Axios

## 📁 项目结构

```
docode/
├── backend/                    # Go 后端
│   ├── cmd/server/            # 服务器入口
│   ├── internal/              # 内部包
│   │   ├── config/           # 配置管理
│   │   ├── models/           # 数据模型
│   │   ├── handlers/         # HTTP 处理器
│   │   ├── middleware/       # 中间件
│   │   ├── services/         # 业务逻辑
│   │   ├── database/         # 数据库连接
│   │   └── docker/           # Docker 服务封装
│   └── pkg/utils/            # 工具函数
├── frontend/                  # React 前端
│   ├── src/
│   │   ├── components/       # React 组件
│   │   ├── pages/            # 页面组件
│   │   ├── services/         # API 服务
│   │   ├── hooks/            # 自定义 Hooks
│   │   ├── types/            # TypeScript 类型
│   │   └── utils/            # 工具函数
│   └── public/               # 静态资源
└── README.md                  # 项目文档
```

## 🚀 快速开始

### 前置要求

- Go 1.21+
- Node.js 18+
- Docker Engine (必须运行)

### 1. 克隆项目

```bash
git clone <repository-url>
cd docode
```

### 2. 启动后端

```bash
cd backend

# 复制环境变量配置
cp .env.example .env

# 安装依赖
go mod download

# 运行服务器
go run cmd/server/main.go
```

后端将在 `http://localhost:3000` 启动

### 3. 启动前端

```bash
cd frontend

# 安装依赖
npm install

# 运行开发服务器
npm run dev
```

前端将在 `http://localhost:5173` 启动

### 4. 访问应用

打开浏览器访问: `http://localhost:5173`

首次使用需要注册账户。

## ⚙️ 配置

### 后端配置 (backend/.env)

```bash
# 服务器端口
PORT=3000

# JWT 密钥 (生产环境请修改!)
JWT_SECRET=your-secret-key-change-this-in-production

# 数据库路径
DB_PATH=./docode.db

# Docker 配置
# Linux/Mac: unix:///var/run/docker.sock
# Windows: npipe:////./pipe/docker_engine
DOCKER_HOST=

# CORS 允许的源
CORS_ORIGINS=http://localhost:5173,http://localhost:3000
```

### 前端配置 (frontend/.env)

```bash
# API 地址
VITE_API_URL=http://localhost:3000/api
```

## 🔌 API 端点

### 认证
- `POST /api/auth/register` - 用户注册
- `POST /api/auth/login` - 用户登录
- `GET /api/auth/me` - 获取当前用户信息

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

## 🔒 安全注意事项

1. **JWT 密钥**: 生产环境务必修改 `JWT_SECRET`
2. **Docker Socket**: 确保 Docker socket 访问权限安全
3. **资源限制**: 建议设置容器资源配额防止滥用
4. **CORS**: 生产环境配置正确的 CORS 源
5. **HTTPS**: 生产环境建议使用 HTTPS

## 📦 生产部署

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
# 构建产物在 dist/ 目录
```

可以使用 Nginx 托管前端静态文件并反向代理后端 API。

## 🐛 常见问题

### Windows 上 Docker 连接问题

Windows 下需要设置:
```bash
DOCKER_HOST=npipe:////./pipe/docker_engine
```

### 容器无法创建

1. 确保 Docker Engine 正在运行
2. 检查镜像名称是否正确
3. 查看后端日志了解详细错误

### 前端无法连接后端

1. 检查后端是否正常运行
2. 确认 `.env` 中的 `VITE_API_URL` 配置正确
3. 检查 CORS 配置

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License

## 👨‍💻 作者

使用 GitHub Copilot 生成

---

**注意**: 本项目仅用于学习和演示目的，生产环境使用前请进行充分的安全审计和测试。

