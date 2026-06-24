#!/usr/bin/env python3
"""Generate academic graphs and raw data table from e2e benchmark results.

Scans bench-results/run-N/<commit>/ directories for pprof files,
extracts performance metrics, and generates:
  - bench_raw_data.csv: one row per run × commit
  - bench_summary.txt: aggregated statistics table with methodology + improvement
  - bench_graphs.png / .pdf: 2×2 bar charts with error bars
"""

from __future__ import annotations

import argparse
import csv
import re
import subprocess
import sys
from collections import defaultdict
from datetime import datetime
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np

# ── pprof extraction ──────────────────────────────────────────────────────────


def _run_pprof(extra_args: list[str]) -> str:
    """Run go tool pprof and return combined stdout+stderr output."""
    result = subprocess.run(
        ["go", "tool", "pprof", "-text", "-nodecount=0", *extra_args],
        capture_output=True,
        text=True,
    )
    return result.stdout + result.stderr


def extract_duration(cpu_prof: Path) -> float | None:
    """Extract wall-clock duration (seconds) from cpu.prof."""
    output = _run_pprof([str(cpu_prof)])
    m = re.search(r"Duration:\s*([\d.]+)s", output)
    return float(m.group(1)) if m else None


def extract_cpu_samples(cpu_prof: Path) -> float | None:
    """Extract total CPU samples (seconds) from cpu.prof."""
    output = _run_pprof([str(cpu_prof)])
    m = re.search(r"Total samples\s*=\s*([\d.]+)s", output)
    return float(m.group(1)) if m else None


def extract_mem_total(mem_prof: Path) -> float | None:
    """Extract total alloc (MB) from mem.prof using -alloc_space."""
    output = _run_pprof(["-alloc_space", str(mem_prof)])
    m = re.search(r"([\d,.]+)\s*([kMG])B\s+total", output)
    if not m:
        return None
    num = float(m.group(1).replace(",", ""))
    unit = m.group(2)
    if unit == "k":
        return num / 1024
    elif unit == "M":
        return num
    elif unit == "G":
        return num * 1024
    return num


def extract_alloc_objects(mem_prof: Path) -> int | None:
    """Extract total allocated objects from mem.prof using -alloc_objects."""
    output = _run_pprof(["-alloc_objects", str(mem_prof)])
    # Line: "Showing nodes accounting for 349952, 98.13% of 356623 total"
    m = re.search(r"of\s+([\d,]+)\s+total", output)
    if not m:
        return None
    return int(m.group(1).replace(",", ""))


# ── label parsing ─────────────────────────────────────────────────────────────


def parse_labels(labels_str: str, head_commit: str | None) -> dict[str, str]:
    """Parse comma-separated sha=label pairs into a dict.

    The special token "HEAD" in the sha position resolves to the HEAD commit.
    """
    labels: dict[str, str] = {}
    if not labels_str:
        return labels
    for pair in labels_str.split(","):
        pair = pair.strip()
        if "=" not in pair:
            continue
        sha, label = pair.split("=", 1)
        sha = sha.strip()
        label = label.strip()
        if sha.upper() == "HEAD" and head_commit:
            sha = head_commit
        labels[sha] = label
    return labels


# ── directory scanning ───────────────────────────────────────────────────────


def scan_bench_dir(
    bench_dir: Path,
    head_commit: str | None,
    labels: dict[str, str],
) -> tuple[list[dict], list[str]]:
    """Scan bench-results directory and extract metrics for all runs/commits.

    Returns (results, commits) where results is a list of metric dicts and
    commits is the ordered list of full commit hashes (discovery order preserved).
    """
    run_dirs = sorted(
        bench_dir.glob("run-*"),
        key=lambda d: int(d.name.split("-")[1]),
    )
    if not run_dirs:
        return [], []

    # Determine commit order from first run (preserve discovery order, no sort)
    first_run = run_dirs[0]
    commits = [d.name for d in first_run.iterdir() if d.is_dir()]

    results: list[dict] = []
    for run_dir in run_dirs:
        run_id = int(run_dir.name.split("-")[1])
        for commit in commits:
            commit_dir = run_dir / commit
            if not commit_dir.is_dir():
                continue
            cpu_prof = commit_dir / "cpu.prof"
            mem_prof = commit_dir / "mem.prof"
            if not cpu_prof.exists() or not mem_prof.exists():
                print(f"  WARN: missing profiles in {commit_dir}", file=sys.stderr)
                continue

            duration = extract_duration(cpu_prof)
            cpu_samples = extract_cpu_samples(cpu_prof)
            cpu_util = (
                (cpu_samples / duration * 100) if duration and cpu_samples else None
            )
            alloc_mb = extract_mem_total(mem_prof)
            alloc_objects = extract_alloc_objects(mem_prof)

            results.append(
                {
                    "run": run_id,
                    "commit": commit,
                    "short_commit": commit[:7],
                    "label": labels.get(commit, commit[:7]),
                    "duration_s": duration,
                    "cpu_samples_s": cpu_samples,
                    "cpu_util_pct": cpu_util,
                    "total_alloc_mb": alloc_mb,
                    "total_alloc_objects": alloc_objects,
                }
            )

    return results, commits


# ── output generation ─────────────────────────────────────────────────────────


def _commit_labels(
    commits: list[str], head_commit: str | None, labels: dict[str, str]
) -> list[str]:
    out = []
    for c in commits:
        if c in labels:
            out.append(labels[c])
        elif c == head_commit:
            out.append(f"HEAD ({c[:7]})")
        else:
            out.append(c[:7])
    return out


def generate_csv(results: list[dict], output_path: Path) -> None:
    fieldnames = [
        "run",
        "commit",
        "short_commit",
        "label",
        "duration_s",
        "cpu_samples_s",
        "cpu_util_pct",
        "total_alloc_mb",
        "total_alloc_objects",
    ]
    with open(output_path, "w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(results)


def _fmt_improvement(
    baseline: float, current: float, lower_is_better: bool = True
) -> str:
    """Format improvement of current vs baseline.

    For lower-is-better metrics (duration, cpu, alloc): negative % = improvement.
    For higher-is-better: positive % = improvement.
    Returns string like "-12.3%" or "+5.0%" or "1.23×" for multipliers.
    """
    if baseline == 0 or baseline is None or current is None:
        return "N/A"
    pct = ((current - baseline) / baseline) * 100
    sign = "+" if pct >= 0 else ""
    improved = (pct < 0) if lower_is_better else (pct > 0)
    marker = " ↓" if improved else " ↑"
    return f"{sign}{pct:.1f}%{marker}"


def generate_summary(
    results: list[dict],
    commits: list[str],
    head_commit: str | None,
    labels: dict[str, str],
    runs_per_commit: int,
    output_path: Path,
) -> None:
    by_commit: dict[str, list[dict]] = defaultdict(list)
    for r in results:
        by_commit[r["commit"]].append(r)

    multi = runs_per_commit > 1
    display_labels = _commit_labels(commits, head_commit, labels)

    # Lower is better for all five metrics
    # (key, display_name, is_integer)
    metrics = [
        ("duration_s", "Duration (s)", False),
        ("cpu_samples_s", "CPU Samples (s)", False),
        ("cpu_util_pct", "CPU Util (%)", False),
        ("total_alloc_mb", "Total Alloc (MB)", False),
        ("total_alloc_objects", "Total Alloc (objects)", True),
    ]

    metric_w = max(len(m[1]) for m in metrics) + 2
    col_w = max(22, max(len(l) for l in display_labels) + 4)
    improvement_w = (
        max(16, max(len("Δ " + l) for l in display_labels[1:]) + 4)
        if len(display_labels) > 1
        else 16
    )

    lines: list[str] = []

    # ── Methodology section ──
    lines.append("=" * 90)
    lines.append("PANIX E2E BENCHMARK REPORT")
    lines.append("=" * 90)
    lines.append("")
    lines.append("METHODOLOGY")
    lines.append("-" * 40)
    lines.append(f"  Date:         {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    lines.append(f"  Runs/commit:  {runs_per_commit}")
    lines.append(f"  Commits:      {len(commits)}")
    lines.append(f"  Test scope:   both (local + remote)")
    lines.append("")
    lines.append("  For each commit:")
    lines.append("    1. Checkout commit in isolated git worktree")
    lines.append("    2. Build panix binary via nix build")
    lines.append(
        "    3. Run e2e test (QEMU VMs: bootstrap + redeploy (redeploy is not measured))"
    )
    lines.append("    4. Collect pprof CPU/memory profiles")
    lines.append("")
    lines.append("  Metrics extracted from pprof:")
    lines.append("    - Duration (s):          Wall-clock time of e2e run (from cpu.prof)")
    lines.append("    - CPU Samples (s):       Total CPU sampling time (from cpu.prof)")
    lines.append("    - CPU Util (%):          CPU samples / wall duration × 100")
    lines.append("    - Total Alloc (MB):      Heap allocation bytes (from mem.prof -alloc_space)")
    lines.append("    - Total Alloc (objects): Heap allocation count (from mem.prof -alloc_objects)")
    lines.append("")
    lines.append("  Baseline: first commit in the list (improvement shown as Δ%)")
    lines.append("    ↓ = improvement (lower is better), ↑ = regression")
    lines.append("")
    lines.append("=" * 90)
    lines.append("")

    # ── Commits table ──
    label_col_w = max(20, max(len(l) for l in display_labels) + 2) if display_labels else 20
    lines.append("COMMITS")
    lines.append("-" * 40)
    for i, commit in enumerate(commits):
        role = "baseline" if i == 0 else ""
        if commit == head_commit:
            role = "HEAD" if not role else f"{role}, HEAD"
        lines.append(f"  {display_labels[i]:<{label_col_w}} {commit[:7]}  {role}")
    lines.append("")

    # ── Mean ± Std table ──
    lines.append("RESULTS (mean ± std)" if multi else "RESULTS")
    lines.append(
        "-" * (metric_w + col_w * len(commits) + improvement_w * (len(commits) - 1))
    )

    header = f"{'Metric':<{metric_w}}"
    for label in display_labels:
        header += f"{label:>{col_w}}"
    for label in display_labels[1:]:
        header += f"{'Δ ' + label:>{improvement_w}}"
    lines.append(header)
    lines.append("-" * len(header))

    baseline_commit = commits[0]

    for key, name, is_int in metrics:
        row = f"{name:<{metric_w}}"
        baseline_vals = [
            r[key] for r in by_commit[baseline_commit] if r[key] is not None
        ]
        baseline_mean = float(np.mean(baseline_vals)) if baseline_vals else None

        for commit in commits:
            vals = [r[key] for r in by_commit[commit] if r[key] is not None]
            if not vals:
                cell = "N/A"
            elif multi:
                mean = float(np.mean(vals))
                std = float(np.std(vals, ddof=1)) if len(vals) > 1 else 0.0
                if is_int:
                    cell = f"{mean:,.0f} ± {std:,.0f}"
                else:
                    cell = f"{mean:.2f} ± {std:.2f}"
            else:
                if is_int:
                    cell = f"{vals[0]:,.0f}"
                else:
                    cell = f"{vals[0]:.2f}"
            row += f"{cell:>{col_w}}"

        for commit in commits[1:]:
            vals = [r[key] for r in by_commit[commit] if r[key] is not None]
            if not vals or baseline_mean is None:
                imp = "N/A"
            else:
                current_mean = float(np.mean(vals))
                imp = _fmt_improvement(
                    baseline_mean, current_mean, lower_is_better=True
                )
            row += f"{imp:>{improvement_w}}"

        lines.append(row)

    # ── Min / Max table (multi-run only) ──
    if multi:
        lines.append("")
        lines.append("[min, max] per commit:")
        lines.append("-" * len(header))
        for key, name, is_int in metrics:
            row = f"{name:<{metric_w}}"
            for commit in commits:
                vals = [r[key] for r in by_commit[commit] if r[key] is not None]
                if not vals:
                    cell = "N/A"
                elif is_int:
                    cell = f"[{min(vals):,.0f}, {max(vals):,.0f}]"
                else:
                    cell = f"[{min(vals):.2f}, {max(vals):.2f}]"
                row += f"{cell:>{col_w}}"
            lines.append(row)

    lines.append("")
    lines.append(f"Raw data: {output_path.parent / 'bench_raw_data.csv'}")

    with open(output_path, "w") as f:
        f.write("\n".join(lines) + "\n")


def generate_graphs(
    results: list[dict],
    commits: list[str],
    head_commit: str | None,
    labels: dict[str, str],
    runs_per_commit: int,
    output_path: Path,
) -> None:
    by_commit: dict[str, list[dict]] = defaultdict(list)
    for r in results:
        by_commit[r["commit"]].append(r)

    multi = runs_per_commit > 1
    display_labels = _commit_labels(commits, head_commit, labels)

    # (key, title, is_integer)
    metrics = [
        ("duration_s", "Wall Duration (s)", False),
        ("cpu_samples_s", "CPU Samples (s)", False),
        ("cpu_util_pct", "CPU Utilization (%)", False),
        ("total_alloc_mb", "Total Alloc (MB)", False),
        ("total_alloc_objects", "Total Alloc (objects)", True),
    ]

    # Colorblind-friendly palette (Paul Tol / Okabe-Ito)
    colors = ["#4477AA", "#EE6677", "#228833", "#CCBB44", "#66CCEE", "#AA3377"]

    n_metrics = len(metrics)
    n_cols = 3
    n_rows = (n_metrics + n_cols - 1) // n_cols
    fig, axes = plt.subplots(n_rows, n_cols, figsize=(18, 5 * n_rows))
    axes_flat = axes.flatten()

    x = np.arange(len(commits))
    rng = np.random.default_rng(seed=42)

    # Determine if labels need rotation based on max label length
    max_label_len = max(len(l) for l in display_labels) if display_labels else 0
    rotate_labels = max_label_len > 12

    for idx, (key, title, is_int) in enumerate(metrics):
        ax = axes_flat[idx]

        means: list[float] = []
        stds: list[float] = []
        all_vals: list[list[float]] = []

        for commit in commits:
            vals = [r[key] for r in by_commit[commit] if r[key] is not None]
            all_vals.append(vals)
            if vals:
                means.append(float(np.mean(vals)))
                stds.append(float(np.std(vals, ddof=1)) if len(vals) > 1 else 0.0)
            else:
                means.append(0.0)
                stds.append(0.0)

        bar_colors = [colors[i % len(colors)] for i in range(len(commits))]

        if multi:
            ax.bar(
                x,
                means,
                yerr=stds,
                capsize=5,
                color=bar_colors,
                edgecolor="black",
                linewidth=0.8,
                error_kw={"linewidth": 1.2, "ecolor": "#333333"},
            )
        else:
            ax.bar(
                x,
                means,
                color=bar_colors,
                edgecolor="black",
                linewidth=0.8,
            )

        # Value labels above bars / error bars
        y_max = max(means) if means else 1
        for i, (mean, std) in enumerate(zip(means, stds)):
            y = mean + (std if multi else 0) + y_max * 0.02
            if is_int:
                label_text = f"{mean:,.0f}"
            else:
                label_text = f"{mean:.2f}"
            ax.text(
                i,
                y,
                label_text,
                ha="center",
                va="bottom",
                fontsize=8,
                color="#333333",
            )

        ax.set_xticks(x)
        if rotate_labels:
            ax.set_xticklabels(display_labels, fontsize=9, rotation=20, ha="right")
        else:
            ax.set_xticklabels(display_labels, fontsize=9)
        ax.set_title(title, fontsize=11, fontweight="bold")
        ax.grid(axis="y", alpha=0.3, linestyle="--")
        ax.spines["top"].set_visible(False)
        ax.spines["right"].set_visible(False)

        # Add headroom for labels
        y_top = y_max * 1.15 if multi else y_max * 1.08
        ax.set_ylim(0, y_top)

    # Hide unused subplots
    for idx in range(n_metrics, len(axes_flat)):
        axes_flat[idx].set_visible(False)

    fig.suptitle(
        f"Panix E2E Benchmark — {runs_per_commit} run(s) per commit",
        fontsize=13,
        fontweight="bold",
        y=1.01,
    )
    fig.tight_layout()
    fig.savefig(output_path, dpi=300, bbox_inches="tight")
    pdf_path = output_path.with_suffix(".pdf")
    fig.savefig(pdf_path, bbox_inches="tight")
    plt.close(fig)


# ── main ───────────────────────────────────────────────────────────────────────


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Generate graphs and data table from e2e benchmark results."
    )
    parser.add_argument(
        "--bench-dir",
        required=True,
        help="Path to bench-results directory (contains run-N/<commit>/ profiles)",
    )
    parser.add_argument(
        "--head-commit",
        default="",
        help="Full SHA of HEAD commit (labeled differently in output)",
    )
    parser.add_argument(
        "--labels",
        default="",
        help='Comma-separated sha=label pairs (e.g. "abc123=My Label,HEAD=Current")',
    )
    args = parser.parse_args()

    bench_dir = Path(args.bench_dir)
    head_commit = args.head_commit or None
    labels = parse_labels(args.labels, head_commit)

    if not bench_dir.is_dir():
        print(f"ERROR: bench dir not found: {bench_dir}", file=sys.stderr)
        sys.exit(1)

    results, commits = scan_bench_dir(bench_dir, head_commit, labels)
    if not results:
        print(
            f"ERROR: no benchmark results found in {bench_dir}",
            file=sys.stderr,
        )
        sys.exit(1)

    # Count runs per commit
    by_commit: dict[str, list[dict]] = defaultdict(list)
    for r in results:
        by_commit[r["commit"]].append(r)
    runs_per_commit = max(len(v) for v in by_commit.values()) if by_commit else 0

    print(f"  Found {len(results)} data points across {len(commits)} commits")
    print("  Extracting metrics...")

    csv_path = bench_dir / "bench_raw_data.csv"
    summary_path = bench_dir / "bench_summary.txt"
    graph_path = bench_dir / "bench_graphs.png"

    generate_csv(results, csv_path)
    print(f"  ✓ {csv_path}")

    generate_summary(
        results, commits, head_commit, labels, runs_per_commit, summary_path
    )
    print(f"  ✓ {summary_path}")

    generate_graphs(results, commits, head_commit, labels, runs_per_commit, graph_path)
    print(f"  ✓ {graph_path}")
    print(f"  ✓ {graph_path.with_suffix('.pdf')}")


if __name__ == "__main__":
    main()
