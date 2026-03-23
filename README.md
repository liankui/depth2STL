# depth2STL

This project provides a server that converts 2D images into 3D relief models in STL format, suitable for 3D printing or rendering.

### 工具参数

- `image_path`：要转换的图像本地路径或 URL
- `model_width`：3D 模型的宽度（毫米，默认：50.0）
- `model_thickness`：3D 模型的最大厚度/高度（毫米，默认：5.0）
- `base_thickness`：底座厚度（毫米，默认：2.0）
- `skip_depth`：是否直接使用图像或生成深度图（默认：true）
- `invert_depth`：是否反转浮雕（明亮区域变低而不是高，默认：false）
- `detail_level`：控制处理图像的分辨率（默认：1.0）。当 detail_level = 1.0 时，图像以 320px 分辨率处理，生成的 STL 文件通常在 100MB 以内。较高的值可以提高细节质量，但会显著增加处理时间和 STL 文件大小。例如，将 detail_level 值加倍可能会使文件大小增加 4 倍或更多，请谨慎使用。

大语言模型可通过提供的 URL 访问生成的文件。

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
