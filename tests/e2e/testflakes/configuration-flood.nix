{
  imports = [./configuration.nix];

  # E2E-only guard fixture for the wire-backpressure legs (tests/e2e/
  # guard_v2.go): activation emits a large burst of output lines, each of
  # which the guardian dual-writes to the slot log and the wire's bounded
  # drop-oldest buffer. Producer-side volume can never outrun the relay's
  # drain, so the leg injects consumer-side backpressure instead: it stalls
  # the relay (SIGSTOP) around the burst, the guardian's fd4 writer blocks,
  # and the 256-frame drop-oldest ring fills (LINK_DEGRADED, log only,
  # spec 6.1) while the transaction itself proceeds untouched to the confirm
  # gate and the commit.
  system.activationScripts.panix-flood-for-e2e = ''
    echo "panix-e2e: flooding the guard wire with 8000 padded lines" >&2
    # Pre-burst window: gives the harness a window to stall the relay before
    # the burst starts, so the stall is deterministic rather than racing the
    # first frames. (A stall landing mid-burst still overflows the ring; the
    # window just removes the timing race.)
    sleep 2
    # Pure shell builtins: the activation script PATH is the minimal init
    # PATH, so external binaries (seq, sed) are not reachable here. Each
    # line is padded to ~2KB: small frames drain as fast as they are
    # produced, and only a throughput-bound consumer lets the 256-frame
    # drop-oldest ring fill (LINK_DEGRADED requires real overflow).
    FILLER=""
    j=0
    while [ "$j" -lt 40 ]; do
      FILLER="$FILLER panix-e2e-filler-0123456789abcdef0123456789abcdef"
      j=$((j + 1))
    done
    i=0
    while [ "$i" -lt 8000 ]; do
      echo "panix-e2e-flood-line-$i$FILLER"
      i=$((i + 1))
    done
    echo "panix-e2e: flood done"
  '';
}
