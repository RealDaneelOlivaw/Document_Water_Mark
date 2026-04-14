import tempfile
import unittest
from pathlib import Path

from ppt_watermark.cli import collect_input_files, parse_args, run


class CliTests(unittest.TestCase):
    def test_parse_args_for_single_file(self) -> None:
        args = parse_args(
            [
                "--input",
                "a.pptx",
                "--text",
                "CONFIDENTIAL",
                "--font-size",
                "30",
                "--opacity",
                "35",
            ]
        )
        self.assertEqual(args.input, "a.pptx")
        self.assertEqual(args.text, "CONFIDENTIAL")
        self.assertEqual(args.font_size, 30)
        self.assertEqual(args.opacity, 35)

    def test_collect_input_files_from_directory(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            (root / "a.pptx").write_bytes(b"x")
            (root / "b.pptx").write_bytes(b"y")
            (root / "c.txt").write_text("ignore", encoding="utf-8")

            files = collect_input_files(str(root))
            self.assertEqual([f.name for f in files], ["a.pptx", "b.pptx"])

    def test_collect_input_files_rejects_non_pptx(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            file_path = Path(tmp_dir) / "not_ppt.txt"
            file_path.write_text("x", encoding="utf-8")
            with self.assertRaises(ValueError):
                collect_input_files(str(file_path))

    def test_run_returns_nonzero_for_invalid_path(self) -> None:
        exit_code = run(["--input", "missing_path", "--text", "A"])
        self.assertEqual(exit_code, 1)


if __name__ == "__main__":
    unittest.main()
