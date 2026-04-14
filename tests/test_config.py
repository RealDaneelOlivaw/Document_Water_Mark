import unittest

from ppt_watermark.config import WatermarkConfig


class WatermarkConfigTests(unittest.TestCase):
    def test_default_values_are_valid(self) -> None:
        cfg = WatermarkConfig(text="Confidential")
        self.assertEqual(cfg.font_size, 36)
        self.assertEqual(cfg.opacity, 40)
        self.assertEqual(cfg.color, "#000000")
        self.assertEqual(cfg.position, "tile")
        self.assertEqual(cfg.gap_x_ratio, 0.5)
        self.assertEqual(cfg.gap_y_ratio, 0.5)
        self.assertEqual(cfg.margin_x, 5)
        self.assertEqual(cfg.margin_y, 5)

    def test_negative_gap_ratio_raises(self) -> None:
        with self.assertRaises(ValueError):
            WatermarkConfig(text="A", gap_x_ratio=-0.1)

    def test_invalid_opacity_raises(self) -> None:
        with self.assertRaises(ValueError):
            WatermarkConfig(text="A", opacity=120)

    def test_invalid_font_size_raises(self) -> None:
        with self.assertRaises(ValueError):
            WatermarkConfig(text="A", font_size=0)

    def test_invalid_hex_color_raises(self) -> None:
        with self.assertRaises(ValueError):
            WatermarkConfig(text="A", color="white")

    def test_invalid_position_raises(self) -> None:
        with self.assertRaises(ValueError):
            WatermarkConfig(text="A", position="middle")

    def test_empty_text_raises(self) -> None:
        with self.assertRaises(ValueError):
            WatermarkConfig(text="   ")


if __name__ == "__main__":
    unittest.main()
