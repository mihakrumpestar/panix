# Changelog

## [0.9.0](https://github.com/mihakrumpestar/panix/compare/v0.8.1..v0.9.0) - 2026-08-05

Panix is now a general installable deployment orchestrator. Installables are no longer limited to NixOS profiles; any installable type with an activation mode can be deployed. Default options and flags are overridable, and nix options can be set via environment variables.

**Breaking:** the config schema for installables has changed. Existing configs need to be updated to the new installable-based format.

Migration: rename `configurations` to `installables` and nest your output type (e.g. `nixosConfigurations`) as a key under it:

Before:
```yaml
fleet:
  flakes:
    my-infra:
      configurations:
        workstation:
          machines:
            workstation:
```

After:
```yaml
fleet:
  flakes:
    my-infra:
      installables:
        nixosConfigurations:
          workstation:
            machines:
              workstation:
```

### Bug Fixes

- Update fullscrean viewport by @mihakrumpestar ([9a209c7](https://github.com/mihakrumpestar/panix/commit/9a209c7cd30715cbfb881e831aa30e4173bde303))

### Features

- Make Panix a general installable deployment orchestrator ([#11](https://github.com/mihakrumpestar/panix/pull/11)) by @mihakrumpestar ([9bc070a](https://github.com/mihakrumpestar/panix/commit/9bc070a152941d332ef6a1512859356c68dde002))

### Miscellaneous

- Small Taskfile and docs reverts by @mihakrumpestar ([cd21519](https://github.com/mihakrumpestar/panix/commit/cd21519fd97a8e3072097d551623d9ffdedf0966))

## [0.8.1](https://github.com/mihakrumpestar/panix/compare/v0.8.0..v0.8.1) - 2026-07-08

### Bug Fixes

- Make kexec properly use default port 22 by @mihakrumpestar ([02a0922](https://github.com/mihakrumpestar/panix/commit/02a092240903a3df7846db2ae69b7d0f469a80b9))

### Features

- Improve e2e benchmarks and update go benchmarks, update docs by @mihakrumpestar ([7b1c022](https://github.com/mihakrumpestar/panix/commit/7b1c02259ae0e4419a09ce7f99936d11ca04cc6e))
- Improve snapshot json entities ordering, improve docs by @mihakrumpestar ([981407b](https://github.com/mihakrumpestar/panix/commit/981407b9eeeceaf6febdeb605ff1868e43524fb2))
- Add limit to scrollback max lines, fixes #10 by @mihakrumpestar ([98c721b](https://github.com/mihakrumpestar/panix/commit/98c721b7009b64d77a9e6c5640f00175a0778cd4))

### Miscellaneous

- Simplify bench_graph output by @mihakrumpestar ([d322285](https://github.com/mihakrumpestar/panix/commit/d32228511f6a8e2318039cff44d5024d8392551f))

## [0.8.0](https://github.com/mihakrumpestar/panix/compare/v0.7.1..v0.8.0) - 2026-06-23

### Bug Fixes

- (pty) Handle split ANSI escapes and cursor flag leaks in terminal output processor; feat: added --keep-going to base args of nix build by @mihakrumpestar ([bba2a67](https://github.com/mihakrumpestar/panix/commit/bba2a671c50fdbc13ff3cd8b15a6e30ac52279cc))
- Nix build, e2e test bench, readme by @mihakrumpestar ([f0ba28e](https://github.com/mihakrumpestar/panix/commit/f0ba28ea50890e8636afc9dfbff53d6f3b053bfd))

### Features

- Small optimizations in TUI  ([#9](https://github.com/mihakrumpestar/panix/pull/9)) by @mihakrumpestar ([fde5446](https://github.com/mihakrumpestar/panix/commit/fde54467ec7219bdedb2e5dd2f5a9323f50c82bd))

### Testing

- Make e2e test more deterministic (using local binary cache); chore: split go.mod for e2e tests and main by @mihakrumpestar ([9c0d7cd](https://github.com/mihakrumpestar/panix/commit/9c0d7cdcb1020682d0a8d8ea107a173b408480df))

## [0.7.1](https://github.com/mihakrumpestar/panix/compare/v0.7.0..v0.7.1) - 2026-06-03

### Bug Fixes

- Fix cliff config by @mihakrumpestar ([c018b3c](https://github.com/mihakrumpestar/panix/commit/c018b3c6bbbd5c823a05c07efda832d2d5b2916b))
- Improve panix config example, fix Viewport race condition, make NixConfig be properly initialized, merged and referenced in readme by @mihakrumpestar ([5f1a993](https://github.com/mihakrumpestar/panix/commit/5f1a993b990a205b07f53ef39020a831f791720a))
- Make commands string be marshalled on snapshot by @mihakrumpestar ([d74ced8](https://github.com/mihakrumpestar/panix/commit/d74ced8dc4dd00a0d56bac87a7529ec7593e7d55))
- Port in Lix store path fix, feat: add nix version to snapshots and config by @mihakrumpestar ([59395c4](https://github.com/mihakrumpestar/panix/commit/59395c4bebcd696c27355b170f024b215c2cae1d))

### Documentation

- Replace tui.png by @mihakrumpestar ([1e64964](https://github.com/mihakrumpestar/panix/commit/1e6496456bd723f8195de9f7c70ddc646859ac78))
- Migrated from readme to wiki by @mihakrumpestar ([e034fd4](https://github.com/mihakrumpestar/panix/commit/e034fd42289b57b0dc8b62f37bb6365aae03d22c))

## [0.7.0](https://github.com/mihakrumpestar/panix/compare/v0.6.0..v0.7.0) - 2026-05-22

### Bug Fixes

- Restore live flag in StartTimer so elapsed time updates after retry by @mihakrumpestar ([6d3c317](https://github.com/mihakrumpestar/panix/commit/6d3c317536df23ad4f2c7a8ce31b90a0d82c69fb))
- Build command by @mihakrumpestar ([89f2caf](https://github.com/mihakrumpestar/panix/commit/89f2caf41406577e78e677593472216c13defa62))
- Seperate eval-cache path to prevent SQlite busy warnings, switch build to --print-out-paths from --json by @mihakrumpestar ([b45c50e](https://github.com/mihakrumpestar/panix/commit/b45c50e416f2629d515d133032a9d0e7bc6edab5))
- Slight changes to benchmarks, no change in results by @mihakrumpestar ([b42235a](https://github.com/mihakrumpestar/panix/commit/b42235a8f715ccf10f309b0aead5a4bdb0b3cfba))
- Make SSH config read only machines left after filtering, fix SSH target resolution by @mihakrumpestar ([7ef8f3d](https://github.com/mihakrumpestar/panix/commit/7ef8f3d111b18231f1aa393eb02dde2f43d21955))
- Prevent overwriting shared memory buffer of viewport in build_logs by @mihakrumpestar ([8328c61](https://github.com/mihakrumpestar/panix/commit/8328c61e789d702d6fe815ee656f49439031d3f7))
- Proper len for prev fix by @mihakrumpestar ([69c8c34](https://github.com/mihakrumpestar/panix/commit/69c8c34b8ae87da03e6015ea03468d60ae73ac1b))
- Prevent SSH config errors on non-existant SSH config by @mihakrumpestar ([36bc366](https://github.com/mihakrumpestar/panix/commit/36bc366c91a517b489f444d94c275b4502832ed6))
- Fix all linter errors, use testify for all tests by @mihakrumpestar ([c92c1b6](https://github.com/mihakrumpestar/panix/commit/c92c1b673464ac2675a15f43cfc9dc0dbafbdbe6))

### Documentation

- Add diagrams to readme by @mihakrumpestar ([1cc7c25](https://github.com/mihakrumpestar/panix/commit/1cc7c256372dc630e180a475fe1d21babe7242d6))

### Features

- Remote builds, custom build TUI library (zeroterm), benchmarks, e2e tests, fixes  by @mihakrumpestar ([1727919](https://github.com/mihakrumpestar/panix/commit/172791901dac1f9ec28772b508709ab8c726c31d))
- Make flake URL accept uri|dir and make it optional, with default being '.' by @mihakrumpestar ([3ff73d3](https://github.com/mihakrumpestar/panix/commit/3ff73d357d67e6c2a6152f049fb157f2c0c9dc79))
- Improve workflow code by @mihakrumpestar ([7f1b5b5](https://github.com/mihakrumpestar/panix/commit/7f1b5b5ac730720433da19a4620c6d6d27954aa2))

### Miscellaneous

- Update deps by @mihakrumpestar ([11e16a8](https://github.com/mihakrumpestar/panix/commit/11e16a8c38f75e70b47a27fdc29342381a6f685d))
- Verify e2e tests by @mihakrumpestar ([ea76434](https://github.com/mihakrumpestar/panix/commit/ea76434d164df787a653f4c14b40fa7fc23f8c5b))

## [0.6.0](https://github.com/mihakrumpestar/panix/compare/v0.5.0..v0.6.0) - 2026-04-25

### Bug Fixes

- Properly format merge commits in changelog by @mihakrumpestar ([df3b31b](https://github.com/mihakrumpestar/panix/commit/df3b31b665d5e175f0941bb222a0e14ef54763ad))
- Fix description, license and maintainer in flake by @mihakrumpestar ([6014f9a](https://github.com/mihakrumpestar/panix/commit/6014f9ae6d3e857b6e9d1fcf32c257d7a1988ed5))
- Security: make kexec ssh use StrictKeyChecking and DisableAutoAddHostKey from parent instead of hardcoding them by @mihakrumpestar ([aef468b](https://github.com/mihakrumpestar/panix/commit/aef468bc4f6700452ae6f73b091b78bb09f5d0d0))
- Security: make KnownHostsFile customizable and enable strict key checking by @mihakrumpestar ([e4f7e85](https://github.com/mihakrumpestar/panix/commit/e4f7e8523a6cda571948cd0941886915c42d1374))

### Documentation

- Segment features and add internal/pkg readme by @mihakrumpestar ([885db2b](https://github.com/mihakrumpestar/panix/commit/885db2bd4e03cebf7cef31aa6df1b23572e5dad0))
- Make slogan consistent by @mihakrumpestar ([2aed414](https://github.com/mihakrumpestar/panix/commit/2aed414baf4bd14686eaed92715c344cd5b5b0d7))

### Features

- Add first tests by @mihakrumpestar ([ea20885](https://github.com/mihakrumpestar/panix/commit/ea20885648fb70570b5e3997929d935d68bd5c1f))
- Add new eval command, rename previous eval to template; extract commands into standalone/workflow packages; add validation subsystem for paths by @mihakrumpestar ([315ea1e](https://github.com/mihakrumpestar/panix/commit/315ea1e0ff48979a08dd5b6091fb5beefa976fd9))
- Add e2e tests by @mihakrumpestar ([4fc50b9](https://github.com/mihakrumpestar/panix/commit/4fc50b9008b21f49174befe3332251dfa1bb1559))
- Autogenerate loc badge by @mihakrumpestar ([a3c620b](https://github.com/mihakrumpestar/panix/commit/a3c620bf20b9056afcae00714a0321134e1c638d))

### Miscellaneous

- Update demo gif by @mihakrumpestar ([23b35a7](https://github.com/mihakrumpestar/panix/commit/23b35a7779dcea6ad12f42ecf15dce103864ef40))
- Update readme for prev commit by @mihakrumpestar ([2439d1e](https://github.com/mihakrumpestar/panix/commit/2439d1e7e689bad287a1e422b5ddc5437d4d9592))

## [0.5.0](https://github.com/mihakrumpestar/panix/compare/v0.4.0..v0.5.0) - 2026-04-19

### Bug Fixes

- Make main viewport not follow bottom by @mihakrumpestar ([ed98e61](https://github.com/mihakrumpestar/panix/commit/ed98e61b81fec1698993d28150706e3cd2c06b99))

### Features

- Snapshot feature refactor  by @mihakrumpestar ([08dce8c](https://github.com/mihakrumpestar/panix/commit/08dce8c9cffdda7ccf736263520111d82351a24a))
- Improve cache for command output, improve/update readme by @mihakrumpestar ([553bf28](https://github.com/mihakrumpestar/panix/commit/553bf28dc7677c02677a43d38afc7d4a42e0cbba))

## [0.4.0](https://github.com/mihakrumpestar/panix/compare/v0.3.0..v0.4.0) - 2026-04-09

### Bug Fixes

- Make rollback generation number an attribute instead of arg, since -1 gets detected as flag instead of value by @mihakrumpestar ([212c93b](https://github.com/mihakrumpestar/panix/commit/212c93b3adc407792d487016382855079c1b6d0f))

### Features

- Add alternative rendering modes besides TUI: console and json by @mihakrumpestar ([b0d8277](https://github.com/mihakrumpestar/panix/commit/b0d82774b1c37e97c9927d927aff4300588b7f30))
- Rename config element: root -> fleet by @mihakrumpestar ([247e8d3](https://github.com/mihakrumpestar/panix/commit/247e8d3bc9a04ce24b08e264bcf671db133ea389))
- Add init command, fix: rename references root -> fleet, chore: improve some readme sections by @mihakrumpestar ([001ffd1](https://github.com/mihakrumpestar/panix/commit/001ffd13e1afabffb0a47d3a35e882bae6781f0a))

## [0.3.0](https://github.com/mihakrumpestar/panix/compare/v0.2.0..v0.3.0) - 2026-04-07

### Bug Fixes

- Deploy command workflow description by @mihakrumpestar ([ccc9d74](https://github.com/mihakrumpestar/panix/commit/ccc9d74508a7cda5633ae1fe033cf5e5297a8f2d))
- Make Running color scheme more visible, make rollback standalone phase work by @mihakrumpestar ([fe85825](https://github.com/mihakrumpestar/panix/commit/fe858259b21ee8eb3fa992fe29a296f7fb9c7096))
- Proper global and subcommand flag separation by @mihakrumpestar ([f70d4f1](https://github.com/mihakrumpestar/panix/commit/f70d4f1f411899334d10c283baf8eb73bef4ea2e))
- Add gh check to release by @mihakrumpestar ([5996f6f](https://github.com/mihakrumpestar/panix/commit/5996f6f0a33585c15308251b35d4dd3b7594f94d))

### Documentation

- Improve docs by @mihakrumpestar ([0d133d0](https://github.com/mihakrumpestar/panix/commit/0d133d08fa4b46455bb5bdc03d8946e0ca9d1706))

### Features

- Added Golang template engine support, fix: golangci-lint, feat: improve CI workflow by @mihakrumpestar ([9e1b201](https://github.com/mihakrumpestar/panix/commit/9e1b20124aee2e5103094f6b18eb5ed6594d9267))
- Added support for references in yaml schema by @mihakrumpestar ([295ae16](https://github.com/mihakrumpestar/panix/commit/295ae16d0d4d17da737551442b0a86f2d02a5a22))
- Removed Bootstrap.DisableAuto flag, made SSH configs deterministic by @mihakrumpestar ([fc186f9](https://github.com/mihakrumpestar/panix/commit/fc186f97dedaad28b73a0b83feeb68321f18a658))
- Filter out unused phases (bootstrap, secrets) that are optional, chore: split loader into more smaller files by @mihakrumpestar ([cae23b6](https://github.com/mihakrumpestar/panix/commit/cae23b6382b02570be0c093af3d162f2e93c87d4))

### Miscellaneous

- Update deps, fix PhaseStateActive colors by @mihakrumpestar ([b9a77fd](https://github.com/mihakrumpestar/panix/commit/b9a77fdc215bd1c614b13f1420a840f6e4677db3))
- Updated demo by @mihakrumpestar ([b6294be](https://github.com/mihakrumpestar/panix/commit/b6294bed71453065adaf8867ebd3826eea15e5d2))
- Some testing with alternative builder by @mihakrumpestar ([f9f2115](https://github.com/mihakrumpestar/panix/commit/f9f21158a71792e5c9dbefb5e3e28e05ddc7a872))

## [0.2.0](https://github.com/mihakrumpestar/panix/compare/v0.1.3..v0.2.0) - 2026-03-31

### Bug Fixes

- Skip version bump commit from release logs by @mihakrumpestar ([c03cfaa](https://github.com/mihakrumpestar/panix/commit/c03cfaacedf05d4bc9fd7fc8ef5cd7bc8f58b62c))
- Make WaitForReconnect properly wait with WaitForReconnectCheckInterval by @mihakrumpestar ([9bc4f97](https://github.com/mihakrumpestar/panix/commit/9bc4f9736ede9235d9edae03b8399e77b9c94489))
- PhasesInOrder now does not contain rollback, correctly handle skipped phases by @mihakrumpestar ([cb7de7e](https://github.com/mihakrumpestar/panix/commit/cb7de7e2416b7483401e11a1817b5bfdd0e3ed69))

### Features

- Added more feature docs, kexec ssh port option and switching by @mihakrumpestar ([b2b02e9](https://github.com/mihakrumpestar/panix/commit/b2b02e9a8fa17dd7d091af9cc105ae15f4c5baf6))
- Improve pre-commit and generate workflows by @mihakrumpestar ([403a179](https://github.com/mihakrumpestar/panix/commit/403a17940fc50b214402f5d2c82e2f5ad58c0f18))
- Remove explicit bootstrap phase by @mihakrumpestar ([3a786cc](https://github.com/mihakrumpestar/panix/commit/3a786cca86d7d68a6ae57c07ec9369d634bdffc7))
- Make timout per command instead of per workflow by @mihakrumpestar ([6eac5e6](https://github.com/mihakrumpestar/panix/commit/6eac5e66ab2bbd4c9bfe078779266da6a71f7da3))
- Added more activation modes by @mihakrumpestar ([29247de](https://github.com/mihakrumpestar/panix/commit/29247de7bde86d29c769b0a0241c8819a6af1e5f))
- Added rollback command by @mihakrumpestar ([fbb46e6](https://github.com/mihakrumpestar/panix/commit/fbb46e6a5e9d3c4ae78bb1f5eae8d0dae05b0fd2))
- Added configuration options to specify nix command flags by @mihakrumpestar ([1e9832b](https://github.com/mihakrumpestar/panix/commit/1e9832bc57afdd3d88a11c51311013f73ea12a4b))

## [0.1.3](https://github.com/mihakrumpestar/panix/compare/v0.1.2..v0.1.3) - 2026-03-24

### Bug Fixes

- Allow nix experimental commands to run anywhere by enabling experimental features by @mihakrumpestar ([9ce7f07](https://github.com/mihakrumpestar/panix/commit/9ce7f079531e4cbd6e850ddebb0b7cc6eaaa50b8))
- Scrollbar to use wrapped content height, add missing zone marker for fullscreen viewport focus, remove fixed width of 3 for stats table symbol by @mihakrumpestar ([43187bb](https://github.com/mihakrumpestar/panix/commit/43187bba5e2b537c76a012ce2a54bb648f64196a))
- Revert phase status status icon back to fixed width by @mihakrumpestar ([8299f53](https://github.com/mihakrumpestar/panix/commit/8299f5353bfcc4fa43a88310fc23cc3dd34640be))

### Documentation

- Add new way of funding/sponsoring by @mihakrumpestar ([ae52b21](https://github.com/mihakrumpestar/panix/commit/ae52b21c6361ada515e7a47e32b7ebc9b78330f7))
- Add a detailed explanation of Secrets phase by @mihakrumpestar ([8bbe48a](https://github.com/mihakrumpestar/panix/commit/8bbe48a8921294c1d9deb22001265f596017655e))

### Miscellaneous

- Update deps by @mihakrumpestar ([f3d166d](https://github.com/mihakrumpestar/panix/commit/f3d166da45c1967938ef481da7598929c81620af))
- Update golang to 1.26.1, update flake nixpkgs, fix minor golangci issues by @mihakrumpestar ([935f86f](https://github.com/mihakrumpestar/panix/commit/935f86f830cc5a91b681bee8f16d6d54620a6487))

## [0.1.2](https://github.com/mihakrumpestar/panix/compare/v0.1.1..v0.1.2) - 2026-03-12

### Bug Fixes

- Add space before commit and PR links by @mihakrumpestar ([70a007e](https://github.com/mihakrumpestar/panix/commit/70a007ea348b237f7a84cfaa3f6688ca3226b1b3))
- Make remote SSH terminal not to allocate PTY which may cause Kexec to fail by @mihakrumpestar ([2679a70](https://github.com/mihakrumpestar/panix/commit/2679a7010e8c564ccc7cab4b507dff9661a3b02e))
- Revert back to full whitespace trim for command outputs by @mihakrumpestar ([0175378](https://github.com/mihakrumpestar/panix/commit/0175378973095b9765d6008ef1c31aeeb23cfa00))

### Documentation

- Add version badge by @mihakrumpestar ([893f3e0](https://github.com/mihakrumpestar/panix/commit/893f3e0821f242c3b61ce256b698a2d3ea22022c))
- Add more README badges by @mihakrumpestar ([410fcb5](https://github.com/mihakrumpestar/panix/commit/410fcb5eeb165777d1206990562b491afda2bcb9))
- Improve README, remove funding (since it has not yet been set up) by @mihakrumpestar ([482528a](https://github.com/mihakrumpestar/panix/commit/482528a35497f83142336b076acc63a87ef854be))
- Add bootstrap with kexec demo video by @mihakrumpestar ([f529f7c](https://github.com/mihakrumpestar/panix/commit/f529f7c999e0de80deeed1d6bcc2c47549601b0a))
- Fix kexec demo by @mihakrumpestar ([88ce4ff](https://github.com/mihakrumpestar/panix/commit/88ce4ff00a3286ec8c3924fd1af2c185322fd6d3))
- Fix kexec demo 2 by @mihakrumpestar ([7115a37](https://github.com/mihakrumpestar/panix/commit/7115a3788c2a6423733526d96335f9e05c1a10fa))
- Fix kexec demo 3 by @mihakrumpestar ([37636fa](https://github.com/mihakrumpestar/panix/commit/37636faaefe9bdabd2dfa5f3ce3280c7d605adaf))
- Update ffmpeg commands by @mihakrumpestar ([ee39500](https://github.com/mihakrumpestar/panix/commit/ee395009b7813c2ecfb011e69d20e16f2182936f))

### Features

- Add force_bootstrap and force_bootstrap_kexec options by @mihakrumpestar ([3c35b11](https://github.com/mihakrumpestar/panix/commit/3c35b11d5ea965f2242a618cd9305607b20f58cf))

### Miscellaneous

- Add new demo video and pic by @mihakrumpestar ([aeee087](https://github.com/mihakrumpestar/panix/commit/aeee0870c076dff00841ce459ce238e19e33c98b))

## [0.1.1](https://github.com/mihakrumpestar/panix/compare/v0.1.0..v0.1.1) - 2026-03-10

### Bug Fixes

- Make Cliff add links to commits and PRs by @mihakrumpestar ([24c14e7](https://github.com/mihakrumpestar/panix/commit/24c14e73d74d7d62ebeff1b56eddeba52d19949e))

## [0.1.0](https://github.com/mihakrumpestar/panix/compare/v0.0.3..v0.1.0) - 2026-03-10

### Bug Fixes

- Add missing permissions to secrets struct, fix: make secrets transefer not use SSH, fix: task release to go generate after new version is written instead of later by @mihakrumpestar ([3e5e7f5](https://github.com/mihakrumpestar/panix/commit/3e5e7f52c23933badebbca18c02723135090b642))
- Add context cancellation in PTY loop, make ActiveSSH thread-safe with atomic pointer, optimize slice appends and xpath building, and stream config loading by @mihakrumpestar ([6cef38c](https://github.com/mihakrumpestar/panix/commit/6cef38c9f8ef753f77a60bd9cae4db793dbea55d))
- Move animation state to PhaseStatus with atomic fields, limit worker pool to 1000 goroutines, and optimize string concatenation with strings.Builder by @mihakrumpestar ([5c191a2](https://github.com/mihakrumpestar/panix/commit/5c191a22fc43143b482297d544794c0f356aaf3a))
- Improve error handling with proper wrapping, context, and panic messages by @mihakrumpestar ([398e49c](https://github.com/mihakrumpestar/panix/commit/398e49c3d53c71cf73f0b69810a4a8c43ffe49db))
- Resource leaks, race conditions, and improve error handling by @mihakrumpestar ([02f5185](https://github.com/mihakrumpestar/panix/commit/02f51855fa258686b80a0589d9f178099b8cae97))
- Connection leak, infinite loop on shutdown, add bounds checks, and use type-safe omap by @mihakrumpestar ([13d5236](https://github.com/mihakrumpestar/panix/commit/13d52362a59ad8a670b30b917749053c07225694))
- Correct MachineCount to return count instead of last index, pass kexec-extra-flags as separate arguments to exec.Command, security: validate URLs have http/https scheme and host, fix: close CPU profile file after stopping profiler, refactor: remove unused Pty field from CommandLog by @mihakrumpestar ([7e34582](https://github.com/mihakrumpestar/panix/commit/7e34582df202deb9767f609ed32c4c7431e98665))
- Make command error be a viewport so it wraps if over-width by @mihakrumpestar ([4a3d80a](https://github.com/mihakrumpestar/panix/commit/4a3d80a643805589b9623207f0b6d2b650b8f459))
- Build logs right timers indentation and don't repeat machine labels by @mihakrumpestar ([07edc1d](https://github.com/mihakrumpestar/panix/commit/07edc1d9c12692a7c4dccc6dff24018809fc0eae))
- Filter out json output for drv paths from being rendered in build logs by @mihakrumpestar ([0cbe75e](https://github.com/mihakrumpestar/panix/commit/0cbe75e1d1db818519135d30e4519c481c29b941))
- Release in Taskfile by @mihakrumpestar ([870ffe4](https://github.com/mihakrumpestar/panix/commit/870ffe4dc26cd2aff51cb4fd766ffd145132ee12))

### Documentation

- Add DiskEncryptionKeys description in schema by @mihakrumpestar ([005d666](https://github.com/mihakrumpestar/panix/commit/005d666321d9a52c25680622ae6d19915b9f809c))
- Update docs by @mihakrumpestar ([444d164](https://github.com/mihakrumpestar/panix/commit/444d164ba1726752ec89519e70b81e0151f9d4ab))

### Features

- Add bootstrap SSH, post-bootstrap hooks, and rename SSH options, fix: copy to clipboard now copys entire content, not just the visible part of viewport by @mihakrumpestar ([c1539f6](https://github.com/mihakrumpestar/panix/commit/c1539f60cf33ac1ee7f44f4947787a30a6bae442))
- Make kexec part of bootstrap process instead of inspect by @mihakrumpestar ([9b8aa07](https://github.com/mihakrumpestar/panix/commit/9b8aa073af41af071c95a055d1b93ce17ebdd407))
- Migrate to bubbletea v2 and lipgloss v2: update View() to return tea.View, replace KeyMsg/MouseMsg with v2 types, change viewport fields to methods by @mihakrumpestar ([c6afeec](https://github.com/mihakrumpestar/panix/commit/c6afeec33a165fc55fab9d1c3b260b9ca0f21601))
- Add golangci by @mihakrumpestar ([f67c1f1](https://github.com/mihakrumpestar/panix/commit/f67c1f1528b4d0b287b9ad2c40b8b49e4a4b41f7))

### Miscellaneous

- Golangci wsl by @mihakrumpestar ([b2f5396](https://github.com/mihakrumpestar/panix/commit/b2f5396e77fa940f7ad129774bbeb02381addc8b))
- Golangci wrapcheck by @mihakrumpestar ([73e5033](https://github.com/mihakrumpestar/panix/commit/73e5033b064500366252395e25a12208c6e42b31))
- Golangci varnamelen by @mihakrumpestar ([eb55890](https://github.com/mihakrumpestar/panix/commit/eb558901a12c3641800e0f3b4150b9a240f8a833))
- Golangci staticcheck by @mihakrumpestar ([99be866](https://github.com/mihakrumpestar/panix/commit/99be8668bb5a059a2119a53c55fbb948092c9453))
- Golangci perfsprint by @mihakrumpestar ([709eaa7](https://github.com/mihakrumpestar/panix/commit/709eaa7974d8590e8bee413b081a4c7a4cdd7ba8))
- Golangci nlreturn by @mihakrumpestar ([558d2a6](https://github.com/mihakrumpestar/panix/commit/558d2a6fdd4a71dd878a78e94553742b644c1f04))
- Golangci nilaway by @mihakrumpestar ([e1abb5d](https://github.com/mihakrumpestar/panix/commit/e1abb5dcee8f7b5a4ab6245bc8e82e1bf4647e37))
- Golangci nestif by @mihakrumpestar ([f98eaab](https://github.com/mihakrumpestar/panix/commit/f98eaab972cbc70ba076e72a6bded67914260748))
- Golangci mnd by @mihakrumpestar ([17ccbcd](https://github.com/mihakrumpestar/panix/commit/17ccbcdd01c00746c8e51f28ffc3d08ace073f82))
- Golangci lll by @mihakrumpestar ([a71b039](https://github.com/mihakrumpestar/panix/commit/a71b0391af5d51f139d258fbc9244418d5c6e2a8))
- Golangci intrange by @mihakrumpestar ([2b6b58a](https://github.com/mihakrumpestar/panix/commit/2b6b58a07f84133e486730cb95a887c282461175))
- Golangci gosec by @mihakrumpestar ([d027b80](https://github.com/mihakrumpestar/panix/commit/d027b80971177cf976a109ea8d4bea3e7a8365e3))
- Golangci godot by @mihakrumpestar ([4f33b19](https://github.com/mihakrumpestar/panix/commit/4f33b19b1652d058c64d1da27d1d506315fb73a6))
- Golangci gocritic by @mihakrumpestar ([933c19f](https://github.com/mihakrumpestar/panix/commit/933c19f80c4d7c6297e94fe4d51178f855e98def))
- Golangci gocognit by @mihakrumpestar ([3341a93](https://github.com/mihakrumpestar/panix/commit/3341a93dec010d38dd12a93e7c460b2d6abe13d4))
- Golangci varnamelen by @mihakrumpestar ([4a79eab](https://github.com/mihakrumpestar/panix/commit/4a79eabf201df37f309b745e393e1594c71db34c))
- Golangci gochecknoinits by @mihakrumpestar ([98f9be4](https://github.com/mihakrumpestar/panix/commit/98f9be41587816947f773b3ebdbbc55838af97ba))
- Golangci funlen by @mihakrumpestar ([1841d08](https://github.com/mihakrumpestar/panix/commit/1841d08df773d64159678c76066ee52db737cdb0))
- Golangci forcetypeassert by @mihakrumpestar ([45f3fda](https://github.com/mihakrumpestar/panix/commit/45f3fda8ba859a05d2f8e4b7373cdabd7eeda97f))
- Golangci exhaustive by @mihakrumpestar ([6e411ac](https://github.com/mihakrumpestar/panix/commit/6e411ac436f24cc7f51e7e1d09a3aefcc6a0ce90))
- Golangci errorlint by @mihakrumpestar ([5888059](https://github.com/mihakrumpestar/panix/commit/58880598190c6ef26dd9c10d840e7e23bf737b9d))
- Golangci errcheck by @mihakrumpestar ([8759a5c](https://github.com/mihakrumpestar/panix/commit/8759a5c5e339b74c615261f9dbe9449ab46e0123))
- Golangci nilaway by @mihakrumpestar ([fea3c5b](https://github.com/mihakrumpestar/panix/commit/fea3c5b0a926af4ec00e627a1bbdd39a4e2470e7))
- Golangci err113 by @mihakrumpestar ([c24d21e](https://github.com/mihakrumpestar/panix/commit/c24d21eaed90428d3af030df48e55526e823e5ac))
- Golangci depguard by @mihakrumpestar ([e2b500c](https://github.com/mihakrumpestar/panix/commit/e2b500c94de41d4ae652d052fb0ad34544e98364))
- Golangci cyclop by @mihakrumpestar ([149ba26](https://github.com/mihakrumpestar/panix/commit/149ba260e5f42871e06098745ed13ce067d0e587))
- Updated deps and go by @mihakrumpestar ([2814abe](https://github.com/mihakrumpestar/panix/commit/2814abea9f0994300a0d5ad209ccf53573c74ab2))
- Golangci for prev commit by @mihakrumpestar ([f742d2d](https://github.com/mihakrumpestar/panix/commit/f742d2d9f69cf2b58cd638a67082cf8ffe5bc8ad))

### Performance

- Optimize buffer ops, time ops, scrollbar rendering, and spinner timeout by @mihakrumpestar ([3cd20a7](https://github.com/mihakrumpestar/panix/commit/3cd20a769fe8cd26d58aa1c630f1bd52aa4356ec))
- Add hash-based caching for stats table rendering, reduce CPU by 96.7%; this unfortunately removes spinners from stats table by @mihakrumpestar ([f0bdb7d](https://github.com/mihakrumpestar/panix/commit/f0bdb7d3f16557df45cbbe8c88eaf20ad32424a2))
- Add caching for footer, remove scroll percent by @mihakrumpestar ([4f4834a](https://github.com/mihakrumpestar/panix/commit/4f4834a9651564865c8c2dd1acc55c6331964e16))
- Add caching for viewports by @mihakrumpestar ([8357458](https://github.com/mihakrumpestar/panix/commit/8357458357741fffa954f5cf306be584927007f1))
- Add caching for phase status by @mihakrumpestar ([b5c638e](https://github.com/mihakrumpestar/panix/commit/b5c638ee2ec5f2916ba332579c6de2d1976d0b63))
- Improve performance in view_build_logs by @mihakrumpestar ([7609843](https://github.com/mihakrumpestar/panix/commit/7609843ac2f5f9144ae72e24cd5e65b47e4bdacc))

### Refactor

- Extract SIGINT handler setup into separate function with proper cleanup by @mihakrumpestar ([365c4b1](https://github.com/mihakrumpestar/panix/commit/365c4b16470064357a6621e56f96a406b530307d))
- Extract magic numbers to constants by @mihakrumpestar ([2e6cd65](https://github.com/mihakrumpestar/panix/commit/2e6cd65b6b690d3156e6f278a01e581edf35ad8e))
- Use PhaseRegistry as single source of truth for phase order/scope in build logs view by @mihakrumpestar ([45a2ddb](https://github.com/mihakrumpestar/panix/commit/45a2ddbfd1632461cc95b41d87dc1cdb88f77ce8))

### Security

- Replace os.Getenv(HOME) with os.UserHomeDir() by @mihakrumpestar ([ff92380](https://github.com/mihakrumpestar/panix/commit/ff92380693d8e85d6030fb303c5509adb5f149af))

## [0.0.3](https://github.com/mihakrumpestar/panix/compare/v0.0.2..v0.0.3) - 2026-03-01

### Bug Fixes

- Prevent logging because of default log file by @mihakrumpestar ([2774777](https://github.com/mihakrumpestar/panix/commit/2774777d52f6fa94d16f63d49477038d1fe46b05))
- Proper wait for kexec, add progress to kexec download and decompression by @mihakrumpestar ([69c6c5d](https://github.com/mihakrumpestar/panix/commit/69c6c5dbb3947d5d81f4edec8d46599905eff233))
- Git-cliff release by @mihakrumpestar ([d03c46c](https://github.com/mihakrumpestar/panix/commit/d03c46cd738158077bfd786567e896671554c631))
- Git-cliff release by @mihakrumpestar ([df81062](https://github.com/mihakrumpestar/panix/commit/df810625e9bbc92f64d081ed5eb077f38a9d7edd))

### Documentation

- Add assets by @mihakrumpestar ([bad2cfe](https://github.com/mihakrumpestar/panix/commit/bad2cfe53ce634c86acae1e521468e5862da0623))
- Devbox in CONTRIBUTIONG.md by @mihakrumpestar ([f1251ea](https://github.com/mihakrumpestar/panix/commit/f1251ead0120b61f4f2c36e0d46696a3581ae87e))
- Improve README and make flags help tags as descriptions in schema by @mihakrumpestar ([9fd1c05](https://github.com/mihakrumpestar/panix/commit/9fd1c051deb7206ef07648e644cc8cf9fa7b828e))
- Fix typos by @mihakrumpestar ([7bf207d](https://github.com/mihakrumpestar/panix/commit/7bf207dcc9e576196cafed0ea0cf00285d27a278))

### Features

- Improve README, fix: schema version parsing by @mihakrumpestar ([5355fa7](https://github.com/mihakrumpestar/panix/commit/5355fa7ec5ec6d196daea6f6b5c2034b98987485))
- Migrate back to devbox by @mihakrumpestar ([4fdba42](https://github.com/mihakrumpestar/panix/commit/4fdba42d7d91b5f19fcc90872fff91ec459c79d1))
- Add git-cliff for releases by @mihakrumpestar ([f3b4fd4](https://github.com/mihakrumpestar/panix/commit/f3b4fd459b55f261631d5e3c116f5bdf9e7f1817))

## [0.0.1](https://github.com/mihakrumpestar/panix/compare/v0.0.1) - 2026-02-22

### Bug Fixes

- Fix command output width by @mihakrumpestar ([f219f87](https://github.com/mihakrumpestar/panix/commit/f219f8796a7a279fba0ce35bead73fc1e810368a))

### MachineName

- String -> url.URL by @mihakrumpestar ([dcc72bc](https://github.com/mihakrumpestar/panix/commit/dcc72bcfaf08884ed584034297b5079bcbc6c4a0))

