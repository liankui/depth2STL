# frontend

一个静态单页前端，用于调试 `depth2STL` 的 API。

## 使用

1. 启动后端服务（默认监听 `31101`）。
2. 在 `frontend` 目录启动静态服务，例如：
   ```bash
   python3 -m http.server 5173
   ```
3. 打开 `http://localhost:5173`。
4. 页面会读取 `config.js` 里的 `apiBaseUrl`（默认是 `http://localhost:31101/v1`）。

## 支持接口

- `POST /v1/relief`
- `GET /v1/relief/:jobId`
- `GET /v1/relief/queue/status`
- `GET /v1/relief/download/image/:jobId`
- `GET /v1/relief/download/stl/:jobId`
- `DELETE /v1/relief/queue/:jobId`
