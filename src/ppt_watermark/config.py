from __future__ import annotations

from dataclasses import dataclass
import re
import sys


ALLOWED_POSITIONS = {"top_left", "center", "bottom_right", "tile"}
HEX_COLOR_PATTERN = re.compile(r"^#(?:[0-9a-fA-F]{6})$")

DEFAULT_FONT_NAME = "msyh.ttc"
DEFAULT_FONT_SIZE = 10
DEFAULT_OPACITY = 30
DEFAULT_COLOR = "#000000"
DEFAULT_ROTATION = 30.0
DEFAULT_POSITION = "tile"
DEFAULT_GAP_X_RATIO = 0.05
DEFAULT_GAP_Y_RATIO = 0.05
DEFAULT_MARGIN_X = 1
DEFAULT_MARGIN_Y = 1
DEFAULT_MIN_AREA_PCT = 0


_dataclass = dataclass(slots=True) if sys.version_info >= (3, 10) else dataclass


@_dataclass
class WatermarkConfig:
    text: str
    font_name: str = DEFAULT_FONT_NAME
    font_path: str | None = None
    font_size: int = DEFAULT_FONT_SIZE
    opacity: int = DEFAULT_OPACITY
    color: str = DEFAULT_COLOR
    rotation: float = DEFAULT_ROTATION
    position: str = DEFAULT_POSITION
    gap_x_ratio: float = DEFAULT_GAP_X_RATIO
    gap_y_ratio: float = DEFAULT_GAP_Y_RATIO
    margin_x: int = DEFAULT_MARGIN_X
    margin_y: int = DEFAULT_MARGIN_Y

    def __post_init__(self) -> None:
        self.text = self.text.strip()
        if not self.text:
            raise ValueError("Watermark text cannot be empty.")
        if self.font_size <= 0:
            raise ValueError("font_size must be greater than 0.")
        if not (0 <= self.opacity <= 100):
            raise ValueError("opacity must be in range 0-100.")
        if not HEX_COLOR_PATTERN.match(self.color):
            raise ValueError("color must be a 6-digit hex string like #FFFFFF.")
        if self.position not in ALLOWED_POSITIONS:
            raise ValueError(
                f"position must be one of: {', '.join(sorted(ALLOWED_POSITIONS))}."
            )
        self.gap_x_ratio = float(self.gap_x_ratio)
        self.gap_y_ratio = float(self.gap_y_ratio)
        if self.gap_x_ratio < 0:
            raise ValueError("gap_x_ratio must be greater than or equal to 0.")
        if self.gap_y_ratio < 0:
            raise ValueError("gap_y_ratio must be greater than or equal to 0.")
