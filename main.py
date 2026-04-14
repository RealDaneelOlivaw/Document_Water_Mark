from __future__ import annotations


def _load_main() -> object:
    try:
        from ppt_watermark.gui import main

        return main
    except ModuleNotFoundError:
        import sys
        from pathlib import Path

        project_root = Path(__file__).resolve().parent
        src_dir = project_root / "src"
        if str(src_dir) not in sys.path:
            sys.path.insert(0, str(src_dir))
        from ppt_watermark.gui import main

        return main


if __name__ == "__main__":
    _load_main()()
