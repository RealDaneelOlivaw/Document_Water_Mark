from __future__ import annotations

from dataclasses import dataclass
from importlib import import_module
from pathlib import Path
import sys

from .config import WatermarkConfig
from .ppt_processor import ProcessStats


SUPPORTED_SUFFIXES = frozenset({".pptx", ".docx", ".pdf"})
LEGACY_SUFFIXES = frozenset({".ppt", ".doc"})
_PROCESSOR_SPECS = {
    ".pptx": (".ppt_processor", "PptWatermarkProcessor"),
    ".docx": (".word_processor", "WordWatermarkProcessor"),
    ".pdf": (".pdf_processor", "PdfWatermarkProcessor"),
}

_dataclass = dataclass(slots=True) if sys.version_info >= (3, 10) else dataclass


@_dataclass
class ProcessResult:
    source: Path
    output: Path
    suffix: str
    stats: ProcessStats | None = None
    error: str | None = None
    pdf_mode: str | None = None

    @property
    def success(self) -> bool:
        return self.error is None


def collect_input_files(input_path: str | Path) -> list[Path]:
    target = Path(input_path)
    if target.is_file():
        suffix = target.suffix.lower()
        if suffix in LEGACY_SUFFIXES:
            raise ValueError(
                f"Legacy Office format is not supported, please convert first: {target}"
            )
        if suffix not in SUPPORTED_SUFFIXES:
            raise ValueError(f"Unsupported input file: {target}")
        return [target]

    if target.is_dir():
        files = sorted(
            file_path
            for file_path in target.iterdir()
            if file_path.is_file() and file_path.suffix.lower() in SUPPORTED_SUFFIXES
        )
        if not files:
            raise ValueError(f"No supported files found in directory: {target}")
        return files

    raise ValueError(f"Input path does not exist: {target}")


def build_output_path(input_file: Path, output_dir: Path | None = None) -> Path:
    directory = output_dir or input_file.parent
    return directory / f"{input_file.stem}_watermarked{input_file.suffix}"


def resolve_output_path(
    input_file: Path,
    *,
    output_dir: Path | None = None,
    reserved_paths: set[Path] | None = None,
) -> Path:
    reserved_paths = reserved_paths or set()
    candidate = build_output_path(input_file, output_dir)
    if candidate not in reserved_paths and not candidate.exists():
        return candidate

    index = 2
    while True:
        renamed = candidate.with_name(f"{candidate.stem}_{index}{candidate.suffix}")
        if renamed not in reserved_paths and not renamed.exists():
            return renamed
        index += 1


def create_processor(
    suffix: str,
    *,
    min_area_pct: int = 0,
) -> object:
    try:
        module_name, class_name = _PROCESSOR_SPECS[suffix]
    except KeyError as exc:
        raise ValueError(f"Unsupported file type: {suffix}") from exc

    try:
        module = import_module(module_name, package=__package__)
    except ModuleNotFoundError as exc:
        if suffix == ".pdf":
            raise RuntimeError(
                "PDF 处理依赖 PyMuPDF 当前不可用，请安装或修复 PyMuPDF 后再试。"
            ) from exc
        raise

    processor_cls = getattr(module, class_name)
    return processor_cls(min_area_pct=min_area_pct)


def create_processor_map(
    *,
    min_area_pct: int = 0,
    suffixes: set[str] | None = None,
) -> dict[str, object]:
    requested_suffixes = suffixes or set(_PROCESSOR_SPECS)
    return {
        suffix: create_processor(suffix, min_area_pct=min_area_pct)
        for suffix in requested_suffixes
    }


def process_inputs(
    inputs: list[Path],
    *,
    config: WatermarkConfig,
    output_dir: Path | None = None,
    min_area_pct: int = 0,
    pdf_mode: str = "images",
) -> list[ProcessResult]:
    if output_dir:
        output_dir.mkdir(parents=True, exist_ok=True)

    reserved_outputs: set[Path] = set()
    processors = create_processor_map(
        min_area_pct=min_area_pct,
        suffixes={source.suffix.lower() for source in inputs},
    )
    results: list[ProcessResult] = []

    for source in inputs:
        suffix = source.suffix.lower()
        output = resolve_output_path(
            source,
            output_dir=output_dir,
            reserved_paths=reserved_outputs,
        )
        reserved_outputs.add(output)

        try:
            processor = processors[suffix]
            if suffix == ".pdf":
                stats = processor.process_file(  # type: ignore[union-attr]
                    source,
                    output,
                    config,
                    mode=pdf_mode,
                )
            else:
                stats = processor.process_file(source, output, config)  # type: ignore[union-attr]
            results.append(
                ProcessResult(
                    source=source,
                    output=output,
                    suffix=suffix,
                    stats=stats,
                    pdf_mode=pdf_mode if suffix == ".pdf" else None,
                )
            )
        except Exception as exc:
            results.append(
                ProcessResult(
                    source=source,
                    output=output,
                    suffix=suffix,
                    error=str(exc),
                    pdf_mode=pdf_mode if suffix == ".pdf" else None,
                )
            )

    return results


def format_stats_line(result: ProcessResult) -> str:
    if result.stats is None:
        return f"失败: {result.source.name} ({result.error})"

    skip_info = (
        f"，跳过 {result.stats.skipped_images} 项"
        if result.stats.skipped_images
        else ""
    )
    if result.suffix == ".pdf" and result.pdf_mode == "page":
        return (
            f"成功: {result.source.name} -> {result.output.name} "
            f"(页面 {result.stats.total_images}，已加水印 {result.stats.watermarked_images}{skip_info})"
        )

    return (
        f"成功: {result.source.name} -> {result.output.name} "
        f"(图片 {result.stats.total_images}，水印 {result.stats.watermarked_images}{skip_info})"
    )
