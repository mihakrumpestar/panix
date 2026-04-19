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

- checks in `task ci`

Icons from [nerdfonts](https://www.nerdfonts.com/cheat-sheet).

## Past

The following packages were inadequate for use for Panix:

- [Koanf link to issue](https://github.com/knadh/koanf/issues/221)
- [Viper link to issue](https://github.com/spf13/viper/issues/819)
- [urfave/cli](https://github.com/urfave/cli): using with [sflags](https://github.com/urfave/sflags) keeps placeholders just as "value" in help, does not properly generate env vars and flag names (have to manually specify them)
- [nix-fast-build](https://github.com/Mic92/nix-fast-build) instead of `nix build`: with `nix run github:Mic92/nix-fast-build -- --flake path:.#nixosConfigurations.personal-workstation.config.system.build.toplevel --no-link --skip-cached` it takes 1 min 6 sec instead of just 24 sec with `nix build`, while at moments using all CPU and all memory. Output also shows a lot of nix store lock errors and duplicated outputs (warnings) from nix eval.

## Future

Future potential:

- replace bubbletea with [tview](https://github.com/rivo/tview) for reduced binary size and improved performance

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
