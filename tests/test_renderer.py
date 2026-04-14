import unittest
from unittest import mock

from ppt_watermark.config import WatermarkConfig
from ppt_watermark.watermark_renderer import GUI_FONT_CHOICES, WatermarkRenderer


class WatermarkRendererTests(unittest.TestCase):
    def setUp(self) -> None:
        self.renderer = WatermarkRenderer()

    def test_single_watermark_image_has_alpha_channel(self) -> None:
        cfg = WatermarkConfig(text="Demo", opacity=35, rotation=30)
        image = self.renderer.create_watermark_image(cfg)
        self.assertEqual(image.mode, "RGBA")
        self.assertGreater(image.width, 0)
        self.assertGreater(image.height, 0)

    def test_multiline_watermark_can_be_rendered(self) -> None:
        cfg = WatermarkConfig(text="Line1\nLine2", opacity=50)
        image = self.renderer.create_watermark_image(cfg)
        self.assertEqual(image.mode, "RGBA")
        self.assertGreater(image.width, 0)
        self.assertGreater(image.height, 0)

    def test_watermark_image_keeps_transparent_padding_after_rotation(self) -> None:
        cfg = WatermarkConfig(text="超卓航科广州研究院", rotation=30, font_size=36)
        image = self.renderer.create_watermark_image(cfg)
        bbox = image.getbbox()
        self.assertIsNotNone(bbox)
        left, top, right, bottom = bbox
        self.assertGreater(left, 0)
        self.assertGreater(top, 0)
        self.assertLess(right, image.width)
        self.assertLess(bottom, image.height)

    def test_tile_canvas_is_generated_with_target_size(self) -> None:
        cfg = WatermarkConfig(text="Tile", position="tile", gap_x_ratio=0.5, gap_y_ratio=0.5)
        tiled = self.renderer.create_tiled_overlay(cfg, width=640, height=360)
        self.assertEqual(tiled.size, (640, 360))
        self.assertEqual(tiled.mode, "RGBA")

    def test_gap_ratio_changes_density_without_changing_mark_size(self) -> None:
        dense = WatermarkConfig(text="Tile", position="tile", gap_x_ratio=0.1, gap_y_ratio=0.1)
        sparse = WatermarkConfig(text="Tile", position="tile", gap_x_ratio=1.5, gap_y_ratio=1.5)
        dense_mark = self.renderer.create_watermark_image(dense)
        sparse_mark = self.renderer.create_watermark_image(sparse)
        dense_overlay = self.renderer.create_tiled_overlay(dense, width=640, height=360)
        sparse_overlay = self.renderer.create_tiled_overlay(sparse, width=640, height=360)
        self.assertEqual(dense_mark.size, sparse_mark.size)
        self.assertNotEqual(dense_overlay.tobytes(), sparse_overlay.tobytes())

    def test_gui_font_choices_start_with_microsoft_yahei(self) -> None:
        self.assertEqual(GUI_FONT_CHOICES[0], "微软雅黑")

    def test_font_candidates_include_mapped_and_fallback_fonts(self) -> None:
        cfg = WatermarkConfig(text="Demo", font_name="楷体")
        candidates = self.renderer._build_font_candidates(cfg)
        joined = "\n".join(candidates)
        self.assertIn("simkai.ttf", joined.lower())
        self.assertIn("msyh.ttc", joined.lower())
        self.assertIn("simsun.ttc", joined.lower())
        self.assertIn("arial.ttf", joined.lower())

    def test_load_font_falls_back_to_next_available_font(self) -> None:
        cfg = WatermarkConfig(text="Demo", font_name="微软雅黑")

        def fake_truetype(path: str, size: int):
            if "arial.ttf" in path.lower():
                return object()
            raise OSError("missing")

        with mock.patch("PIL.ImageFont.truetype", side_effect=fake_truetype):
            font = self.renderer._load_font(cfg)

        self.assertIsNotNone(font)


if __name__ == "__main__":
    unittest.main()
