# 文档图片水印工具

为 Office 文档与常见图片批量添加图片级水印（在嵌入的图片上绘制水印），提供 **Windows 原生 GUI**（Go）与仓库内保留的 **Python 参考实现**。

更详细的界面操作与参数说明见：**[使用说明.html](./使用说明.html)**。

## 仓库结构

| 目录 | 说明 |
|------|------|
| `go-app/` | Windows GUI 程序源码，可构建为单文件 `exe`，面向最终用户分发 |
| `src/` | Python 参考实现，用于对照行为与算法细节 |

## Go 版支持格式

- **演示文稿**：`.pptx`
- **Word**：`.docx`
- **图片**：`.jpg`、`.jpeg`、`.png`、`.bmp`、`.gif`、`.tif`、`.tiff`

不支持 PDF。旧版 `.ppt` / `.doc` 需先另存为 `pptx` / `docx`。

## 输出规则

- 结果文件写在**原文件同一目录**。
- **不覆盖原文件**：默认文件名为 `原名_watermarked` + 原扩展名；若已存在则自动追加 `_2`、`_3` …
- 可选「删除原文件只保留带水印文件」（以界面勾选为准）。

## Go 版默认参数（与 `internal/config` 一致）

| 项 | 默认值 |
|----|--------|
| 水印文字 | 超卓航科广州研究院 |
| 字体 | 微软雅黑 |
| 字号 (pt) | 12 |
| 颜色 | `#000000`（黑） |
| 透明度 | 6（0–100） |
| 旋转角度 | 0 |
| 位置模式 | `tile`（平铺）；还可选 `top_left`、`center`、`bottom_right` |
| 横向间距倍数 | 0.2 |
| 纵向间距倍数 | 1.5 |
| 边距 X / Y (pt) | 2 / 2 |
| 小图过滤 (%) | 3 |

## 构建（Windows）

需已安装 Go，在仓库根目录执行：

```powershell
cd go-app
.\build.ps1
```

默认输出：

```text
dist/watermark-gui.exe
```

等价命令示例：

```powershell
go build -trimpath -ldflags "-s -w -H=windowsgui"
```

## 使用方式概要

1. 运行 `watermark-gui.exe`（或自行构建后的可执行文件）。
2. 「添加文件」或「添加文件夹」（文件夹会递归收集支持的格式）。
3. 在右侧调整水印文字、字体、颜色、位置等参数。
4. 点击「开始处理」，在「处理结果」区域查看每条日志。

## Go 版工程结构

```text
go-app/
  cmd/watermark-gui           GUI 入口
  internal/gui                界面与交互
  internal/config             默认值与参数校验
  internal/watermark          字体与水印渲染
  internal/openxml/pptx       PPTX
  internal/openxml/docx       DOCX
  internal/picture            图片流水线
  internal/processor          统一调度
  build.ps1                   构建脚本
```

## Python 版本

仍保留在 `src/`，主要作为 Go 重写时的行为基线与对照，**不是**面向最终用户的主要分发形态。
