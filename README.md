# depth2STL

This project provides a server that converts 2D images into 3D relief models in STL format, suitable for 3D printing or rendering.

## ref to https://github.com/Bigchx/mcp_3d_relief

### API 参数

`POST /v1/relief` 使用 `multipart/form-data` 上传，当前接口参数如下：

- `file`：必填，待转换的图片文件
- `modelWidth`：模型宽度，单位毫米，默认 `50.0`
- `modelThickness`：模型最大厚度，单位毫米，默认 `5.0`
- `baseThickness`：底座厚度，单位毫米，默认 `2.0`
- `skipConv`：是否跳过深度图转换，默认 `false`
- `invert`：是否反转浮雕方向，默认 `false`
- `detailLevel`：细节等级，默认 `2`

其中 `detailLevel` 为整数等级：
- `1`：普通精度
- `2`：推荐精度，质量和处理开销明显增加
- `3`：高精度，生成更慢且文件更大

## 服务启动

项目入口文件现在位于仓库根目录的 `main.go`，启动服务时请直接在项目根目录执行：

```bash
go run .
```

默认监听端口为 `31101`，启动后可访问：

- `http://localhost:31101/` 前端调试页面

### 外部深度图生成

为获得更高质量的深度图，您可以使用外部深度图生成服务，如 [Depth-Anything-V2](https://huggingface.co/spaces/depth-anything/Depth-Anything-V2)。该服务可以生成更准确的深度图，然后您可以将其用于本项目：

1. 访问 [https://huggingface.co/spaces/depth-anything/Depth-Anything-V2](https://huggingface.co/spaces/depth-anything/Depth-Anything-V2)
2. 上传您的图像以生成深度图
3. 下载生成的深度图
4. 将此深度图与我们的转换器一起使用，设置 `--skipConv=false`

这种方法可以提供更好的 3D 浮雕模型，特别是对于复杂图像。

## 工作原理

1. 处理图像创建深度图（较暗像素 = 较低，较亮像素 = 较高）
2. 将深度图转换为带有三角形面的 3D 网格
3. 在模型底部添加底座
4. 将模型保存为 STL 文件
