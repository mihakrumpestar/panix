#!/usr/bin/env bash
# bench-e2e-tui.sh — Benchmark bubbletea vs zeroterm TUI via e2e tests with profiling.
#
# Output: ./tests/e2e/bench-results/<commit>/{bubbletea,zeroterm}/*.prof

set -euo pipefail

# ── Configuration ──

BASELINE_COMMIT="ab5cd6e970666ecf071c9375c236742924464573"
HEAD_COMMIT="$(git rev-parse HEAD)"
WORKTREE="/tmp/panix-bubbletea-worktree"
BUBBLETEA_BIN="/tmp/panix-bubbletea-bin"
ZEROTERM_BIN="/tmp/panix-zeroterm-bin"
OUTDIR="./tests/e2e/bench-results/${HEAD_COMMIT}"

echo "=== Benchmark: bubbletea vs zeroterm ==="
echo "Baseline:  ${BASELINE_COMMIT} ($(git log --oneline -1 "${BASELINE_COMMIT}"))"
echo "HEAD:      ${HEAD_COMMIT} ($(git log --oneline -1 HEAD))"
echo "Output:    ${OUTDIR}"
echo ""

# ── Check dependencies ──

for dep in qemu-system-x86_64 qemu-img nix curl genisoimage konsole; do
    if ! command -v "$dep" >/dev/null 2>&1; then
        echo "ERROR: required dependency '$dep' not found in PATH"
        exit 1
    fi
done

if [ ! -e /dev/kvm ]; then
    echo "ERROR: /dev/kvm not available"
    exit 1
fi

if [ -z "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ]; then
    echo "ERROR: no display server (DISPLAY or WAYLAND_DISPLAY required for konsole)"
    exit 1
fi

# ── Activate devbox (provides harmonia-cache) ──

echo ">>> Activating devbox environment..."
eval "$(devbox shell --print-env 2>/dev/null)"

if ! command -v harmonia-cache >/dev/null 2>&1; then
    echo "ERROR: harmonia-cache not in PATH (install via devbox)"
    exit 1
fi

# ── Build baseline panix from worktree ──

echo ""
echo ">>> Building baseline panix (${BASELINE_COMMIT:0:7})..."
if [ -d "$WORKTREE" ]; then
    git worktree remove --force "$WORKTREE" 2>/dev/null || true
fi
git worktree add "$WORKTREE" "$BASELINE_COMMIT"

pushd "$WORKTREE" > /dev/null
GOWORK=off nix build .#default --extra-experimental-features "nix-command flakes"
cp result/bin/panix "$BUBBLETEA_BIN"
popd > /dev/null

echo "    Built: $BUBBLETEA_BIN ($(sha256sum "$BUBBLETEA_BIN" | cut -c1-12))"

# ── Build current panix (HEAD) ──

echo ""
echo ">>> Building current panix (${HEAD_COMMIT:0:7})..."
GOWORK=off nix build .#default --extra-experimental-features "nix-command flakes"
cp result/bin/panix "$ZEROTERM_BIN"

echo "    Built: $ZEROTERM_BIN ($(sha256sum "$ZEROTERM_BIN" | cut -c1-12))"

# ── Run e2e for bubbletea ──

echo ""
echo ">>> Running e2e test with BUBBLETEA panix..."
echo "    (konsole will open — this is the TUI under test)"
mkdir -p "${OUTDIR}/bubbletea"

PANIX_BIN="$BUBBLETEA_BIN" go run ./tests/e2e/ --test=both

for f in tests/e2e/profile/*.prof; do
    [ -f "$f" ] && cp "$f" "${OUTDIR}/bubbletea/"
done

echo "    Profiles saved to ${OUTDIR}/bubbletea/"
ls -lh "${OUTDIR}/bubbletea/"*.prof 2>/dev/null || echo "    WARNING: no .prof files found"

# ── Run e2e for zeroterm ──

echo ""
echo ">>> Running e2e test with ZEROTERM panix..."
echo "    (konsole will open — this is the TUI under test)"
mkdir -p "${OUTDIR}/zeroterm"

PANIX_BIN="$ZEROTERM_BIN" go run ./tests/e2e/ --test=both

for f in tests/e2e/profile/*.prof; do
    [ -f "$f" ] && cp "$f" "${OUTDIR}/zeroterm/"
done

echo "    Profiles saved to ${OUTDIR}/zeroterm/"
ls -lh "${OUTDIR}/zeroterm/"*.prof 2>/dev/null || echo "    WARNING: no .prof files found"

# ── Cleanup ──

echo ""
echo ">>> Cleaning up..."
git worktree remove "$WORKTREE" 2>/dev/null || true
rm -f "$BUBBLETEA_BIN" "$ZEROTERM_BIN"

# ── Summary ──

cat <<EOF

=== Results ===
Baseline:  ${BASELINE_COMMIT} (bubbletea, v0.6.0)
HEAD:      ${HEAD_COMMIT} (zeroterm)
Profiles:  ${OUTDIR}/

Compare (interactive web UI):
  go tool pprof -http=:8080 -diff_base=${OUTDIR}/bubbletea/cpu.prof ${OUTDIR}/zeroterm/cpu.prof
  go tool pprof -http=:8081 -diff_base=${OUTDIR}/bubbletea/mem.prof ${OUTDIR}/zeroterm/mem.prof

Compare (text):
  go tool pprof -top -diff_base=${OUTDIR}/bubbletea/cpu.prof ${OUTDIR}/zeroterm/cpu.prof
  go tool pprof -top -diff_base=${OUTDIR}/bubbletea/mem.prof ${OUTDIR}/zeroterm/mem.prof

Individual profiles:
  go tool pprof -top ${OUTDIR}/bubbletea/cpu.prof
  go tool pprof -top ${OUTDIR}/zeroterm/cpu.prof
EOF
