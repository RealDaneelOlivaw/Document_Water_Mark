from __future__ import annotations

import hashlib
import io
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import zipfile

from docx import Document
from docx.opc.constants import RELATIONSHIP_TYPE as RT
from lxml import etree
from PIL import Image

from .config import WatermarkConfig
from .image_ops import (
    build_image_stream,
    build_overlay_cache_key,
    build_rendered_blob_key,
    build_scaled_config,
    compute_render_metrics,
    serialize_composited_image,
)
from .ppt_processor import ProcessStats
from .watermark_renderer import WatermarkRenderer


EMU_PER_INCH = 914400
EMU_PER_POINT = 12700
_NS_A = "http://schemas.openxmlformats.org/drawingml/2006/main"
_NS_R_EMBED = "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}embed"
_NS_R_ID = "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}id"
_NS_V = "urn:schemas-microsoft-com:vml"
_VML_STYLE_DIMENSION = re.compile(r"(width|height)\s*:\s*([0-9.]+)\s*([a-zA-Z]+)")
_DOCX_NORMALIZE_ENV_SOURCE = "PPT_WATERMARK_DOCX_SOURCE"
_DOCX_NORMALIZE_ENV_TARGET = "PPT_WATERMARK_DOCX_TARGET"
_DOCX_NORMALIZE_SCRIPT = rf"""
$ErrorActionPreference = 'Stop'
$source = $env:{_DOCX_NORMALIZE_ENV_SOURCE}
$target = $env:{_DOCX_NORMALIZE_ENV_TARGET}

if ([string]::IsNullOrWhiteSpace($source) -or [string]::IsNullOrWhiteSpace($target)) {{
    Write-Output 'ERROR|Missing DOCX normalization paths.'
    exit 1
}}

$attempts = @(
    @{{ Name = 'Word'; ProgId = 'Word.Application'; SaveMethod = 'SaveAs'; Format = 16 }},
    @{{ Name = 'WPS'; ProgId = 'Kwps.Application'; SaveMethod = 'SaveAs2'; Format = 16 }},
    @{{ Name = 'WPS'; ProgId = 'KWPS.Application'; SaveMethod = 'SaveAs2'; Format = 16 }}
)

$failures = New-Object System.Collections.Generic.List[string]
foreach ($attempt in $attempts) {{
    $app = $null
    $doc = $null
    try {{
        $app = New-Object -ComObject $attempt.ProgId
        $app.Visible = $false
        if ($app.PSObject.Properties.Name -contains 'DisplayAlerts') {{
            $app.DisplayAlerts = 0
        }}

        $doc = $app.Documents.Open($source, $false, $true)
        if ($attempt.SaveMethod -eq 'SaveAs2' -and ($doc.PSObject.Methods.Name -contains 'SaveAs2')) {{
            $doc.SaveAs2($target, $attempt.Format)
        }} else {{
            $doc.SaveAs([ref]$target, [ref]$attempt.Format)
        }}

        Write-Output ('OK|' + $attempt.Name + '|' + $attempt.ProgId)
        exit 0
    }} catch {{
        $failures.Add($attempt.ProgId + ': ' + $_.Exception.Message)
    }} finally {{
        if ($doc -ne $null) {{
            try {{ $doc.Close([ref]$false) }} catch {{ }}
        }}
        if ($app -ne $null) {{
            try {{ $app.Quit() }} catch {{ }}
        }}
    }}
}}

Write-Output ('ERROR|' + ($failures -join ' || '))
exit 1
"""


class WordImageInstance:
    __slots__ = ("owner_part", "element", "attribute", "r_id", "width", "height", "size_is_known")

    def __init__(
        self,
        owner_part: object,
        element: object,
        attribute: str,
        r_id: str,
        width: int,
        height: int,
        size_is_known: bool,
    ) -> None:
        self.owner_part = owner_part
        self.element = element
        self.attribute = attribute
        self.r_id = r_id
        self.width = width
        self.height = height
        self.size_is_known = size_is_known


class WordWatermarkProcessor:
    """Watermark embedded images in a DOCX package."""

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
        resolved_input = input_path.resolve()
        if not resolved_input.exists():
            raise FileNotFoundError(f"输入文件不存在: {input_path}")

        temp_dir: tempfile.TemporaryDirectory[str] | None = None
        try:
            prepared_input = resolved_input
            if not zipfile.is_zipfile(resolved_input):
                temp_dir = tempfile.TemporaryDirectory(prefix="ppt_watermark_docx_")
                normalized_input = Path(temp_dir.name) / f"{resolved_input.stem}_normalized.docx"
                self._normalize_with_local_office(resolved_input, normalized_input)
                if not normalized_input.exists() or not zipfile.is_zipfile(normalized_input):
                    raise ValueError(
                        f"{input_path.name} 已尝试通过本机 Word/WPS 自动另存为标准 `.docx`，"
                        "但生成的结果仍无法被内置 DOCX 解析器读取。"
                    )
                prepared_input = normalized_input

            doc = Document(str(prepared_input))
            stats = ProcessStats()
            overlay_cache: dict[tuple[object, ...], Image.Image] = {}
            rendered_blobs: dict[tuple[object, ...], tuple[bytes, str]] = {}
            invalid_blobs: set[str] = set()

            try:
                section = doc.sections[0]
                doc_area = section.page_width * section.page_height
            except (IndexError, AttributeError):
                doc_area = 0

            for instance in self._iter_image_instances(doc):
                try:
                    rel = instance.owner_part.rels[instance.r_id]
                    part = rel.target_part
                    content_type: str = part._content_type or ""
                except (AttributeError, KeyError, ValueError):
                    continue

                if "image" not in content_type:
                    continue

                stats.total_images += 1

                if self.min_area_pct > 0 and doc_area > 0 and instance.size_is_known:
                    pct = ((instance.width * instance.height) / doc_area) * 100.0
                    if pct < self.min_area_pct:
                        stats.skipped_images += 1
                        continue

                try:
                    blob: bytes = part._blob
                except AttributeError:
                    stats.skipped_images += 1
                    continue

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
                    new_part = instance.owner_part.package.get_or_add_image_part(
                        build_image_stream(new_blob, new_content_type)
                    )
                    new_r_id = instance.owner_part.relate_to(new_part, RT.IMAGE)
                    if new_r_id != instance.r_id:
                        instance.element.set(instance.attribute, new_r_id)
                    stats.watermarked_images += 1
                except Exception:
                    invalid_blobs.add(blob_hash)
                    stats.skipped_images += 1

            doc.save(str(output_path.resolve()))
            return stats
        finally:
            if temp_dir is not None:
                temp_dir.cleanup()

    def _normalize_with_local_office(self, input_path: Path, target_path: Path) -> str:
        if sys.platform != "win32":
            raise ValueError(
                f"{input_path.name} 当前无法被内置 DOCX 解析器直接读取，"
                "并且自动兼容兜底仅支持 Windows 上的 Word/WPS。"
            )

        environment = os.environ.copy()
        environment[_DOCX_NORMALIZE_ENV_SOURCE] = str(input_path)
        environment[_DOCX_NORMALIZE_ENV_TARGET] = str(target_path)

        try:
            completed = subprocess.run(
                [
                    "powershell",
                    "-NoProfile",
                    "-STA",
                    "-ExecutionPolicy",
                    "Bypass",
                    "-Command",
                    _DOCX_NORMALIZE_SCRIPT,
                ],
                capture_output=True,
                check=False,
                encoding="utf-8",
                errors="replace",
                env=environment,
                timeout=180,
            )
        except FileNotFoundError as exc:
            raise ValueError(
                f"{input_path.name} 当前无法被内置 DOCX 解析器直接读取，"
                "并且系统里没有可用的 PowerShell 来触发 Word/WPS 自动兼容。"
            ) from exc
        except subprocess.TimeoutExpired as exc:
            raise ValueError(
                f"自动调用本机 Word/WPS 兼容处理 {input_path.name} 超时，请手动在 Word/WPS 中另存为新的 `.docx` 后再试。"
            ) from exc

        if completed.returncode != 0:
            detail = (completed.stderr or completed.stdout or "").strip()
            if detail.startswith("ERROR|"):
                detail = detail.split("|", 1)[1].strip()
            if not detail:
                detail = "未返回更多错误细节。"
            raise ValueError(
                f"{input_path.name} 当前无法被内置 DOCX 解析器直接读取，"
                "并且自动调用本机 Word/WPS 另存为标准 `.docx` 也失败了。"
                f"详细信息：{detail}"
            )

        return (completed.stdout or "").strip()

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

    @staticmethod
    def _iter_image_instances(doc: Document) -> list[WordImageInstance]:
        try:
            package_parts = list(doc.part.package.parts)  # type: ignore[attr-defined]
        except AttributeError:
            package_parts = [doc.part]

        instances: list[WordImageInstance] = []
        for owner_part in package_parts:
            owner_element = getattr(owner_part, "element", None)
            rels = getattr(owner_part, "rels", None)
            if owner_element is None or not rels:
                continue

            for blip in owner_element.iter(f"{{{_NS_A}}}blip"):
                r_id = blip.get(_NS_R_EMBED, "")
                if not r_id:
                    continue
                width, height, size_is_known = WordWatermarkProcessor._find_size_for_blip(blip)
                instances.append(
                    WordImageInstance(
                        owner_part,
                        blip,
                        _NS_R_EMBED,
                        r_id,
                        width,
                        height,
                        size_is_known,
                    )
                )

            for imagedata in owner_element.iter(f"{{{_NS_V}}}imagedata"):
                r_id = imagedata.get(_NS_R_ID, "")
                if not r_id:
                    continue
                width, height, size_is_known = WordWatermarkProcessor._find_size_for_vml_image(
                    imagedata
                )
                instances.append(
                    WordImageInstance(
                        owner_part,
                        imagedata,
                        _NS_R_ID,
                        r_id,
                        width,
                        height,
                        size_is_known,
                    )
                )

        return instances

    @staticmethod
    def _find_size_for_blip(blip: object) -> tuple[int, int, bool]:
        best_width = EMU_PER_INCH * 6
        best_height = EMU_PER_INCH * 4
        best_area = 0
        found = False

        element: object | None = blip
        while element is not None:
            name = etree.QName(element.tag).localname  # type: ignore[arg-type]
            if name in {"inline", "anchor"}:
                extent = None
                for child in element:  # type: ignore[union-attr]
                    if etree.QName(child.tag).localname == "extent":
                        extent = child
                        break
                if extent is None:
                    for child in element.iter():  # type: ignore[union-attr]
                        if child is element:
                            continue
                        if etree.QName(child.tag).localname == "extent":
                            extent = child
                            break
                if extent is not None:
                    try:
                        cx = int(extent.get("cx", "0"))
                        cy = int(extent.get("cy", "0"))
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
                break
            element = element.getparent()  # type: ignore[union-attr]

        return best_width, best_height, found

    @staticmethod
    def _find_size_for_vml_image(imagedata: object) -> tuple[int, int, bool]:
        shape = imagedata.getparent()
        while shape is not None:
            local_name = etree.QName(shape.tag).localname  # type: ignore[arg-type]
            if local_name == "shape":
                cx, cy = WordWatermarkProcessor._extract_vml_shape_size(shape)
                if cx > 0 and cy > 0:
                    return cx, cy, True
                break
            shape = shape.getparent()  # type: ignore[union-attr]
        return EMU_PER_INCH * 6, EMU_PER_INCH * 4, False

    @staticmethod
    def _extract_vml_shape_size(shape: object) -> tuple[int, int]:
        style = getattr(shape, "get", lambda *_args, **_kwargs: None)("style", "") or ""
        width = 0
        height = 0

        for match in _VML_STYLE_DIMENSION.finditer(style):
            dimension, raw_value, unit = match.groups()
            emu = WordWatermarkProcessor._convert_length_to_emu(raw_value, unit)
            if dimension == "width":
                width = emu
            elif dimension == "height":
                height = emu

        return width, height

    @staticmethod
    def _convert_length_to_emu(raw_value: str, unit: str) -> int:
        try:
            value = float(raw_value)
        except ValueError:
            return 0

        factor_map = {
            "in": EMU_PER_INCH,
            "pt": EMU_PER_POINT,
            "cm": 360000,
            "mm": 36000,
            "px": 9525,
        }
        factor = factor_map.get(unit.lower())
        if factor is None:
            return 0
        return max(int(value * factor), 0)
