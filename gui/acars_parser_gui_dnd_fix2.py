"""ACARS Parser — GUI (extract) with Drag & Drop + auto output naming

Drag & drop support uses tkinterdnd2.

Install:
  pip install tkinterdnd2
"""
import os
import threading
import subprocess
import tkinter as tk
from tkinter import ttk, filedialog, messagebox

_HAS_DND = False
try:
    # IMPORTANT: TkinterDnD is a MODULE; the actual window base class is TkinterDnD.Tk
    from tkinterdnd2 import DND_FILES, TkinterDnD  # type: ignore
    _HAS_DND = True
except Exception:
    DND_FILES = None
    TkinterDnD = None  # type: ignore


def _split_dnd_files(data: str):
    """Split the string from tkinterdnd2 into file paths."""
    if not data:
        return []
    data = data.strip()
    out = []
    cur = ""
    in_brace = False
    for ch in data:
        if ch == "{":
            in_brace = True
            cur = ""
        elif ch == "}":
            in_brace = False
            if cur:
                out.append(cur)
                cur = ""
        elif ch.isspace() and not in_brace:
            if cur:
                out.append(cur)
                cur = ""
        else:
            cur += ch
    if cur:
        out.append(cur)

    # Normalize slashes (some setups deliver escaped backslashes)
    out = [p.replace('\\\\', '\\') for p in out]
    return out


BaseTk = TkinterDnD.Tk if _HAS_DND else tk.Tk  # type: ignore[attr-defined]


class App(BaseTk):
    def __init__(self):
        super().__init__()
        self.title("ACARS Parser — GUI (extract) • Drag & Drop")
        self.geometry("1050x720")

        self.exe_var = tk.StringVar(value=r".\acars_parser.exe")
        self.outdir_var = tk.StringVar(value="")  # empty => same folder as input
        self.pretty_var = tk.BooleanVar(value=True)
        self.all_var = tk.BooleanVar(value=True)
        self.stats_var = tk.BooleanVar(value=False)

        self._build_ui()

        if _HAS_DND:
            # Drop onto the whole window
            self.drop_target_register(DND_FILES)  # type: ignore[attr-defined]
            self.dnd_bind("<<Drop>>", self._on_drop)  # type: ignore[attr-defined]

    def _build_ui(self):
        frm = ttk.Frame(self, padding=10)
        frm.pack(fill="both", expand=True)

        row0 = ttk.Frame(frm); row0.pack(fill="x", pady=(0, 6))
        ttk.Label(row0, text="acars_parser executable:").pack(side="left")
        ttk.Entry(row0, textvariable=self.exe_var, width=80).pack(side="left", padx=6, fill="x", expand=True)
        ttk.Button(row0, text="Browse…", command=self.pick_exe).pack(side="left")

        row1 = ttk.Frame(frm); row1.pack(fill="x", pady=6)
        ttk.Label(row1, text="Output folder (optional):").pack(side="left")
        ttk.Entry(row1, textvariable=self.outdir_var, width=80).pack(side="left", padx=6, fill="x", expand=True)
        ttk.Button(row1, text="Browse…", command=self.pick_outdir).pack(side="left")

        row2 = ttk.Frame(frm); row2.pack(fill="x", pady=6)
        ttk.Checkbutton(row2, text="Pretty (-pretty)", variable=self.pretty_var).pack(side="left")
        ttk.Checkbutton(row2, text="All (-all)", variable=self.all_var).pack(side="left", padx=(10, 0))
        ttk.Checkbutton(row2, text="Stats (-stats) if supported", variable=self.stats_var).pack(side="left", padx=(10, 0))
        ttk.Button(row2, text="Add files…", command=self.add_files).pack(side="right")
        ttk.Button(row2, text="Run Extract (queue)", command=self.run_queue).pack(side="right", padx=(0, 8))

        hint = "Drag & drop files into this window to add them to queue." if _HAS_DND else \
               "Drag & drop disabled (install: pip install tkinterdnd2). Use 'Add files…'."
        ttk.Label(frm, text=hint).pack(anchor="w", pady=(4, 2))

        box = ttk.Frame(frm); box.pack(fill="both", expand=False, pady=(6, 6))
        ttk.Label(box, text="Queue:").pack(anchor="w")
        self.lst = tk.Listbox(box, height=7, selectmode="extended")
        self.lst.pack(fill="both", expand=True, side="left")
        sb = ttk.Scrollbar(box, orient="vertical", command=self.lst.yview)
        sb.pack(side="left", fill="y")
        self.lst.configure(yscrollcommand=sb.set)

        btncol = ttk.Frame(box); btncol.pack(side="left", fill="y", padx=(8, 0))
        ttk.Button(btncol, text="Remove selected", command=self.remove_selected).pack(fill="x", pady=(0, 6))
        ttk.Button(btncol, text="Clear", command=self.clear_queue).pack(fill="x")

        self.txt = tk.Text(frm, wrap="none")
        self.txt.pack(fill="both", expand=True, pady=(10, 0))

        xscroll = ttk.Scrollbar(frm, orient="horizontal", command=self.txt.xview)
        xscroll.pack(fill="x")
        yscroll = ttk.Scrollbar(frm, orient="vertical", command=self.txt.yview)
        yscroll.place(relx=1.0, rely=0.30, relheight=0.64, anchor="ne")
        self.txt.configure(xscrollcommand=xscroll.set, yscrollcommand=yscroll.set)

    def pick_exe(self):
        path = filedialog.askopenfilename(
            title="Select acars_parser executable",
            filetypes=[("Executable", "*.exe"), ("All", "*.*")]
        )
        if path:
            self.exe_var.set(path)

    def pick_outdir(self):
        path = filedialog.askdirectory(title="Select output folder")
        if path:
            self.outdir_var.set(path)

    def add_files(self):
        paths = filedialog.askopenfilenames(
            title="Select input files",
            filetypes=[("Logs / JSONL", "*.jsonl;*.log;*.txt;*.json"), ("All", "*.*")]
        )
        if paths:
            self._add_to_queue(list(paths))

    def _on_drop(self, event):
        paths = _split_dnd_files(getattr(event, "data", ""))
        if paths:
            self._add_to_queue(paths)

    def _add_to_queue(self, paths):
        existing = set(self.lst.get(0, "end"))
        for p in paths:
            p = p.strip()
            if not p:
                continue
            if not os.path.exists(p):
                self._append(f"[WARN] Not found: {p}\n")
                continue
            if p in existing:
                continue
            self.lst.insert("end", p)
            existing.add(p)

    def remove_selected(self):
        sel = list(self.lst.curselection())
        sel.reverse()
        for i in sel:
            self.lst.delete(i)

    def clear_queue(self):
        self.lst.delete(0, "end")

    def _output_path_for(self, inp: str) -> str:
        stem, _ext = os.path.splitext(os.path.basename(inp))
        outname = stem + ".json"
        outdir = self.outdir_var.get().strip()
        return os.path.join(outdir if outdir else os.path.dirname(inp), outname)

    def run_queue(self):
        exe = self.exe_var.get().strip()
        if not exe:
            messagebox.showerror("Missing", "Select acars_parser executable.")
            return
        if not os.path.exists(exe):
            messagebox.showerror("Not found", f"Executable not found:\n{exe}")
            return

        items = list(self.lst.get(0, "end"))
        if not items:
            messagebox.showinfo("Queue empty", "Add one or more input files first.")
            return

        self.txt.delete("1.0", "end")
        self._append(f"Queue length: {len(items)}\n\n")

        def worker():
            creationflags = 0
            if os.name == "nt":
                creationflags = getattr(subprocess, "CREATE_NO_WINDOW", 0)

            for idx, inp in enumerate(items, 1):
                outp = self._output_path_for(inp)

                args = [exe, "extract", "-input", inp, "-output", outp]
                if self.pretty_var.get():
                    args.append("-pretty")
                if self.all_var.get():
                    args.append("-all")
                if self.stats_var.get():
                    args.append("-stats")

                self._append(f"\n[{idx}/{len(items)}] Running:\n  " + " ".join(args) + "\n")

                try:
                    p = subprocess.Popen(
                        args,
                        stdout=subprocess.PIPE,
                        stderr=subprocess.STDOUT,
                        text=True,
                        creationflags=creationflags,
                    )
                    assert p.stdout is not None
                    combined = ""
                    for line in p.stdout:
                        combined += line
                        self._append(line)
                    rc = p.wait()

                    if rc != 0 and self.stats_var.get() and ("flag provided but not defined: -stats" in combined):
                        self._append("\n[INFO] This build does not support -stats. Retrying without -stats...\n")
                        args2 = [a for a in args if a != "-stats"]
                        self._append("  " + " ".join(args2) + "\n")
                        p2 = subprocess.Popen(
                            args2,
                            stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT,
                            text=True,
                            creationflags=creationflags,
                        )
                        assert p2.stdout is not None
                        for line in p2.stdout:
                            self._append(line)
                        rc = p2.wait()

                    if rc == 0 and os.path.exists(outp):
                        self._append(f"[OK] Output: {outp}\n")
                    else:
                        self._append(f"[FAIL] Exit code: {rc}\n")
                        self._append(f"       Expected output: {outp}\n")

                except Exception as e:
                    self._append(f"[ERROR] {e}\n")

            self._append("\nDone.\n")

        threading.Thread(target=worker, daemon=True).start()

    def _append(self, s: str):
        def _do():
            self.txt.insert("end", s)
            self.txt.see("end")
        self.after(0, _do)


if __name__ == "__main__":
    App().mainloop()
