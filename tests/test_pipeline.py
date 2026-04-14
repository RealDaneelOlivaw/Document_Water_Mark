import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

from ppt_watermark.pipeline import collect_input_files, create_processor, resolve_output_path


class PipelineTests(unittest.TestCase):
    def test_collect_input_files_supports_multiple_suffixes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            (root / "deck.pptx").write_bytes(b"x")
            (root / "doc.docx").write_bytes(b"y")
            (root / "sheet.pdf").write_bytes(b"z")
            (root / "ignore.txt").write_text("n", encoding="utf-8")

            files = collect_input_files(root)
            self.assertEqual([file.name for file in files], ["deck.pptx", "doc.docx", "sheet.pdf"])

    def test_resolve_output_path_avoids_existing_and_reserved_conflicts(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            source = root / "report.pptx"
            source.write_bytes(b"x")
            existing = root / "report_watermarked.pptx"
            existing.write_bytes(b"done")
            reserved = {root / "report_watermarked_2.pptx"}

            resolved = resolve_output_path(source, output_dir=root, reserved_paths=reserved)
            self.assertEqual(resolved.name, "report_watermarked_3.pptx")

    def test_create_processor_imports_only_requested_suffix_module(self) -> None:
        class FakeWordProcessor:
            def __init__(self, min_area_pct: int = 0) -> None:
                self.min_area_pct = min_area_pct

        fake_module = SimpleNamespace(WordWatermarkProcessor=FakeWordProcessor)
        with mock.patch("ppt_watermark.pipeline.import_module", return_value=fake_module) as mocked_import:
            processor = create_processor(".docx", min_area_pct=7)

        self.assertEqual(processor.min_area_pct, 7)
        mocked_import.assert_called_once_with(".word_processor", package="ppt_watermark")

    def test_create_processor_reports_missing_pdf_dependency_clearly(self) -> None:
        with mock.patch(
            "ppt_watermark.pipeline.import_module",
            side_effect=ModuleNotFoundError("No module named 'frontend'"),
        ):
            with self.assertRaisesRegex(RuntimeError, "PyMuPDF"):
                create_processor(".pdf")


if __name__ == "__main__":
    unittest.main()
