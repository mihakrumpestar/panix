# panix

Nix deployment tool

## Setup

Remote requires to have ssh key authentication (kay file has to be without password, unless you are using an SSH agent), and password auth is also not supported.

If you have only password auth, create and add a temporary key to remote with the following commands:

```sh
# On remote (set password for root user)
sudo passwd
```

```sh
export REMOTE=<host>

# Generate key pair
ssh-keygen -t ed25519 -f ./temp_key -C "temporary_deployment_key" -N ""

# Copy key to remote (with disabled SSH agent to prevent trying to auth with keys in agent)
SSH_AUTH_SOCK="" ssh-copy-id -i ./temp_key.pub root@$REMOTE
```

You now may test the login with:

```sh
ssh -i ./temp_key -o IdentitiesOnly=yes root@$REMOTE
```

Now you can get (for example) the hardware config:

```sh
nixos-generate-config --no-filesystems --show-hardware-config
```

## Caviats

Not caviats so per se, just good things to know:

- local and remote have to have `rsync` installed to transfer plan files/dirs (note that `kexec` already includes `rsync`)
- default remote shell (e.g. `sh`) has to be POSIX compliant shell for specific commands to run (e.g. can't use `Fish` shell)
- for `kexec`, make sure you satisfy the [minimum system requirements](https://github.com/nix-community/nixos-images#requirements) (e.g. 1GB of memory without swap)

### YAML schema

Panix can generate YAML schema with `panix schema` for seeing parameter descriptions and their validation in your IDE. You just reference it in `panix.yml` as (you might need to add support using an extension like [vscode-yaml](https://github.com/redhat-developer/vscode-yaml)):

```yml
# yaml-language-server: $schema=./panix-schema.yaml
```

## Contributing

### Notes

The following packages were inadequate for use for panix:

- [Koanf](https://github.com/knadh/koanf/issues/221)
- [Viper](https://github.com/spf13/viper/issues/819)

Icons from [nerdfonts](https://www.nerdfonts.com/cheat-sheet).
