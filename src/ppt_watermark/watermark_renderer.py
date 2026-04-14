from __future__ import annotations

import os
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont, ImageOps

from .config import WatermarkConfig

_WINDOWS_FONT_DIR = Path(os.environ.get("WINDIR", r"C:\Windows")) / "Fonts"
GUI_FONT_CHOICES = (
    "微软雅黑",
    "宋体",
    "黑体",
    "楷体",
    "仿宋",
    "Arial",
    "Calibri",
    "Cambria",
    "Times New Roman",
    "Consolas",
)
_FONT_NAME_CANDIDATES = {
    "微软雅黑": ("msyh.ttc", "msyhbd.ttc"),
    "宋体": ("simsun.ttc",),
    "黑体": ("simhei.ttf",),
    "楷体": ("simkai.ttf", "kaiti.ttf"),
    "仿宋": ("simfang.ttf", "fangsong.ttf"),
    "Arial": ("arial.ttf",),
    "Calibri": ("calibri.ttf",),
    "Cambria": ("cambria.ttc", "cambria.ttf"),
    "Times New Roman": ("times.ttf", "timesbd.ttf"),
    "Consolas": ("consola.ttf",),
}
_FALLBACK_FONT_NAMES = ("微软雅黑", "宋体", "Arial")


class WatermarkRenderer:
    def create_watermark_image(self, config: WatermarkConfig) -> Image.Image:
        font = self._load_font(config)
        line_spacing = max(int(config.font_size * 0.25), 8)
        padding_x = max(int(config.font_size * 0.45), 18)
        padding_y = max(int(config.font_size * 0.55), 22)
        text = config.text or " "

        measurement = Image.new("RGBA", (1, 1), (0, 0, 0, 0))
        measurement_draw = ImageDraw.Draw(measurement)
        bbox = measurement_draw.multiline_textbbox(
            (0, 0),
            text,
            font=font,
            spacing=line_spacing,
            align="left",
        )

        text_width = max((bbox[2] - bbox[0]) + padding_x * 2, 1)
        text_height = max((bbox[3] - bbox[1]) + padding_y * 2, 1)

        base = Image.new("RGBA", (text_width, text_height), (0, 0, 0, 0))
        draw = ImageDraw.Draw(base)
        rgba = self._hex_to_rgba(config.color, config.opacity)
        draw.multiline_text(
            (padding_x - bbox[0], padding_y - bbox[1]),
            text,
            fill=rgba,
            font=font,
            spacing=line_spacing,
            align="left",
        )

        if config.rotation:
            base = base.rotate(config.rotation, expand=True, resample=Image.Resampling.BICUBIC)
            base = ImageOps.expand(base, border=max(int(config.font_size * 0.18), 8), fill=(0, 0, 0, 0))
        return base

    def create_tiled_overlay(
        self, config: WatermarkConfig, width: int, height: int
    ) -> Image.Image:
        mark = self.create_watermark_image(config)
        bbox = self._visible_bbox(mark)
        visible_width = max(bbox[2] - bbox[0], 1)
        visible_height = max(bbox[3] - bbox[1], 1)
        step_x = max(int(round(visible_width * (1.0 + config.gap_x_ratio))), 1)
        step_y = max(int(round(visible_height * (1.0 + config.gap_y_ratio))), 1)

        overlay = Image.new("RGBA", (width, height), (0, 0, 0, 0))
        start_x = config.margin_x - bbox[0]
        start_y = config.margin_y - bbox[1]

        for y in range(start_y, height + visible_height + step_y, step_y):
            for x in range(start_x, width + visible_width + step_x, step_x):
                self._paste_with_clipping(overlay, mark, x, y)
        return overlay

    def create_overlay(self, config: WatermarkConfig, width: int, height: int) -> Image.Image:
        if config.position == "tile":
            return self.create_tiled_overlay(config, width, height)

        overlay = Image.new("RGBA", (width, height), (0, 0, 0, 0))
        mark = self.create_watermark_image(config)
        bbox = self._visible_bbox(mark)
        x, y = self._resolve_position(config, bbox, width, height)
        self._paste_with_clipping(overlay, mark, x, y)
        return overlay

    @staticmethod
    def _paste_with_clipping(
        canvas: Image.Image,
        mark: Image.Image,
        x: int,
        y: int,
    ) -> None:
        src_left = max(0, -x)
        src_top = max(0, -y)
        src_right = min(mark.width, canvas.width - x)
        src_bottom = min(mark.height, canvas.height - y)
        if src_left >= src_right or src_top >= src_bottom:
            return

        cropped = mark.crop((src_left, src_top, src_right, src_bottom))
        dest_x = max(x, 0)
        dest_y = max(y, 0)
        canvas.alpha_composite(cropped, (dest_x, dest_y))

    @staticmethod
    def _load_font(config: WatermarkConfig) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
        size = config.font_size
        candidates = WatermarkRenderer._build_font_candidates(config)
        for path in candidates:
            try:
                return ImageFont.truetype(path, size)
            except OSError:
                continue
        return ImageFont.load_default()

    @staticmethod
    def _build_font_candidates(config: WatermarkConfig) -> list[str]:
        requested = []
        if config.font_path:
            requested.append(config.font_path)

        for font_name in (config.font_name, *_FALLBACK_FONT_NAMES):
            requested.extend(_FONT_NAME_CANDIDATES.get(font_name, (font_name,)))

        expanded: list[str] = []
        seen: set[str] = set()
        for item in requested:
            for candidate in WatermarkRenderer._expand_font_candidate(item):
                key = candidate.lower()
                if key in seen:
                    continue
                seen.add(key)
                expanded.append(candidate)
        return expanded

    @staticmethod
    def _expand_font_candidate(candidate: str) -> list[str]:
        path = Path(candidate)
        values = [candidate]
        if path.is_absolute():
            return values

        values.append(str(_WINDOWS_FONT_DIR / candidate))
        if path.suffix:
            lower_name = candidate.lower()
            if lower_name != candidate:
                values.append(str(_WINDOWS_FONT_DIR / lower_name))
            return values

        for suffix in (".ttf", ".ttc", ".otf"):
            values.append(str(_WINDOWS_FONT_DIR / (candidate + suffix)))
            values.append(str(_WINDOWS_FONT_DIR / (candidate.lower() + suffix)))
        return values

    @staticmethod
    def _hex_to_rgba(hex_color: str, opacity: int) -> tuple[int, int, int, int]:
        color = hex_color.lstrip("#")
        r = int(color[0:2], 16)
        g = int(color[2:4], 16)
        b = int(color[4:6], 16)
        a = int(255 * (opacity / 100))
        return r, g, b, a

    @staticmethod
    def _resolve_position(
        config: WatermarkConfig,
        bbox: tuple[int, int, int, int],
        canvas_width: int,
        canvas_height: int,
    ) -> tuple[int, int]:
        bbox_left, bbox_top, bbox_right, bbox_bottom = bbox
        visible_width = bbox_right - bbox_left
        visible_height = bbox_bottom - bbox_top

        if config.position == "top_left":
            return config.margin_x - bbox_left, config.margin_y - bbox_top
        if config.position == "center":
            return (
                max((canvas_width - visible_width) // 2 - bbox_left, -bbox_left),
                max((canvas_height - visible_height) // 2 - bbox_top, -bbox_top),
            )
        if config.position == "bottom_right":
            return (
                canvas_width - visible_width - config.margin_x - bbox_left,
                canvas_height - visible_height - config.margin_y - bbox_top,
            )
        return config.margin_x - bbox_left, config.margin_y - bbox_top

    @staticmethod
    def _visible_bbox(mark: Image.Image) -> tuple[int, int, int, int]:
        bbox = mark.getbbox()
        if bbox is None:
            return (0, 0, mark.width, mark.height)
        return bbox
