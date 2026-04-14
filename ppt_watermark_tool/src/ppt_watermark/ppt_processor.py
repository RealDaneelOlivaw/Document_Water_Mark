from __future__ import annotations

import hashlib
import io
from dataclasses import dataclass
from pathlib import Path
import sys

from lxml import etree
from PIL import Image
from pptx import Presentation
from pptx.opc.constants import RELATIONSHIP_TYPE as RT

from .config import WatermarkConfig
from .image_ops import (
    build_image_stream,
    build_overlay_cache_key,
    build_rendered_blob_key,
    build_scaled_config,
    compute_render_metrics,
    serialize_composited_image,
)
from .watermark_renderer import WatermarkRenderer


EMU_PER_INCH = 914400
_NS_A = "http://schemas.openxmlformats.org/drawingml/2006/main"
_NS_R_EMBED = "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}embed"

_dataclass = dataclass(slots=True) if sys.version_info >= (3, 10) else dataclass


@_dataclass
class ProcessStats:
    total_images: int = 0
    watermarked_images: int = 0
    skipped_images: int = 0


@_dataclass
class PptImageInstance:
    blip: object
    r_id: str
    width: int
    height: int
    size_is_known: bool


class PptWatermarkProcessor:
    def __init__(
        self,
        renderer: WatermarkRenderer | None = None,
        min_area_pct: int = 0,
    ) -> None:
        self.renderer = renderer or WatermarkRenderer()
        self.min_area_pct = min_area_pct

    def process_file(
        self, input_path: Path, output_path: Path, config: WatermarkConfig
    ) -> ProcessStats:
        prs = Presentation(str(input_path))
        stats = ProcessStats()
        overlay_cache: dict[tuple[object, ...], Image.Image] = {}
        rendered_blobs: dict[tuple[object, ...], tuple[bytes, str]] = {}
        invalid_blobs: set[str] = set()
        slide_area = prs.slide_width * prs.slide_height

        for slide in prs.slides:
            self._process_slide(
                slide,
                slide_area,
                config,
                overlay_cache,
                rendered_blobs,
                invalid_blobs,
                stats,
            )

        prs.save(str(output_path))
        return stats

    def _process_slide(
        self,
        slide: object,
        slide_area: int,
        config: WatermarkConfig,
        overlay_cache: dict[tuple[object, ...], Image.Image],
        rendered_blobs: dict[tuple[object, ...], tuple[bytes, str]],
        invalid_blobs: set[str],
        stats: ProcessStats,
    ) -> None:
        for instance in self._iter_slide_image_instances(slide):
            try:
                rel = slide.part.rels[instance.r_id]  # type: ignore[attr-defined]
                part = rel.target_part
                content_type: str = part._content_type or ""
            except (AttributeError, KeyError, ValueError):
                continue

            if "image" not in content_type:
                continue

            stats.total_images += 1

            if self.min_area_pct > 0 and slide_area > 0 and instance.size_is_known:
                pct = ((instance.width * instance.height) / slide_area) * 100.0
                if pct < self.min_area_pct:
                    stats.skipped_images += 1
                    continue

            blob: bytes = part._blob
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

            display_w = instance.width if instance.size_is_known else EMU_PER_INCH * 6
            display_h = instance.height if instance.size_is_known else EMU_PER_INCH * 4

            metrics = compute_render_metrics(
                config,
                image_px_width=px_w,
                image_px_height=px_h,
                display_width_units=display_w,
                display_height_units=display_h,
                units_per_inch=EMU_PER_INCH,
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
                new_blob, new_content_type = cached
                new_part = slide.part.package.get_or_add_image_part(  # type: ignore[attr-defined]
                    build_image_stream(new_blob, new_content_type)
                )
                new_r_id = slide.part.relate_to(new_part, RT.IMAGE)  # type: ignore[attr-defined]
                if new_r_id != instance.r_id:
                    instance.blip.set(_NS_R_EMBED, new_r_id)
                stats.watermarked_images += 1
            except Exception:
                invalid_blobs.add(blob_hash)
                stats.skipped_images += 1

        self._sync_img_layers(slide)

    @staticmethod
    def _iter_slide_image_instances(slide: object) -> list[PptImageInstance]:
        try:
            slide_el = slide.element  # type: ignore[attr-defined]
        except AttributeError:
            return []

        instances: list[PptImageInstance] = []
        for blip in slide_el.iter(f"{{{_NS_A}}}blip"):
            if PptWatermarkProcessor._is_img_layer_reference(blip):
                continue
            r_id = blip.get(_NS_R_EMBED, "")
            if not r_id:
                continue
            width, height, size_is_known = PptWatermarkProcessor._find_shape_size_for_blip(blip)
            instances.append(
                PptImageInstance(
                    blip=blip,
                    r_id=r_id,
                    width=width,
                    height=height,
                    size_is_known=size_is_known,
                )
            )
        return instances

    @staticmethod
    def _is_img_layer_reference(blip: object) -> bool:
        element = getattr(blip, "getparent", lambda: None)()
        while element is not None:
            if etree.QName(element.tag).localname == "imgLayer":  # type: ignore[arg-type]
                return True
            element = element.getparent()  # type: ignore[union-attr]
        return False

    @staticmethod
    def _find_shape_size_for_blip(blip: object) -> tuple[int, int, bool]:
        best_width = EMU_PER_INCH * 6
        best_height = EMU_PER_INCH * 4
        best_area = 0
        found = False

        element: object | None = blip
        while element is not None:
            ext = element.find(f".//{{{_NS_A}}}ext")  # type: ignore[union-attr]
            if ext is not None:
                try:
                    cx = int(ext.get("cx", "0"))
                    cy = int(ext.get("cy", "0"))
                except (TypeError, ValueError):
                    cx = 0
                    cy = 0
                if cx > 0 and cy > 0:
                    area = cx * cy
                    if area > best_area:
                        best_width = cx
                        best_height = cy
                        best_area = area
                        found = True
            element = element.getparent()  # type: ignore[union-attr]

        return best_width, best_height, found

    @staticmethod
    def _sync_img_layers(slide: object) -> None:
        ns_a = "http://schemas.openxmlformats.org/drawingml/2006/main"
        ns_r = "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}"
        try:
            slide_el = slide.element  # type: ignore[attr-defined]
            rels = slide.part.rels  # type: ignore[attr-defined]
        except AttributeError:
            return

        for blip in slide_el.iter("{%s}blip" % ns_a):
            blip_rid = blip.get(ns_r + "embed", "")
            if not blip_rid:
                continue

            for child in blip.iter():
                if etree.QName(child.tag).localname != "imgLayer":
                    continue
                layer_rid = child.get(ns_r + "embed", "")
                if not layer_rid:
                    continue
                try:
                    blip_part = rels[blip_rid].target_part
                    layer_part = rels[layer_rid].target_part
                    layer_part._blob = blip_part._blob
                    layer_part._content_type = blip_part._content_type
                except (KeyError, AttributeError):
                    pass

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
