# 文档图片自动加文字水印

本工具用于给 `.pptx`、`.docx`、`.pdf` 中的图片批量叠加文字水印，支持配置字体、字号、透明度、颜色、旋转角度与位置。

## 环境要求

- Python 3.10+

## 安装依赖

```bash
python -m pip install -e .
```

## 命令行用法

单文件处理：

```bash
python -m ppt_watermark --input "demo.pptx" --text "CONFIDENTIAL" --font-size 32 --opacity 35
```

目录批处理：

```bash
python -m ppt_watermark --input "C:\\docs" --text "内部资料" --position tile --opacity 30 --output-dir "C:\\docs_out"
```

多行水印：

```bash
python -m ppt_watermark --input "demo.pptx" --text "CONFIDENTIAL\\nDO NOT SHARE"
```

PDF 整页模式：

```bash
python -m ppt_watermark --input "report.pdf" --text "CONFIDENTIAL" --pdf-mode page
```

## 参数说明

- `--input`：输入 `.pptx`、`.docx`、`.pdf` 文件路径，或包含这些文件的目录（必填）
- `--output-dir`：输出目录（可选，默认输出到原文件目录）
- `--text`：水印文字（必填，支持 `\\n` 表示换行）
- `--font-name`：字体名（默认 `msyh.ttc`）
- `--font-path`：字体文件路径（设置后优先于字体名）
- `--font-size`：字号（默认 `28`）
- `--opacity`：透明度，`0-100`（默认 `40`）
- `--color`：颜色，十六进制（默认 `#FFFFFF`）
- `--rotation`：旋转角度（默认 `30`）
- `--position`：`top_left | center | bottom_right | tile`（默认 `tile`）
- `--tile-spacing`：平铺间距像素（默认 `160`）
- `--margin-x` / `--margin-y`：非平铺模式下的边距（默认 `24`）
- `--min-area-pct`：跳过显示面积占比低于该值的小图（默认 `5`）
- `--pdf-mode`：PDF 处理模式，`images` 仅处理图片，`page` 对整页叠加水印（默认 `images`）

## 输出规则

- 默认输出文件名为 `原文件名_watermarked.扩展名`
- 不覆盖原始文档；若目标文件已存在，会自动追加序号
