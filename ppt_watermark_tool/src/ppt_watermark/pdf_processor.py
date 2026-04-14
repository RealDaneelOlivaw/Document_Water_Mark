from __future__ import annotations

import hashlib
import io
from pathlib import Path

import fitz
from PIL import Image

from .config import WatermarkConfig
from .image_ops import (
    DisplayUsage,
    build_overlay_cache_key,
    build_rendered_blob_key,
    build_scaled_config,
    compute_render_metrics,
    serialize_composited_image,
)
from .ppt_processor import ProcessStats
from .watermark_renderer import WatermarkRenderer


class PdfWatermarkProcessor:
    """PDF watermark: full-page overlay or per-image burn-in."""

    def __init__(
        self,
        renderer: WatermarkRenderer | None = None,
        min_area_pct: int = 0,
        page_render_dpi: float = 150.0,
    ) -> None:
        self.renderer = renderer or WatermarkRenderer()
        self.min_area_pct = min_area_pct
        self.page_render_dpi = page_render_dpi

    def process_file(
        self,
        input_path: Path,
        output_path: Path,
        config: WatermarkConfig,
        mode: str = "page",
    ) -> ProcessStats:
        if mode == "page":
            return self._process_page_mode(input_path, output_path, config)
        if mode == "images":
            return self._process_image_mode(input_path, output_path, config)
        raise ValueError(f"Unsupported PDF mode: {mode!r}")

    def _process_page_mode(
        self,
        input_path: Path,
        output_path: Path,
        config: WatermarkConfig,
    ) -> ProcessStats:
        stats = ProcessStats()
        zoom = self.page_render_dpi / 72.0
        doc = fitz.open(str(input_path))
        try:
            stats.total_images = len(doc)
            for page in doc:
                rect = page.rect
                width = max(int(rect.width * zoom), 1)
                height = max(int(rect.height * zoom), 1)
                scaled_config = build_scaled_config(
                    config,
                    font_size=max(int(round(config.font_size * zoom)), 1),
                    margin_x=max(int(round(config.margin_x * zoom)), 0),
                    margin_y=max(int(round(config.margin_y * zoom)), 0),
                )
                overlay = self.renderer.create_overlay(scaled_config, width, height)
                buf = io.BytesIO()
                overlay.save(buf, format="PNG")
                page.insert_image(rect, stream=buf.getvalue(), overlay=True)
                stats.watermarked_images += 1
            doc.save(str(output_path), garbage=4, deflate=True)
        finally:
            doc.close()
        return stats

    def _process_image_mode(
        self,
        input_path: Path,
        output_path: Path,
        config: WatermarkConfig,
    ) -> ProcessStats:
        stats = ProcessStats()
        overlay_cache: dict[tuple[object, ...], Image.Image] = {}
        rendered_blobs: dict[tuple[object, ...], tuple[bytes, str]] = {}
        invalid_blobs: set[str] = set()

        doc = fitz.open(str(input_path))
        try:
            usage_by_xref, pct_by_xref = self._collect_display_usage(doc)

            for xref in sorted(usage_by_xref):
                stats.total_images += 1
                pct, size_is_known = pct_by_xref.get(xref, (0.0, False))
                if self.min_area_pct > 0 and size_is_known and pct < self.min_area_pct:
                    stats.skipped_images += 1
                    continue

                try:
                    extracted = doc.extract_image(xref)
                except Exception:
                    stats.skipped_images += 1
                    continue

                blob: bytes = extracted["image"]
                blob_hash = hashlib.md5(blob).hexdigest()
                if blob_hash in invalid_blobs:
                    stats.skipped_images += 1
                    continue

                try:
                    img = Image.open(io.BytesIO(blob))
                    px_w, px_h = img.size
                except Exception:
                    invalid_blobs.add(blob_hash)
                    stats.skipped_images += 1
                    continue

                if px_w < 10 or px_h < 10:
                    invalid_blobs.add(blob_hash)
                    stats.skipped_images += 1
                    continue

                ext = (extracted.get("ext") or "png").lower()
                content_type = "image/jpeg" if ext in {"jpg", "jpeg"} else "image/png"
                usage = usage_by_xref.get(xref, DisplayUsage(px_w, px_h, False))
                display_w = usage.width if usage.size_is_known else px_w
                display_h = usage.height if usage.size_is_known else px_h
                metrics = compute_render_metrics(
                    config,
                    image_px_width=px_w,
                    image_px_height=px_h,
                    display_width_units=display_w,
                    display_height_units=display_h,
                    units_per_inch=72.0,
                )
                scaled_config = build_scaled_config(
                    config,
                    font_size=metrics.font_size_px,
                    margin_x=metrics.margin_x_px,
                    margin_y=metrics.margin_y_px,
                )
                overlay_key = build_overlay_cache_key(
                    scaled_config,
                    width=px_w,
                    height=px_h,
                )
                render_key = build_rendered_blob_key(blob_hash, content_type, overlay_key)

                try:
                    cached = rendered_blobs.get(render_key)
                    if cached is None:
                        cached = self._render_watermark(
                            blob,
                            content_type,
                            px_w,
                            px_h,
                            scaled_config,
                            overlay_cache,
                        )
                        rendered_blobs[render_key] = cached
                    new_blob, _new_content_type = cached
                    self._replace_image(doc, xref, new_blob)
                    stats.watermarked_images += 1
                except Exception:
                    invalid_blobs.add(blob_hash)
                    stats.skipped_images += 1

            doc.save(str(output_path), garbage=4, deflate=True)
        finally:
            doc.close()

        return stats

    def _collect_display_usage(
        self,
        doc: fitz.Document,
    ) -> tuple[dict[int, DisplayUsage], dict[int, tuple[float, bool]]]:
        usage_by_xref: dict[int, DisplayUsage] = {}
        pct_by_xref: dict[int, tuple[float, bool]] = {}

        for page in doc:
            page_area = page.rect.width * page.rect.height
            for info in page.get_images(full=True):
                xref = int(info[0])
                width, height, pct, size_is_known = self._inspect_image_rects(
                    page,
                    xref,
                    page_area,
                )
                usage = usage_by_xref.setdefault(xref, DisplayUsage(0, 0, False))
                usage.update(width, height, size_is_known)
                prev_pct, prev_known = pct_by_xref.get(xref, (0.0, False))
                pct_by_xref[xref] = (max(prev_pct, pct), prev_known or size_is_known)

        return usage_by_xref, pct_by_xref

    @staticmethod
    def _inspect_image_rects(
        page: fitz.Page,
        xref: int,
        page_area: float,
    ) -> tuple[int, int, float, bool]:
        rects_fn = getattr(page, "get_image_rects", None)
        if not callable(rects_fn):
            return 0, 0, 0.0, False

        try:
            rects = rects_fn(xref)
        except Exception:
            return 0, 0, 0.0, False

        if not rects:
            return 0, 0, 0.0, False

        best_rect = max(rects, key=lambda rect: rect.width * rect.height)
        best_area = best_rect.width * best_rect.height
        pct = (best_area / page_area) * 100.0 if page_area > 0 else 0.0
        return (
            int(round(best_rect.width)),
            int(round(best_rect.height)),
            pct,
            True,
        )

    @staticmethod
    def _replace_image(doc: fitz.Document, xref: int, stream: bytes) -> None:
        if len(doc) < 1:
            raise RuntimeError("PDF has no pages")
        page = doc[0]
        page_replace = getattr(page, "replace_image", None)
        if callable(page_replace):
            page_replace(xref, stream=stream)
            return
        doc_replace = getattr(doc, "replace_image", None)
        if callable(doc_replace):
            doc_replace(xref, stream=stream)
            return
        raise RuntimeError("PyMuPDF version does not support replace_image")

    def _render_watermark(
        self,
        blob: bytes,
        content_type: str,
        px_w: int,
        px_h: int,
        scaled_config: WatermarkConfig,
        overlay_cache: dict[tuple[object, ...], Image.Image],
    ) -> tuple[bytes, str]:
        original = Image.open(io.BytesIO(blob))
        if original.mode not in ("RGBA", "RGB"):
            original = original.convert("RGBA")
        has_alpha = original.mode in ("RGBA", "LA")
        original = original.convert("RGBA")

        overlay_key = build_overlay_cache_key(
            scaled_config,
            width=px_w,
            height=px_h,
        )
        overlay = overlay_cache.get(overlay_key)
        if overlay is None:
            overlay = self.renderer.create_overlay(scaled_config, px_w, px_h)
            overlay_cache[overlay_key] = overlay

        composited = Image.alpha_composite(original, overlay)
        return serialize_composited_image(composited, content_type, has_alpha)
