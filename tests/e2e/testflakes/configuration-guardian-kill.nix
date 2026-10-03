{pkgs, ...}: {
  imports = [./configuration.nix];

  # E2E-only guard fixture for the guardian-death leg (tests/e2e/guard_v2.go):
  # activation kills the DETACHED guardian mid-window (PANIX_GUARD_DETACHED
  # marks it; the relay and every other panix-guard process is left alive) and
  # then hangs, so the orphaned activation child stays around for the
  # post-mortem converge to kill. The relay must classify the event-pipe EOF
  # as cancelled (exit 6) and panix must converge inline from the
  # transaction's own records. The kill waits for a STATE record carrying the
  # child pid first, so the post-mortem converge reliably finds the orphan.
  system.activationScripts.panix-guardian-kill-for-e2e =
    let
      killGuardian = pkgs.writeShellScript "panix-e2e-kill-guardian" ''
        for p in /proc/[0-9]*; do
          pid=''${p#/proc/}
          tr '\0' ' ' < "$p/cmdline" 2>/dev/null | grep -q 'panix-guard start' || continue
          tr '\0' '\n' < "$p/environ" 2>/dev/null | grep -q '^PANIX_GUARD_DETACHED=1$' || continue
          kill -9 "$pid" 2>/dev/null && echo "panix-e2e: guardian $pid killed for the death leg" >&2
        done
      '';
    in
    ''
      echo "panix-e2e: waiting for the guardian to record the child pid" >&2
      attempts=0
      while [ "$attempts" -lt 30 ]; do
        grep -q '"cpid":' /run/panix-guard/v2/*/log 2>/dev/null && break
        attempts=$((attempts + 1))
        sleep 0.5
      done
      echo "panix-e2e: killing the guardian mid-window" >&2
      ${killGuardian}
      echo "panix-e2e: guardian killed, hanging as an orphan" >&2
      sleep 600
    '';
}
