from __future__ import annotations

import argparse
import logging
from pathlib import Path
from typing import Sequence

from .config import (
    DEFAULT_FONT_NAME,
    DEFAULT_FONT_SIZE,
    DEFAULT_GAP_X_RATIO,
    DEFAULT_GAP_Y_RATIO,
    DEFAULT_MARGIN_X,
    DEFAULT_MARGIN_Y,
    DEFAULT_MIN_AREA_PCT,
    DEFAULT_OPACITY,
    DEFAULT_ROTATION,
    WatermarkConfig,
)
from .pipeline import (
    build_output_path,
    collect_input_files,
    format_stats_line,
    process_inputs,
)


logger = logging.getLogger("ppt_watermark")


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Add text watermarks to pictures in PPT, Word, and PDF files."
    )
    parser.add_argument(
        "--input",
        required=True,
        help="Input supported file path or a directory containing supported files.",
    )
    parser.add_argument("--output-dir", help="Output directory. Default uses input location.")
    parser.add_argument("--text", required=True, help="Watermark text, supports \\n for multiline.")
    parser.add_argument("--font-name", default=DEFAULT_FONT_NAME, help="Font name.")
    parser.add_argument("--font-path", help="Font file path, preferred over font name.")
    parser.add_argument(
        "--font-size",
        type=int,
        default=DEFAULT_FONT_SIZE,
        help="Watermark visible font size in document points.",
    )
    parser.add_argument("--opacity", type=int, default=DEFAULT_OPACITY, help="Opacity in 0-100.")
    parser.add_argument("--color", default="#000000", help="Hex color, such as #000000.")
    parser.add_argument("--rotation", type=float, default=DEFAULT_ROTATION, help="Rotation angle.")
    parser.add_argument(
        "--position",
        choices=("top_left", "center", "bottom_right", "tile"),
        default="tile",
        help="Watermark position mode.",
    )
    parser.add_argument(
        "--gap-x",
        "--tile-spacing",
        dest="gap_x_ratio",
        type=float,
        default=DEFAULT_GAP_X_RATIO,
        help="Horizontal gap as a multiple of watermark width. Legacy alias: --tile-spacing.",
    )
    parser.add_argument(
        "--gap-y",
        "--tile-spacing-y",
        dest="gap_y_ratio",
        type=float,
        default=DEFAULT_GAP_Y_RATIO,
        help="Vertical gap as a multiple of watermark height. Legacy alias: --tile-spacing-y.",
    )
    parser.add_argument(
        "--margin-x",
        type=int,
        default=DEFAULT_MARGIN_X,
        help="Horizontal margin in document points.",
    )
    parser.add_argument(
        "--margin-y",
        type=int,
        default=DEFAULT_MARGIN_Y,
        help="Vertical margin in document points.",
    )
    parser.add_argument(
        "--min-area-pct",
        type=int,
        default=DEFAULT_MIN_AREA_PCT,
        help="Skip images smaller than this displayed area percentage.",
    )
    parser.add_argument(
        "--pdf-mode",
        choices=("page", "images"),
        default="images",
        help="How to watermark PDF files.",
    )
    return parser.parse_args(argv)


def run(argv: Sequence[str] | None = None) -> int:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s - %(message)s")
    try:
        args = parse_args(argv)
        config = WatermarkConfig(
            text=args.text.replace("\\n", "\n"),
            font_name=args.font_name,
            font_path=args.font_path,
            font_size=args.font_size,
            opacity=args.opacity,
            color=args.color,
            rotation=args.rotation,
            position=args.position,
            gap_x_ratio=args.gap_x_ratio,
            gap_y_ratio=args.gap_y_ratio,
            margin_x=args.margin_x,
            margin_y=args.margin_y,
        )

        inputs = collect_input_files(args.input)
        output_dir = Path(args.output_dir) if args.output_dir else None
        results = process_inputs(
            inputs,
            config=config,
            output_dir=output_dir,
            min_area_pct=args.min_area_pct,
            pdf_mode=args.pdf_mode,
        )

        has_error = False
        for result in results:
            if result.success:
                logger.info(format_stats_line(result))
            else:
                has_error = True
                logger.error("Failed to process %s: %s", result.source, result.error)
        return 1 if has_error else 0
    except Exception as exc:
        logger.error("Invalid parameters: %s", exc)
        return 1


if __name__ == "__main__":
    raise SystemExit(run())
