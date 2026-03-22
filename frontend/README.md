# frontend

一个由 Gin 直接托管的静态单页前端，用于调试 `depth2STL` 的 API。

## 使用

1. 启动后端服务（默认监听 `31101`）。
2. 直接打开 `http://localhost:31101/`（或 `http://localhost:31101/frontend/`）。
3. 前端固定通过同源路径 `/v1` 调用后端接口。

## 支持接口

- `POST /v1/relief`
- `GET /v1/relief/:jobId`
- `GET /v1/relief/queue/status`
- `GET /v1/relief/download/image/:jobId`
- `GET /v1/relief/download/stl/:jobId`
