# Docker 沙盒管理工具 - 快速开始指南

## 🎉 项目已成功创建！

这是一个功能完整的 Docker 容器管理系统，包含：
- ✅ 后端 Go Fiber API (已编译)
- ✅ 前端 React + TypeScript (已构建)
- ✅ 用户认证系统
- ✅ 容器全生命周期管理
- ✅ 实时监控和统计

## 🚀 立即开始

### 1. 启动后端服务器

```bash
cd backend
./server.exe  # Windows
# 或
./server     # Linux/Mac
```

后端将在 `http://localhost:3000` 启动

### 2. 启动前端开发服务器

在新的终端窗口:

```bash
cd frontend
npm run dev
```

前端将在 `http://localhost:5173` 启动

### 3. 访问应用

打开浏览器访问: **http://localhost:5173**

1. 点击"立即注册"创建账户
2. 使用用户名和密码登录
3. 开始创建和管理容器！

## 📋 功能特性

### 容器管理
- 🐳 **创建容器**: 指定镜像、环境变量、端口映射
- ▶️ **启动/停止**: 控制容器运行状态
- ⏸️ **暂停/恢复**: 暂停和恢复容器
- 📊 **实时监控**: CPU、内存使用率
- 📝 **日志查看**: 查看容器输出
- 🗑️ **删除容器**: 清理不需要的容器
- ⚙️ **资源限制**: 设置 CPU 和内存限额

### 用户功能
- 🔐 JWT 身份认证
- 👤 多用户隔离
- 🔒 密码 Bcrypt 加密

## 🎨 技术架构

### 后端
- **语言**: Go 1.21+
- **框架**: Fiber (高性能 Web 框架)
- **数据库**: SQLite + GORM
- **认证**: JWT Token
- **容器**: Docker Engine API

### 前端
- **框架**: React 18 + TypeScript
- **构建**: Vite
- **UI**: Ant Design + TailwindCSS
- **路由**: React Router
- **状态**: Context API + Hooks

## 📁 项目结构

```
docode/
├── backend/               # Go 后端
│   ├── cmd/server/       # 服务器入口
│   ├── internal/         # 业务逻辑
│   │   ├── config/      # 配置
│   │   ├── models/      # 数据模型
│   │   ├── handlers/    # API 处理器
│   │   ├── middleware/  # 中间件
│   │   ├── database/    # 数据库
│   │   └── docker/      # Docker 服务
│   └── server.exe       # 编译好的可执行文件
│
├── frontend/             # React 前端
│   ├── src/
│   │   ├── components/  # UI 组件
│   │   ├── pages/       # 页面
│   │   ├── services/    # API 服务
│   │   ├── hooks/       # React Hooks
│   │   └── types/       # TypeScript 类型
│   └── dist/            # 构建产物
│
└── README.md            # 详细文档
```

## ⚙️ 配置说明

### 后端环境变量 (backend/.env)

```bash
PORT=3000                              # 服务器端口
JWT_SECRET=your-secret-key             # JWT 密钥
DB_PATH=./docode.db                   # 数据库路径
CORS_ORIGINS=http://localhost:5173    # CORS 允许源
```

### 前端环境变量 (frontend/.env)

```bash
VITE_API_URL=http://localhost:3000/api
```

## 📝 API 端点

### 认证 API
```
POST /api/auth/register  - 注册新用户
POST /api/auth/login     - 用户登录
GET  /api/auth/me        - 获取当前用户
```

### 容器 API
```
GET    /api/containers           - 列出容器
POST   /api/containers           - 创建容器
GET    /api/containers/:id       - 获取容器详情
POST   /api/containers/:id/start - 启动容器
POST   /api/containers/:id/stop  - 停止容器
POST   /api/containers/:id/pause - 暂停容器
POST   /api/containers/:id/unpause - 恢复容器
PUT    /api/containers/:id/limits  - 更新资源限制
DELETE /api/containers/:id       - 删除容器
GET    /api/containers/:id/logs  - 获取日志
GET    /api/containers/:id/stats - 获取统计
```

## 🔧 开发命令

### 后端
```bash
cd backend
go run cmd/server/main.go  # 开发模式
go build -o server cmd/server/main.go  # 构建
```

### 前端
```bash
cd frontend
npm run dev    # 开发服务器
npm run build  # 生产构建
npm run preview # 预览构建
```

## 📦 生产部署

### 1. 后端部署
```bash
cd backend
go build -o server cmd/server/main.go
./server
```

### 2. 前端部署
```bash
cd frontend
npm run build
# 将 dist/ 目录部署到 Web 服务器
```

推荐使用 Nginx 反向代理。

## 💡 注意事项

### Docker 集成
当前已接入真实本机 Docker（通过 docker CLI）。确保 Docker Desktop/Engine 已启动并且 `docker` 在 PATH 中。

### 安全建议
1. ✅ 修改默认 JWT_SECRET
2. ✅ 生产环境使用 HTTPS
3. ✅ 设置适当的 CORS 策略
4. ✅ 限制容器资源配额
5. ✅ 定期更新依赖包

## 🐛 常见问题

**Q: 前端无法连接后端？**
- 检查后端是否在运行
- 确认 VITE_API_URL 配置正确
- 检查 CORS 设置

**Q: 容器操作失败？**
- 确认 Docker Desktop/Engine 已启动
- 在终端执行 `docker ps` 确认可连通
- 检查当前用户是否有 Docker 权限

**Q: 登录后立即跳转到登录页？**
- 清除浏览器 localStorage
- 检查 JWT_SECRET 配置

## 📚 更多信息

详细文档请查看项目根目录的 `README.md`

## 🎯 下一步

1. **测试应用**: 注册账户并尝试各项功能
2. **自定义配置**: 修改 .env 文件
3. **增强 Docker 功能**: 增加更多镜像与网络策略支持
4. **添加功能**: 根据需求扩展功能
5. **部署上线**: 部署到生产环境

---

**祝您使用愉快！** 🎉

如有问题，请查看详细文档或提交 Issue。

