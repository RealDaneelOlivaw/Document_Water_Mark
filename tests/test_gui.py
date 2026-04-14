import unittest
from unittest import mock

from ppt_watermark.config import (
    DEFAULT_FONT_SIZE,
    DEFAULT_GAP_X_RATIO,
    DEFAULT_GAP_Y_RATIO,
    DEFAULT_MARGIN_X,
    DEFAULT_MARGIN_Y,
)
from ppt_watermark.gui import (
    COLOR_CHOICE_VALUES,
    CUSTOM_COLOR_LABEL,
    DEFAULT_COLOR_NAME,
    DEFAULT_WATERMARK_TEXT,
    WatermarkGuiApp,
)
from ppt_watermark.watermark_renderer import GUI_FONT_CHOICES


class GuiTests(unittest.TestCase):
    def test_gui_uses_expected_defaults_and_layout(self) -> None:
        app = WatermarkGuiApp()
        try:
            app.root.update_idletasks()
            self.assertEqual(app.font_name_var.get(), GUI_FONT_CHOICES[0])
            self.assertEqual(app.font_size_var.get(), DEFAULT_FONT_SIZE)
            self.assertEqual(app.gap_x_ratio_var.get(), DEFAULT_GAP_X_RATIO)
            self.assertEqual(app.gap_y_ratio_var.get(), DEFAULT_GAP_Y_RATIO)
            self.assertEqual(app.margin_x_var.get(), DEFAULT_MARGIN_X)
            self.assertEqual(app.margin_y_var.get(), DEFAULT_MARGIN_Y)
            self.assertEqual(app.watermark_text.get("1.0", "end-1c"), DEFAULT_WATERMARK_TEXT)
            self.assertEqual(app.title_label.cget("text"), "文档图片水印工具V1.0")
            self.assertIn("Kejie Zhang", app.subtitle_label.cget("text"))
            self.assertEqual(int(float(app.root.winfo_width())), 960)
            self.assertEqual(str(app.font_name_combo.cget("state")), "readonly")
            self.assertEqual(str(app.color_choice_combo.cget("state")), "readonly")
            self.assertEqual(app.color_choice_var.get(), DEFAULT_COLOR_NAME)
            self.assertEqual(DEFAULT_COLOR_NAME, "黑色")
            self.assertIn(CUSTOM_COLOR_LABEL, app.color_choice_combo.cget("values"))
            self.assertEqual(app.color_choice_combo.cget("values"), COLOR_CHOICE_VALUES)
            self.assertEqual(str(app.custom_color_entry.cget("state")), "disabled")
            self.assertEqual(app.watermark_text.winfo_manager(), "grid")
            self.assertEqual(app.parameter_scrollbar.winfo_manager(), "grid")
            self.assertEqual(int(app.parameter_panel.grid_info()["rowspan"]), 2)
            self.assertEqual(int(app.result_panel.grid_info()["row"]), 1)
            self.assertEqual(int(app.file_panel.grid_info()["column"]), 0)
            self.assertEqual(int(app.parameter_panel.grid_info()["column"]), 1)
        finally:
            app.root.destroy()

    def test_build_config_requires_watermark_text(self) -> None:
        app = WatermarkGuiApp()
        try:
            app.watermark_text.delete("1.0", "end")
            with mock.patch("tkinter.messagebox.showerror") as showerror:
                result = app._build_config()
            self.assertIsNone(result)
            showerror.assert_called_once()
            self.assertIn("请先填写水印文字", showerror.call_args.args[1])
        finally:
            app.root.destroy()

    def test_build_config_accepts_custom_rgb_color(self) -> None:
        app = WatermarkGuiApp()
        try:
            app.color_choice_var.set(CUSTOM_COLOR_LABEL)
            app._on_color_choice_changed()
            app.custom_color_var.set("12, 34, 56")

            built = app._build_config()

            self.assertIsNotNone(built)
            config, _min_area_pct = built
            self.assertEqual(config.color, "#0C2238")
            self.assertEqual(config.gap_x_ratio, DEFAULT_GAP_X_RATIO)
            self.assertEqual(config.gap_y_ratio, DEFAULT_GAP_Y_RATIO)
            self.assertEqual(str(app.custom_color_entry.cget("state")), "normal")
        finally:
            app.root.destroy()


if __name__ == "__main__":
    unittest.main()
