# Contributing

Get into dev shell:

```sh
devbox shell
```

Tasks that are used are implemented in [Taskfile.yml](Taskfile.yml).

Check commands for usage:

```sh
go-task

# or if you configure alias
task
```

Code has to pass `task ci` checks, if larger or critical sections were changed, then also `task go:test:e2e`.

## Testing Conventions

- **Table-driven tests**: use `[]struct{ name string; ... }` with `t.Run(tt.name, func(t *testing.T) { ... })`. See existing tests for examples.
- **Parallel execution**: every subtest calls `t.Parallel()` at the top.
- **Assertions**: use `testify/assert` for non-fatal checks, `testify/require` for fatal preconditions. Expected value comes first (enforced by `testifylint`).
- **Same-package testing**: tests live in the same package as the code they test (no `_test` suffix packages).
- **Test data**: use `testdata/` directories for fixture files (e.g. `internal/config/testdata/`).
- **Test factories**: use `internal/testutil/faker.go` for generating fake domain objects (SSH clients, machines, configs, flakes, fleets).
- **Linting**: `testifylint` is enforced via golangci-lint with all checks enabled.
- **Coverage**: `task go:test` runs with `-race -shuffle=on`, generates a coverage profile at `test/cover.out` and a badge at `gen/coverage.svg`.

Icons from [nerdfonts](https://www.nerdfonts.com/cheat-sheet).

## Future

Future potential/to-do list (by priority):

- improve code quality
- increase unit tests

## Past

The following packages were inadequate for use for Panix:

- [bubbletea](https://github.com/charmbracelet/bubbletea): too slow, it is severly unoptimized
- [Koanf link to issue](https://github.com/knadh/koanf/issues/221)
- [Viper link to issue](https://github.com/spf13/viper/issues/819)
- [urfave/cli](https://github.com/urfave/cli): using with [sflags](https://github.com/urfave/sflags) keeps placeholders just as "value" in help, does not properly generate env vars and flag names (have to manually specify them)
- [nix-fast-build](https://github.com/Mic92/nix-fast-build) instead of `nix build`: speed is about the same, and it does not seem to provide a meaningfull benefit over `nix build`

## Demo video

Record using OBS with [these settings](assets/obs), and show keys on screen:

```sh
nix-shell -p showmethekey --command showmethekey-gtk
```

### GIF

Convert using ([article](https://www.ffmpeg.media/articles/working-with-gifs-convert-optimize)):

```sh
ffmpeg -i demo.mp4 -vf "fps=10,scale=iw*0.8:-1:flags=lanczos,format=rgba,eq=saturation=1:contrast=1.3,split[s0][s1];[s0]palettegen=stats_mode=diff[p];[s1][p]paletteuse=dither=bayer:bayer_scale=5:diff_mode=rectangle" -loop 0 demo.gif
```

### MP4

Convert using:

```sh
INPUT="kexec-demo.mp4"
OUTPUT="kexec-demo-output.mp4"

nix run nixpkgs#ffmpeg-full -- -hide_banner -loglevel error -stats -i "$INPUT" \
  -vf "scale=-2:1080,fps=24" \
  -c:v libsvtav1 -crf 35 \
  -an \
  "$OUTPUT"
```
