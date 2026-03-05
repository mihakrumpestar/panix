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


The following packages were inadequate for use for Panix:

- [Koanf link to issue](https://github.com/knadh/koanf/issues/221)
- [Viper link to issue](https://github.com/spf13/viper/issues/819)
- [urfave/cli](https://github.com/urfave/cli): using with [sflags](https://github.com/urfave/sflags) keeps placeholders just as "value" in help, does not properly generate env vars and flag names (have to manually specify them)

Icons from [nerdfonts](https://www.nerdfonts.com/cheat-sheet).
