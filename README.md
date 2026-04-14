# 文档图片水印工具

这个仓库现在保留两条实现线：

1. `src/` 下的 Python 版本  
   用来保存当前已经验证过的参考实现与处理逻辑。
2. `go-app/` 下的 Go 版本  
   这是后续准备分发给别人使用的 Windows 原生 GUI 版，目标是打成单文件 exe。

## 当前分支

当前 `go-branch` 的工作重点是 Go 重写版。

## Go 版范围

- 只支持 `PPTX`
- 只支持 `DOCX`
- 不再支持 `PDF`
- 输出文件写回原文档所在目录
- 不覆盖原文件，默认输出为 `_watermarked`，重名时自动追加序号

## Go 版默认参数

- 水印文字：`超卓航科广州研究院`
- 字号：`14`
- 颜色：`#000000`
- 透明度：`30`
- 旋转角度：`30`
- 位置模式：`tile`
- 横向间距倍数：`0.2`
- 纵向间距倍数：`0.2`
- 边距 X / Y：`1`
- 小图过滤：`0`

## Go 版工程结构

```text
go-app/
  cmd/watermark-gui           Windows GUI 入口
  internal/gui                原生 GUI
  internal/config             默认值与参数校验
  internal/watermark          字体加载与水印渲染
  internal/openxml/pptx       PPTX 处理
  internal/openxml/docx       DOCX 处理
  internal/compat/office      Word/WPS 兼容兜底
  build.ps1                   构建脚本
```

## Go 版构建

确保本机已经安装 Go，然后执行：

```powershell
cd go-app
.\build.ps1
```

默认会产出：

```text
dist/watermark-gui.exe
```

构建命令等价于：

```powershell
go build -trimpath -ldflags "-s -w -H=windowsgui"
```

## Python 版本说明

Python 版本仍然保留在仓库里，主要用途是：

- 作为 Go 重写时的行为基线
- 用于对照算法与兼容细节
- 在 Go 版未完全替代前继续作为参考实现

它不再是后续面向最终用户的主要分发形态。
