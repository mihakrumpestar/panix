# Contributing

Get into dev shell:

```sh
devbox shell
```

Tasks that are used are implemented in [Taskfile.yml](Taskfile.yml).

Check

```sh
go-task

# or if you configure alias
task
```

for usage.

Code has to pass:

- all checks in `pre-commit run`: most importantly `task golangci`

Icons from [nerdfonts](https://www.nerdfonts.com/cheat-sheet).

## Past

The following packages were inadequate for use for Panix:

- [Koanf link to issue](https://github.com/knadh/koanf/issues/221)
- [Viper link to issue](https://github.com/spf13/viper/issues/819)
- [urfave/cli](https://github.com/urfave/cli): using with [sflags](https://github.com/urfave/sflags) keeps placeholders just as "value" in help, does not properly generate env vars and flag names (have to manually specify them)

## Future

Future potential:

- replace bubbletea with [tview](https://github.com/rivo/tview) for reduced binary size and improved performance
- [nix-fast-build](https://github.com/Mic92/nix-fast-build) instead of `nix build`

## Demo video

### GIF

Convert using:

```sh
nix run nixpkgs#ffmpeg-full -- -i <source>.mp4 -lavfi "split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse" demo.gif'
```

### MP4

Convert using:

```sh
INPUT="panix kexec.mp4"
OUTPUT="output-variable35.mp4"

nix run nixpkgs#ffmpeg-full -- -hide_banner -loglevel error -stats -i "$INPUT" \
  -vf "scale=-2:1080,fps=24" \
  -c:v libsvtav1 -crf 35 \
  -an \
  "$OUTPUT"
```
