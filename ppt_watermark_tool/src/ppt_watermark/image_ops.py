from __future__ import annotations

from dataclasses import dataclass
import io
import sys
from typing import Hashable

from PIL import Image

from .config import WatermarkConfig

_dataclass = dataclass(slots=True) if sys.version_info >= (3, 10) else dataclass


@_dataclass
class DisplayUsage:
    width: int
    height: int
    size_is_known: bool = False

    @property
    def area(self) -> int:
        return self.width * self.height

    def update(self, width: int, height: int, size_is_known: bool) -> None:
        if not size_is_known:
            return
        if not self.size_is_known or (width * height) > self.area:
            self.width = width
            self.height = height
            self.size_is_known = True


@_dataclass
class RenderMetrics:
    font_size_px: int
    margin_x_px: int
    margin_y_px: int


class _NamedBytesIO(io.BytesIO):
    def __init__(self, data: bytes, name: str):
        super().__init__(data)
        self.name = name


def compute_render_metrics(
    config: WatermarkConfig,
    *,
    image_px_width: int,
    image_px_height: int,
    display_width_units: int,
    display_height_units: int,
    units_per_inch: float,
) -> RenderMetrics:
    display_w_in = max(display_width_units / units_per_inch, 0.1)
    display_h_in = max(display_height_units / units_per_inch, 0.1)
    dpi_x = image_px_width / display_w_in
    dpi_y = image_px_height / display_h_in
    avg_dpi = (dpi_x + dpi_y) / 2.0
    return RenderMetrics(
        font_size_px=max(int(round(config.font_size * avg_dpi / 72.0)), 1),
        margin_x_px=max(int(round(config.margin_x * dpi_x / 72.0)), 0),
        margin_y_px=max(int(round(config.margin_y * dpi_y / 72.0)), 0),
    )


def build_scaled_config(
    config: WatermarkConfig,
    *,
    font_size: int,
    margin_x: int,
    margin_y: int,
) -> WatermarkConfig:
    return WatermarkConfig(
        text=config.text,
        font_name=config.font_name,
        font_path=config.font_path,
        font_size=max(font_size, 1),
        opacity=config.opacity,
        color=config.color,
        rotation=config.rotation,
        position=config.position,
        gap_x_ratio=config.gap_x_ratio,
        gap_y_ratio=config.gap_y_ratio,
        margin_x=max(margin_x, 0),
        margin_y=max(margin_y, 0),
    )


def build_overlay_cache_key(
    config: WatermarkConfig,
    *,
    width: int,
    height: int,
) -> tuple[Hashable, ...]:
    return (
        width,
        height,
        config.text,
        config.font_name,
        config.font_path or "",
        config.font_size,
        config.opacity,
        config.color,
        config.rotation,
        config.position,
        config.gap_x_ratio,
        config.gap_y_ratio,
        config.margin_x,
        config.margin_y,
    )


def build_rendered_blob_key(
    blob_hash: str,
    content_type: str,
    overlay_key: tuple[Hashable, ...],
) -> tuple[Hashable, ...]:
    return (
        blob_hash,
        content_type,
        *overlay_key,
    )


def build_image_stream(blob: bytes, content_type: str) -> io.BytesIO:
    return _NamedBytesIO(blob, f"watermarked.{resolve_output_extension(content_type)}")


def serialize_composited_image(
    composited: Image.Image,
    content_type: str,
    has_alpha: bool,
) -> tuple[bytes, str]:
    fmt, output_content_type = _resolve_output_format(content_type)
    buf = io.BytesIO()

    if fmt == "JPEG":
        composited.convert("RGB").save(buf, format=fmt, quality=95)
    elif fmt in {"BMP", "GIF"}:
        composited.convert("RGB").save(buf, format=fmt)
    elif fmt == "TIFF":
        composited.convert("RGB").save(buf, format=fmt, compression="tiff_deflate")
    elif has_alpha:
        composited.save(buf, format=fmt)
    else:
        composited.convert("RGB").save(buf, format=fmt)

    return buf.getvalue(), output_content_type


def resolve_output_extension(content_type: str) -> str:
    fmt, _ = _resolve_output_format(content_type)
    if fmt == "JPEG":
        return "jpg"
    return fmt.lower()


def _resolve_output_format(content_type: str) -> tuple[str, str]:
    normalized = content_type.lower()
    if "jpeg" in normalized or "jpg" in normalized:
        return "JPEG", "image/jpeg"
    if "png" in normalized:
        return "PNG", "image/png"
    if "gif" in normalized:
        return "GIF", "image/gif"
    if "bmp" in normalized:
        return "BMP", "image/bmp"
    if "tif" in normalized:
        return "TIFF", "image/tiff"
    return "PNG", "image/png"
