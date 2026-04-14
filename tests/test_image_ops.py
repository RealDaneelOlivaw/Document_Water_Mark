import unittest

from PIL import Image

from ppt_watermark.config import WatermarkConfig
from ppt_watermark.image_ops import (
    build_overlay_cache_key,
    build_rendered_blob_key,
    compute_render_metrics,
    serialize_composited_image,
)


class ImageOpsTests(unittest.TestCase):
    def test_overlay_cache_key_changes_with_gap_ratio(self) -> None:
        config = WatermarkConfig(text="Demo")
        config_b = WatermarkConfig(text="Demo", gap_y_ratio=1.25)
        key_a = build_overlay_cache_key(
            config,
            width=640,
            height=360,
        )
        key_b = build_overlay_cache_key(
            config_b,
            width=640,
            height=360,
        )
        self.assertNotEqual(key_a, key_b)

    def test_rendered_blob_key_changes_with_overlay_key(self) -> None:
        config = WatermarkConfig(text="Demo")
        config_b = WatermarkConfig(text="Demo", gap_x_ratio=1.25)
        overlay_a = build_overlay_cache_key(
            config,
            width=640,
            height=360,
        )
        overlay_b = build_overlay_cache_key(
            config_b,
            width=640,
            height=360,
        )
        key_a = build_rendered_blob_key("blob", "image/png", overlay_a)
        key_b = build_rendered_blob_key("blob", "image/png", overlay_b)
        self.assertNotEqual(key_a, key_b)

    def test_compute_render_metrics_keeps_font_visible_size_constant(self) -> None:
        config = WatermarkConfig(text="Demo", font_size=36)
        large_display = compute_render_metrics(
            config,
            image_px_width=1200,
            image_px_height=600,
            display_width_units=914400 * 4,
            display_height_units=914400 * 2,
            units_per_inch=914400,
        )
        small_display = compute_render_metrics(
            config,
            image_px_width=1200,
            image_px_height=600,
            display_width_units=914400 * 2,
            display_height_units=914400,
            units_per_inch=914400,
        )
        self.assertGreater(small_display.font_size_px, large_display.font_size_px)
        self.assertEqual(small_display.font_size_px, large_display.font_size_px * 2)

    def test_serialize_composited_image_keeps_gif_content_type(self) -> None:
        image = Image.new("RGBA", (16, 16), (255, 0, 0, 128))
        blob, content_type = serialize_composited_image(image, "image/gif", has_alpha=True)
        self.assertEqual(content_type, "image/gif")
        self.assertTrue(blob.startswith(b"GIF"))


if __name__ == "__main__":
    unittest.main()
