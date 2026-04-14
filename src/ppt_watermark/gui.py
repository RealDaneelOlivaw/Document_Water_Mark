from __future__ import annotations

from pathlib import Path
import tkinter as tk
from tkinter import filedialog, messagebox, ttk

from .config import (
    ALLOWED_POSITIONS,
    DEFAULT_COLOR,
    DEFAULT_FONT_SIZE,
    DEFAULT_GAP_X_RATIO,
    DEFAULT_GAP_Y_RATIO,
    DEFAULT_MARGIN_X,
    DEFAULT_MARGIN_Y,
    DEFAULT_MIN_AREA_PCT,
    DEFAULT_OPACITY,
    DEFAULT_POSITION,
    DEFAULT_ROTATION,
    HEX_COLOR_PATTERN,
    WatermarkConfig,
)
from .pipeline import (
    LEGACY_SUFFIXES,
    SUPPORTED_SUFFIXES,
    format_stats_line,
    process_inputs,
)
from .watermark_renderer import GUI_FONT_CHOICES


DEFAULT_WATERMARK_TEXT = "超卓航科广州研究院"
CUSTOM_COLOR_LABEL = "自定义"
COLOR_PRESETS = (
    ("黑色", "#000000"),
    ("白色", "#FFFFFF"),
    ("红色", "#FF0000"),
    ("蓝色", "#1F4E79"),
    ("绿色", "#00B050"),
    ("黄色", "#FFD966"),
    ("灰色", "#808080"),
)
COLOR_PRESET_MAP = dict(COLOR_PRESETS)
COLOR_CHOICE_VALUES = tuple(name for name, _value in COLOR_PRESETS) + (CUSTOM_COLOR_LABEL,)
DEFAULT_COLOR_NAME = "黑色"


class WatermarkGuiApp:
    def __init__(self) -> None:
        self.root = tk.Tk()
        self.root.title("文档图片水印工具")
        self.root.geometry("960x600")
        self.root.minsize(880, 540)

        self.selected_files: list[Path] = []
        self.pending_files: list[Path] = []
        self.results = []
        self.is_processing = False

        self.file_panel: ttk.LabelFrame
        self.parameter_panel: ttk.LabelFrame
        self.result_panel: ttk.LabelFrame
        self.file_listbox: tk.Listbox
        self.result_text: tk.Text
        self.watermark_text: tk.Text
        self.parameter_canvas: tk.Canvas
        self.parameter_scrollbar: ttk.Scrollbar
        self.parameter_content: ttk.Frame
        self.title_label: ttk.Label
        self.subtitle_label: ttk.Label
        self.font_name_combo: ttk.Combobox
        self.color_choice_combo: ttk.Combobox
        self.custom_color_entry: ttk.Entry
        self.progress_text = tk.StringVar(value="请选择要处理的文档。")
        self.progress_value = tk.DoubleVar(value=0)

        self.font_name_var = tk.StringVar(value=GUI_FONT_CHOICES[0])
        self.font_size_var = tk.IntVar(value=DEFAULT_FONT_SIZE)
        self.opacity_var = tk.IntVar(value=DEFAULT_OPACITY)
        self.color_choice_var = tk.StringVar(value=DEFAULT_COLOR_NAME)
        self.custom_color_var = tk.StringVar(value="0,0,0")
        self.rotation_var = tk.DoubleVar(value=DEFAULT_ROTATION)
        self.position_var = tk.StringVar(value=DEFAULT_POSITION)
        self.gap_x_ratio_var = tk.DoubleVar(value=DEFAULT_GAP_X_RATIO)
        self.gap_y_ratio_var = tk.DoubleVar(value=DEFAULT_GAP_Y_RATIO)
        self.margin_x_var = tk.IntVar(value=DEFAULT_MARGIN_X)
        self.margin_y_var = tk.IntVar(value=DEFAULT_MARGIN_Y)
        self.min_area_pct_var = tk.IntVar(value=DEFAULT_MIN_AREA_PCT)
        self.pdf_mode_var = tk.StringVar(value="images")

        self.start_button: ttk.Button
        self.add_files_button: ttk.Button
        self.add_folder_button: ttk.Button
        self.remove_button: ttk.Button
        self.clear_button: ttk.Button

        self.current_config = WatermarkConfig(text=DEFAULT_WATERMARK_TEXT)
        self.current_min_area_pct = DEFAULT_MIN_AREA_PCT

        self._build_ui()

    def _build_ui(self) -> None:
        container = ttk.Frame(self.root, padding=10)
        container.pack(fill="both", expand=True)
        container.columnconfigure(0, weight=1)
        container.rowconfigure(1, weight=1)

        title_bar = ttk.Frame(container)
        title_bar.grid(row=0, column=0, sticky="w")

        self.title_label = ttk.Label(
            title_bar,
            text="文档图片水印工具V1.0",
            font=("Microsoft YaHei UI", 12, "bold"),
        )
        self.title_label.grid(row=0, column=0, sticky="w")

        self.subtitle_label = ttk.Label(
            title_bar,
            text="——Developed by Kejie Zhang, with assistance from Claude.",
            font=("Segoe UI", 8),
        )
        self.subtitle_label.grid(row=0, column=1, sticky="w", padx=(10, 0))

        content = ttk.Frame(container)
        content.grid(row=1, column=0, sticky="nsew", pady=(6, 0))
        content.columnconfigure(0, weight=6)
        content.columnconfigure(1, weight=5)
        content.rowconfigure(0, weight=3)
        content.rowconfigure(1, weight=2)

        self._build_file_panel(content)
        self._build_result_panel(content)
        self._build_parameter_panel(content)
        self._build_footer(container)

    def _build_file_panel(self, parent: ttk.Frame) -> None:
        self.file_panel = ttk.LabelFrame(parent, text="文件选择", padding=8)
        self.file_panel.grid(row=0, column=0, sticky="nsew", padx=(0, 8), pady=(0, 8))
        self.file_panel.columnconfigure(0, weight=1)
        self.file_panel.rowconfigure(1, weight=1)

        tip = ttk.Label(
            self.file_panel,
            text="支持 .pptx / .docx / .pdf，输出会写回原文档所在文件夹。",
        )
        tip.grid(row=0, column=0, sticky="w", pady=(0, 8))

        list_frame = ttk.Frame(self.file_panel)
        list_frame.grid(row=1, column=0, sticky="nsew")
        list_frame.columnconfigure(0, weight=1)
        list_frame.rowconfigure(0, weight=1)

        self.file_listbox = tk.Listbox(
            list_frame,
            selectmode=tk.EXTENDED,
            activestyle="dotbox",
            font=("Consolas", 10),
            height=8,
        )
        self.file_listbox.grid(row=0, column=0, sticky="nsew")

        scrollbar = ttk.Scrollbar(list_frame, orient="vertical", command=self.file_listbox.yview)
        scrollbar.grid(row=0, column=1, sticky="ns")
        self.file_listbox.configure(yscrollcommand=scrollbar.set)

        button_bar = ttk.Frame(self.file_panel)
        button_bar.grid(row=2, column=0, sticky="ew", pady=(8, 0))

        self.add_files_button = ttk.Button(button_bar, text="添加文件", command=self._choose_files)
        self.add_files_button.pack(side="left")

        self.add_folder_button = ttk.Button(button_bar, text="添加文件夹", command=self._choose_folder)
        self.add_folder_button.pack(side="left", padx=(8, 0))

        self.remove_button = ttk.Button(button_bar, text="移除选中", command=self._remove_selected_files)
        self.remove_button.pack(side="left", padx=(8, 0))

        self.clear_button = ttk.Button(button_bar, text="清空列表", command=self._clear_files)
        self.clear_button.pack(side="left", padx=(8, 0))

    def _build_parameter_panel(self, parent: ttk.Frame) -> None:
        self.parameter_panel = ttk.LabelFrame(parent, text="参数设置", padding=0)
        self.parameter_panel.grid(row=0, column=1, rowspan=2, sticky="nsew")
        self.parameter_panel.columnconfigure(0, weight=1)
        self.parameter_panel.rowconfigure(0, weight=1)

        canvas_frame = ttk.Frame(self.parameter_panel)
        canvas_frame.grid(row=0, column=0, sticky="nsew")
        canvas_frame.columnconfigure(0, weight=1)
        canvas_frame.rowconfigure(0, weight=1)

        self.parameter_canvas = tk.Canvas(canvas_frame, borderwidth=0, highlightthickness=0)
        self.parameter_canvas.grid(row=0, column=0, sticky="nsew")

        self.parameter_scrollbar = ttk.Scrollbar(
            canvas_frame,
            orient="vertical",
            command=self.parameter_canvas.yview,
        )
        self.parameter_scrollbar.grid(row=0, column=1, sticky="ns")
        self.parameter_canvas.configure(yscrollcommand=self.parameter_scrollbar.set)

        self.parameter_content = ttk.Frame(self.parameter_canvas, padding=8)
        window_id = self.parameter_canvas.create_window(
            (0, 0),
            window=self.parameter_content,
            anchor="nw",
        )
        self.parameter_content.columnconfigure(0, weight=1)

        self.parameter_content.bind(
            "<Configure>",
            lambda _event: self.parameter_canvas.configure(
                scrollregion=self.parameter_canvas.bbox("all")
            ),
        )
        self.parameter_canvas.bind(
            "<Configure>",
            lambda event: self.parameter_canvas.itemconfigure(window_id, width=event.width),
        )

        self._bind_canvas_mousewheel(self.parameter_canvas)

        watermark_frame = ttk.Frame(self.parameter_content)
        watermark_frame.grid(row=0, column=0, sticky="ew")
        watermark_frame.columnconfigure(0, weight=1)

        ttk.Label(watermark_frame, text="水印文字").grid(row=0, column=0, sticky="w")
        self.watermark_text = tk.Text(
            watermark_frame,
            height=5,
            width=32,
            wrap="word",
            font=("Microsoft YaHei UI", 10),
        )
        self.watermark_text.grid(row=1, column=0, sticky="ew", pady=(6, 12))
        self.watermark_text.insert("1.0", DEFAULT_WATERMARK_TEXT)

        form = ttk.Frame(self.parameter_content)
        form.grid(row=1, column=0, sticky="nsew")
        form.columnconfigure(1, weight=1)

        row = 0
        ttk.Label(form, text="字体名称").grid(row=row, column=0, sticky="w", pady=4)
        self.font_name_combo = ttk.Combobox(
            form,
            textvariable=self.font_name_var,
            values=GUI_FONT_CHOICES,
            state="readonly",
        )
        self.font_name_combo.grid(row=row, column=1, sticky="ew", pady=4)
        row += 1

        self._add_spinbox_row(form, row, "字号 (pt)", self.font_size_var, 1, 500)
        row += 1
        self._add_spinbox_row(form, row, "透明度", self.opacity_var, 0, 100)
        row += 1

        ttk.Label(form, text="颜色").grid(row=row, column=0, sticky="w", pady=4)
        color_frame = ttk.Frame(form)
        color_frame.grid(row=row, column=1, sticky="ew", pady=4)
        color_frame.columnconfigure(0, weight=2)
        color_frame.columnconfigure(1, weight=3)

        self.color_choice_combo = ttk.Combobox(
            color_frame,
            textvariable=self.color_choice_var,
            values=COLOR_CHOICE_VALUES,
            state="readonly",
        )
        self.color_choice_combo.grid(row=0, column=0, sticky="ew")
        self.color_choice_combo.bind("<<ComboboxSelected>>", self._on_color_choice_changed)

        self.custom_color_entry = ttk.Entry(
            color_frame,
            textvariable=self.custom_color_var,
        )
        self.custom_color_entry.grid(row=0, column=1, sticky="ew", padx=(8, 0))
        self._sync_custom_color_entry_state()
        row += 1

        self._add_spinbox_row(form, row, "旋转角度", self.rotation_var, -360, 360, increment=1)
        row += 1

        ttk.Label(form, text="位置模式").grid(row=row, column=0, sticky="w", pady=4)
        position_combo = ttk.Combobox(
            form,
            textvariable=self.position_var,
            values=tuple(sorted(ALLOWED_POSITIONS)),
            state="readonly",
        )
        position_combo.grid(row=row, column=1, sticky="ew", pady=4)
        row += 1

        self._add_spinbox_row(
            form,
            row,
            "横向间距倍数",
            self.gap_x_ratio_var,
            0.0,
            10.0,
            increment=0.1,
        )
        row += 1
        self._add_spinbox_row(
            form,
            row,
            "纵向间距倍数",
            self.gap_y_ratio_var,
            0.0,
            10.0,
            increment=0.1,
        )
        row += 1
        self._add_spinbox_row(form, row, "边距 X", self.margin_x_var, 0, 2000)
        row += 1
        self._add_spinbox_row(form, row, "边距 Y", self.margin_y_var, 0, 2000)
        row += 1
        self._add_spinbox_row(form, row, "小图过滤(%)", self.min_area_pct_var, 0, 100)
        row += 1

        ttk.Label(form, text="PDF 模式").grid(row=row, column=0, sticky="w", pady=4)
        pdf_combo = ttk.Combobox(
            form,
            textvariable=self.pdf_mode_var,
            values=("images", "page"),
            state="readonly",
        )
        pdf_combo.grid(row=row, column=1, sticky="ew", pady=4)
        row += 1

        note = ttk.Label(
            form,
            text=(
                "提示：字号按文档最终显示大小解释，不会因为图片分辨率或铺排密度被自动缩放。"
                "横向/纵向间距倍数按单个水印的宽高计算，例如 0.5 表示两个水印之间留半个水印宽/高的空隙。"
                "图片太小时会直接裁切，不会为了塞进更多水印而缩小文字。"
            ),
            wraplength=360,
        )
        note.grid(row=row, column=0, columnspan=2, sticky="w", pady=(8, 0))

    @staticmethod
    def _bind_canvas_mousewheel(canvas: tk.Canvas) -> None:
        def _on_mousewheel(event: tk.Event) -> str | None:
            delta = getattr(event, "delta", 0)
            if delta == 0:
                return None
            step = -1 if delta > 0 else 1
            canvas.yview_scroll(step, "units")
            return "break"

        canvas.bind("<MouseWheel>", _on_mousewheel)

    def _build_result_panel(self, parent: ttk.Frame) -> None:
        self.result_panel = ttk.LabelFrame(parent, text="处理结果", padding=8)
        self.result_panel.grid(row=1, column=0, sticky="nsew", padx=(0, 8))
        self.result_panel.columnconfigure(0, weight=1)
        self.result_panel.rowconfigure(0, weight=1)

        self.result_text = tk.Text(self.result_panel, height=4, wrap="word", state="disabled")
        self.result_text.grid(row=0, column=0, sticky="nsew")

        scrollbar = ttk.Scrollbar(self.result_panel, orient="vertical", command=self.result_text.yview)
        scrollbar.grid(row=0, column=1, sticky="ns")
        self.result_text.configure(yscrollcommand=scrollbar.set)

    def _build_footer(self, parent: ttk.Frame) -> None:
        footer = ttk.Frame(parent)
        footer.grid(row=2, column=0, sticky="ew", pady=(8, 0))
        footer.columnconfigure(0, weight=1)

        ttk.Label(footer, textvariable=self.progress_text).grid(row=0, column=0, sticky="w")

        progress = ttk.Progressbar(footer, maximum=100, variable=self.progress_value)
        progress.grid(row=1, column=0, sticky="ew", pady=(4, 8))

        action_bar = ttk.Frame(footer)
        action_bar.grid(row=2, column=0, sticky="e")

        self.start_button = ttk.Button(action_bar, text="开始处理", command=self._start_processing)
        self.start_button.pack(side="right")

    def _add_spinbox_row(
        self,
        parent: ttk.Frame,
        row: int,
        label: str,
        variable: tk.Variable,
        minimum: float,
        maximum: float,
        *,
        increment: float = 1,
    ) -> None:
        ttk.Label(parent, text=label).grid(row=row, column=0, sticky="w", pady=4)
        ttk.Spinbox(
            parent,
            textvariable=variable,
            from_=minimum,
            to=maximum,
            increment=increment,
        ).grid(row=row, column=1, sticky="ew", pady=4)

    def _choose_files(self) -> None:
        paths = filedialog.askopenfilenames(
            parent=self.root,
            title="选择要处理的文件",
            filetypes=[
                ("支持的文件", "*.pptx *.docx *.pdf"),
                ("PowerPoint", "*.pptx"),
                ("Word", "*.docx"),
                ("PDF", "*.pdf"),
            ],
        )
        if paths:
            self._append_files([Path(item) for item in paths])

    def _choose_folder(self) -> None:
        folder = filedialog.askdirectory(parent=self.root, title="选择文件夹")
        if not folder:
            return

        root_path = Path(folder)
        files = sorted(
            path
            for path in root_path.iterdir()
            if path.is_file() and path.suffix.lower() in SUPPORTED_SUFFIXES
        )
        if not files:
            messagebox.showwarning(
                "没有可处理文件",
                "所选文件夹中没有找到 .pptx / .docx / .pdf 文件。",
                parent=self.root,
            )
            return
        self._append_files(files)

    def _append_files(self, files: list[Path]) -> None:
        legacy = [path for path in files if path.suffix.lower() in LEGACY_SUFFIXES]
        invalid = [
            path
            for path in files
            if path.suffix.lower() not in SUPPORTED_SUFFIXES
            and path.suffix.lower() not in LEGACY_SUFFIXES
        ]

        valid_files = [
            path
            for path in files
            if path.suffix.lower() in SUPPORTED_SUFFIXES and path not in self.selected_files
        ]
        self.selected_files.extend(valid_files)
        self._refresh_file_list()

        notices = []
        if valid_files:
            notices.append(f"已添加 {len(valid_files)} 个文件。")
        if legacy:
            notices.append(
                "以下旧版 Office 文件不支持，请先转换后再处理：\n"
                + "\n".join(path.name for path in legacy)
            )
        if invalid:
            notices.append("以下文件格式不支持：\n" + "\n".join(path.name for path in invalid))
        if notices:
            self._append_result("\n\n".join(notices))

    def _refresh_file_list(self) -> None:
        self.file_listbox.delete(0, tk.END)
        for path in self.selected_files:
            self.file_listbox.insert(tk.END, str(path))
        self.progress_text.set(f"已选择 {len(self.selected_files)} 个文件。")

    def _remove_selected_files(self) -> None:
        indices = list(self.file_listbox.curselection())
        if not indices:
            return
        for index in reversed(indices):
            del self.selected_files[index]
        self._refresh_file_list()

    def _clear_files(self) -> None:
        self.selected_files.clear()
        self._refresh_file_list()

    def _set_processing_state(self, processing: bool) -> None:
        self.is_processing = processing
        state = "disabled" if processing else "normal"
        self.add_files_button.configure(state=state)
        self.add_folder_button.configure(state=state)
        self.remove_button.configure(state=state)
        self.clear_button.configure(state=state)
        self.watermark_text.configure(state="disabled" if processing else "normal")
        self.font_name_combo.configure(state="disabled" if processing else "readonly")
        self.color_choice_combo.configure(state="disabled" if processing else "readonly")
        if processing:
            self.custom_color_entry.configure(state="disabled")
        else:
            self._sync_custom_color_entry_state()
        self.start_button.configure(state=state)

    def _build_config(self) -> tuple[WatermarkConfig, int] | None:
        text = self.watermark_text.get("1.0", "end-1c")
        if not text.strip():
            messagebox.showerror("参数错误", "请先填写水印文字。", parent=self.root)
            return None

        try:
            config = WatermarkConfig(
                text=text,
                font_name=self.font_name_var.get().strip() or GUI_FONT_CHOICES[0],
                font_size=int(self.font_size_var.get()),
                opacity=int(self.opacity_var.get()),
                color=self._resolve_selected_color(),
                rotation=float(self.rotation_var.get()),
                position=self.position_var.get().strip() or DEFAULT_POSITION,
                gap_x_ratio=float(self.gap_x_ratio_var.get()),
                gap_y_ratio=float(self.gap_y_ratio_var.get()),
                margin_x=int(self.margin_x_var.get()),
                margin_y=int(self.margin_y_var.get()),
            )
            min_area_pct = int(self.min_area_pct_var.get())
        except Exception as exc:
            messagebox.showerror("参数错误", f"请检查参数设置：{exc}", parent=self.root)
            return None

        return config, min_area_pct

    def _start_processing(self) -> None:
        if self.is_processing:
            return
        if not self.selected_files:
            messagebox.showwarning("未选择文件", "请先添加要处理的文档。", parent=self.root)
            return

        built = self._build_config()
        if built is None:
            return

        self.current_config, self.current_min_area_pct = built
        self.pending_files = list(self.selected_files)
        self.results = []
        self.progress_value.set(0)
        self._append_result("开始处理...")
        self._set_processing_state(True)
        self.root.after(10, self._process_next_file)

    def _process_next_file(self) -> None:
        total = len(self.selected_files)
        processed = len(self.results)

        if not self.pending_files:
            self._set_processing_state(False)
            success_count = sum(1 for item in self.results if item.success)
            fail_count = len(self.results) - success_count
            summary = (
                f"处理完成。成功 {success_count} 个，失败 {fail_count} 个。\n"
                "输出文件已保存到原文档所在文件夹。"
            )
            self.progress_text.set(summary)
            messagebox.showinfo("处理完成", summary, parent=self.root)
            return

        current = self.pending_files.pop(0)
        self.progress_text.set(f"正在处理 {processed + 1}/{total}: {current.name}")
        self.root.update_idletasks()

        result = process_inputs(
            [current],
            config=self.current_config,
            output_dir=None,
            min_area_pct=self.current_min_area_pct,
            pdf_mode=self.pdf_mode_var.get(),
        )[0]
        self.results.append(result)
        self.progress_value.set((len(self.results) / max(total, 1)) * 100)
        self._append_result(format_stats_line(result))
        self.root.after(10, self._process_next_file)

    def _append_result(self, message: str) -> None:
        self.result_text.configure(state="normal")
        if self.result_text.index("end-1c") != "1.0":
            self.result_text.insert(tk.END, "\n")
        self.result_text.insert(tk.END, message)
        self.result_text.see(tk.END)
        self.result_text.configure(state="disabled")

    def _on_color_choice_changed(self, _event: tk.Event | None = None) -> None:
        self._sync_custom_color_entry_state()

    def _sync_custom_color_entry_state(self) -> None:
        is_custom = self.color_choice_var.get() == CUSTOM_COLOR_LABEL
        self.custom_color_entry.configure(state="normal" if is_custom else "disabled")

    def _resolve_selected_color(self) -> str:
        choice = self.color_choice_var.get().strip() or DEFAULT_COLOR_NAME
        if choice != CUSTOM_COLOR_LABEL:
            return COLOR_PRESET_MAP.get(choice, DEFAULT_COLOR)
        return self._normalize_custom_color(self.custom_color_var.get())

    @staticmethod
    def _normalize_custom_color(raw_value: str) -> str:
        value = raw_value.strip()
        if HEX_COLOR_PATTERN.match(value):
            return value.upper()

        normalized = value.replace("，", ",").replace(" ", ",")
        parts = [part for part in normalized.split(",") if part]
        if len(parts) != 3:
            raise ValueError("自定义颜色请输入 `R,G,B` 或 `#RRGGBB`。")

        try:
            red, green, blue = (int(part) for part in parts)
        except ValueError as exc:
            raise ValueError("自定义颜色的 RGB 值必须是 0-255 的整数。") from exc

        if any(channel < 0 or channel > 255 for channel in (red, green, blue)):
            raise ValueError("自定义颜色的 RGB 值必须在 0-255 之间。")

        return f"#{red:02X}{green:02X}{blue:02X}"

    def run(self) -> None:
        self.root.mainloop()


def main() -> None:
    WatermarkGuiApp().run()
