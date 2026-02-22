# Contributing

Get into dev shell

```sh
nix develop
```

The following packages were inadequate for use for panix:

- [Koanf](https://github.com/knadh/koanf/issues/221)
- [Viper](https://github.com/spf13/viper/issues/819)
- [urfave/cli](https://github.com/urfave/cli): using with [sflags](https://github.com/urfave/sflags) keeps placeholders just as "value" in help, does not properly generate env vars and flag names, so you have to do it manualy for the ones that are not just single word names

Icons from [nerdfonts](https://www.nerdfonts.com/cheat-sheet).
