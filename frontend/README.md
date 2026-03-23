# frontend

一个由 Gin 直接托管的静态单页前端，用于调试 `depth2STL` 的 API。

## 使用

1. 在项目根目录启动后端服务，默认监听 `31101`：

```bash
go run .
```
2. 直接打开 `http://localhost:31101/` （或 `http://localhost:31101/frontend/` ）。
3. 前端会先读取 `/config.js`，再决定 API 地址。

当前实现中，前端首页挂载在 `/`，静态资源目录挂载在 `/frontend`。

## 配置

- `PORT`
  默认值：`31101`
  用于控制 Gin Web 服务监听端口。
- `API_BASE_URL`
  默认值：根据当前请求主机和 `PORT` 生成，例如 `http://localhost:31101`
  用于覆盖前端实际请求的 API 地址。

示例：

```bash
PORT=32000 API_BASE_URL=http://localhost:32000 go run .
```

## 支持接口

- `POST /v1/relief`
- `GET /v1/relief/:jobId`
- `GET /v1/relief/queue/status`
- `GET /v1/relief/download/image/:jobId`
- `GET /v1/relief/download/stl/:jobId`
