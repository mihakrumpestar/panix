# panix

Nix deployment tool

## Setup

Remote requires to have ssh key authentication (kay file has to be without password, unless you are uaing SSH agent), password auth is not supported.

If you have only password auth, create and add a temporary key to remote with the following commands:

```sh
ssh-keygen -t ed25519 -f ./temp_key -C "temporary_deployment_key" -N ""
SSH_AUTH_SOCK="" ssh-copy-id -i ./temp_key.pub nixos@<host>

# Installation requires root user
ssh -i ./temp_key -o IdentitiesOnly=yes nixos@<host> "sudo mkdir -p /root/.ssh; sudo cp ~/.ssh/authorized_keys /root/.ssh"
```

You now may test the login with:

```sh
ssh -i ./temp_key -o IdentitiesOnly=yes root@<host>
```

## Notes

Problematic:

- [Koanf](https://github.com/knadh/koanf/issues/221)

Not using:

- [Viper](https://github.com/spf13/viper/issues/819)
