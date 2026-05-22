# Changelog

## [0.7.0](https://github.com/mihakrumpestar/panix/compare/v0.6.0..v0.7.0) - 2026-05-22

### Bug Fixes

- Restore live flag in StartTimer so elapsed time updates after retry by [@mihakrumpestar](https://github.com/mihakrumpestar) ([8b285af](https://github.com/mihakrumpestar/panix/commit/8b285afad95b9b19fc567c10e9b084bef936b734))
- Build command by [@mihakrumpestar](https://github.com/mihakrumpestar) ([a920b67](https://github.com/mihakrumpestar/panix/commit/a920b678eb555669c6e3e27c920610ac7d7ac2fb))
- Seperate eval-cache path to prevent SQlite busy warnings, switch build to --print-out-paths from --json by [@mihakrumpestar](https://github.com/mihakrumpestar) ([708d8b6](https://github.com/mihakrumpestar/panix/commit/708d8b6fa9a0bd685b7bc3d2998204e5e93480b4))
- Slight changes to benchmarks, no change in results by [@mihakrumpestar](https://github.com/mihakrumpestar) ([9a74133](https://github.com/mihakrumpestar/panix/commit/9a741339d9bdc110fbd9bd67682f9657f7857198))
- Make SSH config read only machines left after filtering, fix SSH target resolution by [@mihakrumpestar](https://github.com/mihakrumpestar) ([7c09a47](https://github.com/mihakrumpestar/panix/commit/7c09a4700be903b0847c5694c9a040134bfe6da7))
- Prevent overwriting shared memory buffer of viewport in build_logs by [@mihakrumpestar](https://github.com/mihakrumpestar) ([3988150](https://github.com/mihakrumpestar/panix/commit/3988150f0a1e51b7c1cb1c99ac035180631cd402))
- Proper len for prev fix by [@mihakrumpestar](https://github.com/mihakrumpestar) ([52d3f41](https://github.com/mihakrumpestar/panix/commit/52d3f41087fc5d584d0e6f195722bcdba5ccbac3))
- Prevent SSH config errors on non-existant SSH config by [@mihakrumpestar](https://github.com/mihakrumpestar) ([b45b62e](https://github.com/mihakrumpestar/panix/commit/b45b62e3a2217b8c40b417353e4d93799c75d5cb))
- Fix all linter errors, use testify for all tests by [@mihakrumpestar](https://github.com/mihakrumpestar) ([cd6ad5f](https://github.com/mihakrumpestar/panix/commit/cd6ad5feb2ed60dbccc588589362611eb596e708))

### Documentation

- Add diagrams to readme by [@mihakrumpestar](https://github.com/mihakrumpestar) ([a58f768](https://github.com/mihakrumpestar/panix/commit/a58f76899a6d9be17921cd63e9b5a60e04926f19))

### Features

- Remote builds, custom build TUI library (zeroterm), benchmarks, e2e tests, fixes (#2) ([#2](https://github.com/mihakrumpestar/panix/pull/2)) by [@mihakrumpestar](https://github.com/mihakrumpestar) ([b014e7c](https://github.com/mihakrumpestar/panix/commit/b014e7cd77b94527d77c86fe16a83ba84b2cf979))
- Make flake URL accept uri|dir and make it optional, with default being '.' by [@mihakrumpestar](https://github.com/mihakrumpestar) ([4e3bcbd](https://github.com/mihakrumpestar/panix/commit/4e3bcbd3e310cbb90440a3fda7928df4733316a3))
- Improve workflow code by [@mihakrumpestar](https://github.com/mihakrumpestar) ([48fc208](https://github.com/mihakrumpestar/panix/commit/48fc208ad7a1e1c75db60e45dff6a7dac0427695))

### Miscellaneous

- Update deps by [@mihakrumpestar](https://github.com/mihakrumpestar) ([0808953](https://github.com/mihakrumpestar/panix/commit/0808953e98f04511dda557998df2e1e3fac8cbf9))
- Verify e2e tests by [@mihakrumpestar](https://github.com/mihakrumpestar) ([c21a177](https://github.com/mihakrumpestar/panix/commit/c21a1774e7b4e88b9de6fe2094f1698eae03766d))

## [0.6.0](https://github.com/mihakrumpestar/panix/compare/v0.5.0..v0.6.0) - 2026-04-25

### Bug Fixes

- Properly format merge commits in changelog by [@mihakrumpestar](https://github.com/mihakrumpestar) ([33f512e](https://github.com/mihakrumpestar/panix/commit/33f512ed5832c5ea1145d71440ae7a80eab0d3fe))
- Fix description, license and maintainer in flake by [@mihakrumpestar](https://github.com/mihakrumpestar) ([86951dc](https://github.com/mihakrumpestar/panix/commit/86951dc0434242e7e806b475a259bbdfd3c152b7))
- Security: make kexec ssh use StrictKeyChecking and DisableAutoAddHostKey from parent instead of hardcoding them by [@mihakrumpestar](https://github.com/mihakrumpestar) ([ca08797](https://github.com/mihakrumpestar/panix/commit/ca08797476d9dee48ac4dd14df0bb110ea17d33a))
- Security: make KnownHostsFile customizable and enable strict key checking by [@mihakrumpestar](https://github.com/mihakrumpestar) ([d2890c5](https://github.com/mihakrumpestar/panix/commit/d2890c5402f7a1052832356fef477427a9773537))

### Documentation

- Segment features and add internal/pkg readme by [@mihakrumpestar](https://github.com/mihakrumpestar) ([b90df1a](https://github.com/mihakrumpestar/panix/commit/b90df1a3d01e18bd7bbdb72b419ec1d691e10b5a))
- Make slogan consistent by [@mihakrumpestar](https://github.com/mihakrumpestar) ([2023ed5](https://github.com/mihakrumpestar/panix/commit/2023ed5d855fc632788cef187827f787dbb5bebb))

### Features

- Add first tests by [@mihakrumpestar](https://github.com/mihakrumpestar) ([ae220d5](https://github.com/mihakrumpestar/panix/commit/ae220d5fb20e5bbdc4dc4e38b6c9b28539e053be))
- Add new eval command, rename previous eval to template; extract commands into standalone/workflow packages; add validation subsystem for paths by [@mihakrumpestar](https://github.com/mihakrumpestar) ([5529bff](https://github.com/mihakrumpestar/panix/commit/5529bff4a09e339e7e198b0485af4d946013e9cb))
- Add e2e tests by [@mihakrumpestar](https://github.com/mihakrumpestar) ([dc3efa3](https://github.com/mihakrumpestar/panix/commit/dc3efa34da268239dd0aa0ba085b21610f858061))
- Autogenerate loc badge by [@mihakrumpestar](https://github.com/mihakrumpestar) ([7f19be0](https://github.com/mihakrumpestar/panix/commit/7f19be0046582bbb1393791c2d245ff9df130ad7))

### Miscellaneous

- Update demo gif by [@mihakrumpestar](https://github.com/mihakrumpestar) ([60cad7e](https://github.com/mihakrumpestar/panix/commit/60cad7ef1ce80901231560aeef054a492a60ed1d))
- Update readme for prev commit by [@mihakrumpestar](https://github.com/mihakrumpestar) ([6f72daf](https://github.com/mihakrumpestar/panix/commit/6f72daf16aeef0807215695a945a01ce1d3947a9))

## [0.5.0](https://github.com/mihakrumpestar/panix/compare/v0.4.0..v0.5.0) - 2026-04-19

### Bug Fixes

- Make main viewport not follow bottom by [@mihakrumpestar](https://github.com/mihakrumpestar) ([7d4580b](https://github.com/mihakrumpestar/panix/commit/7d4580ba44ce5c4aa8f9336f607f3bfbd8b37870))

### Features

- Snapshot feature refactor (#1) ([#1](https://github.com/mihakrumpestar/panix/pull/1)) by [@mihakrumpestar](https://github.com/mihakrumpestar) ([35c8229](https://github.com/mihakrumpestar/panix/commit/35c822925dad282b64b54be4739dfd53ace78069))
- Improve cache for command output, improve/update readme by [@mihakrumpestar](https://github.com/mihakrumpestar) ([873bc09](https://github.com/mihakrumpestar/panix/commit/873bc095ad0293fe67cc7a059e6d9329a2990ba6))

## [0.4.0](https://github.com/mihakrumpestar/panix/compare/v0.3.0..v0.4.0) - 2026-04-09

### Bug Fixes

- Make rollback generation number an attribute instead of arg, since -1 gets detected as flag instead of value by [@mihakrumpestar](https://github.com/mihakrumpestar) ([c07230a](https://github.com/mihakrumpestar/panix/commit/c07230a8be01649c258e99010d8791ef376b4349))

### Features

- Add alternative rendering modes besides TUI: console and json by [@mihakrumpestar](https://github.com/mihakrumpestar) ([2f7599c](https://github.com/mihakrumpestar/panix/commit/2f7599cdf3675d86fbf9ad5bfb02576d4081c311))
- Rename config element: root -> fleet by [@mihakrumpestar](https://github.com/mihakrumpestar) ([7a459a3](https://github.com/mihakrumpestar/panix/commit/7a459a3c4c7ec5eb194b5ee92794bcbcedd162ac))
- Add init command, fix: rename references root -> fleet, chore: improve some readme sections by [@mihakrumpestar](https://github.com/mihakrumpestar) ([66ebfe7](https://github.com/mihakrumpestar/panix/commit/66ebfe7fa18738e573bf47a96e6f66fc1f1696e6))

## [0.3.0](https://github.com/mihakrumpestar/panix/compare/v0.2.0..v0.3.0) - 2026-04-07

### Bug Fixes

- Deploy command workflow description by [@mihakrumpestar](https://github.com/mihakrumpestar) ([3c4bde6](https://github.com/mihakrumpestar/panix/commit/3c4bde6237114ab5f9bb2583c9276d73dee1f0d4))
- Make Running color scheme more visible, make rollback standalone phase work by [@mihakrumpestar](https://github.com/mihakrumpestar) ([45ce617](https://github.com/mihakrumpestar/panix/commit/45ce617c2f1f16530d37e6be2ca10a5f7cd34832))
- Proper global and subcommand flag separation by [@mihakrumpestar](https://github.com/mihakrumpestar) ([80b4b59](https://github.com/mihakrumpestar/panix/commit/80b4b5951d1377b1f2fe83a0612fee8da61af793))
- Add gh check to release by [@mihakrumpestar](https://github.com/mihakrumpestar) ([56eaf14](https://github.com/mihakrumpestar/panix/commit/56eaf14def8b1323295d4f4cd5a2d572dae14c6a))

### Documentation

- Improve docs by [@mihakrumpestar](https://github.com/mihakrumpestar) ([9919b00](https://github.com/mihakrumpestar/panix/commit/9919b003908de43b0ae3ceb8afe1a59a7fcd58e5))

### Features

- Added Golang template engine support, fix: golangci-lint, feat: improve CI workflow by [@mihakrumpestar](https://github.com/mihakrumpestar) ([22de339](https://github.com/mihakrumpestar/panix/commit/22de339b0a7ed935ecbc8c80ff8368d7345b4d0f))
- Added support for references in yaml schema by [@mihakrumpestar](https://github.com/mihakrumpestar) ([145f53f](https://github.com/mihakrumpestar/panix/commit/145f53fea0efac574f7febd9d3741de42d5e4e76))
- Removed Bootstrap.DisableAuto flag, made SSH configs deterministic by [@mihakrumpestar](https://github.com/mihakrumpestar) ([01e5333](https://github.com/mihakrumpestar/panix/commit/01e53335fc69f1f8f93e19c5a72fcac9ee36a74f))
- Filter out unused phases (bootstrap, secrets) that are optional, chore: split loader into more smaller files by [@mihakrumpestar](https://github.com/mihakrumpestar) ([815bf7c](https://github.com/mihakrumpestar/panix/commit/815bf7c54c68e69bfe5b85fafc8912c621a3d529))

### Miscellaneous

- Update deps, fix PhaseStateActive colors by [@mihakrumpestar](https://github.com/mihakrumpestar) ([0d6331e](https://github.com/mihakrumpestar/panix/commit/0d6331e769a41634032e37a2a305a0a7646e6719))
- Updated demo by [@mihakrumpestar](https://github.com/mihakrumpestar) ([cd96d57](https://github.com/mihakrumpestar/panix/commit/cd96d57b63831e41a0e5da6dde4f12984192173b))
- Some testing with alternative builder by [@mihakrumpestar](https://github.com/mihakrumpestar) ([4df1852](https://github.com/mihakrumpestar/panix/commit/4df18525511680816a4f0a4769a71ce263fca698))

## [0.2.0](https://github.com/mihakrumpestar/panix/compare/v0.1.3..v0.2.0) - 2026-03-31

### Bug Fixes

- Skip version bump commit from release logs by [@mihakrumpestar](https://github.com/mihakrumpestar) ([ee97f53](https://github.com/mihakrumpestar/panix/commit/ee97f53c5563430c02d361996d5f85acc8a58426))
- Make WaitForReconnect properly wait with WaitForReconnectCheckInterval by [@mihakrumpestar](https://github.com/mihakrumpestar) ([c0039d4](https://github.com/mihakrumpestar/panix/commit/c0039d4b2d2c4c5037f301c69307cde9345d9cf9))
- PhasesInOrder now does not contain rollback, correctly handle skipped phases by [@mihakrumpestar](https://github.com/mihakrumpestar) ([0617f67](https://github.com/mihakrumpestar/panix/commit/0617f678ffa5f80d78961a4b919b6b6e770fdc81))

### Features

- Added more feature docs, kexec ssh port option and switching by [@mihakrumpestar](https://github.com/mihakrumpestar) ([8e08437](https://github.com/mihakrumpestar/panix/commit/8e084376ea804ae4bf382ecef3494c3692d4401f))
- Improve pre-commit and generate workflows by [@mihakrumpestar](https://github.com/mihakrumpestar) ([4973894](https://github.com/mihakrumpestar/panix/commit/497389401bc4deed1f32feca2ec03c9b0e2a0fd8))
- Remove explicit bootstrap phase by [@mihakrumpestar](https://github.com/mihakrumpestar) ([2bd40cb](https://github.com/mihakrumpestar/panix/commit/2bd40cb86d11205b20e6a717ce976d6e0bb19e14))
- Make timout per command instead of per workflow by [@mihakrumpestar](https://github.com/mihakrumpestar) ([3f563a9](https://github.com/mihakrumpestar/panix/commit/3f563a920df8843ffa3d690b71fbb1265caaa393))
- Added more activation modes by [@mihakrumpestar](https://github.com/mihakrumpestar) ([5a3167b](https://github.com/mihakrumpestar/panix/commit/5a3167b98ff1f96c218d3d3997bd2e918e051d97))
- Added rollback command by [@mihakrumpestar](https://github.com/mihakrumpestar) ([1119972](https://github.com/mihakrumpestar/panix/commit/1119972b22f50edbc98346a20f9dc09fa1a0858a))
- Added configuration options to specify nix command flags by [@mihakrumpestar](https://github.com/mihakrumpestar) ([7022d56](https://github.com/mihakrumpestar/panix/commit/7022d564c1dca4048e0eca923eb1eba1e589815f))

## [0.1.3](https://github.com/mihakrumpestar/panix/compare/v0.1.2..v0.1.3) - 2026-03-24

### Bug Fixes

- Allow nix experimental commands to run anywhere by enabling experimental features by [@mihakrumpestar](https://github.com/mihakrumpestar) ([01c1320](https://github.com/mihakrumpestar/panix/commit/01c1320a45fd17a7bff888a6c9b09dfc7ee3c8bd))
- Scrollbar to use wrapped content height, add missing zone marker for fullscreen viewport focus, remove fixed width of 3 for stats table symbol by [@mihakrumpestar](https://github.com/mihakrumpestar) ([042ff8a](https://github.com/mihakrumpestar/panix/commit/042ff8a6767f157d0072862a634233e341d1f453))
- Revert phase status status icon back to fixed width by [@mihakrumpestar](https://github.com/mihakrumpestar) ([3a70825](https://github.com/mihakrumpestar/panix/commit/3a708259985e96b1e1d7212c0bda32d45cd908d8))

### Documentation

- Add new way of funding/sponsoring by [@mihakrumpestar](https://github.com/mihakrumpestar) ([33995ca](https://github.com/mihakrumpestar/panix/commit/33995ca06234b0ad2e32d6bfc0398b28409cb2ca))
- Add a detailed explanation of Secrets phase by [@mihakrumpestar](https://github.com/mihakrumpestar) ([07ba1b6](https://github.com/mihakrumpestar/panix/commit/07ba1b632bd8a3d2395ac2d39901ffa3d84aaa85))

### Miscellaneous

- Update deps by [@mihakrumpestar](https://github.com/mihakrumpestar) ([8a7c700](https://github.com/mihakrumpestar/panix/commit/8a7c70093a98bef0a98f6362976a3e37dcc21744))
- Update golang to 1.26.1, update flake nixpkgs, fix minor golangci issues by [@mihakrumpestar](https://github.com/mihakrumpestar) ([cdfe645](https://github.com/mihakrumpestar/panix/commit/cdfe645050954190c871a2256af20894d3db4b2c))

## [0.1.2](https://github.com/mihakrumpestar/panix/compare/v0.1.1..v0.1.2) - 2026-03-12

### Bug Fixes

- Add space before commit and PR links by [@mihakrumpestar](https://github.com/mihakrumpestar) ([08554b1](https://github.com/mihakrumpestar/panix/commit/08554b15a799f9183f5c7d8404789da0250af238))
- Make remote SSH terminal not to allocate PTY which may cause Kexec to fail by [@mihakrumpestar](https://github.com/mihakrumpestar) ([0e1498a](https://github.com/mihakrumpestar/panix/commit/0e1498a711a3934be272bf43ce2acc88b793ddc2))
- Revert back to full whitespace trim for command outputs by [@mihakrumpestar](https://github.com/mihakrumpestar) ([b503334](https://github.com/mihakrumpestar/panix/commit/b50333427f86101d3fea7bc329bfd94656094ed0))

### Documentation

- Add version badge by [@mihakrumpestar](https://github.com/mihakrumpestar) ([7f05424](https://github.com/mihakrumpestar/panix/commit/7f0542415308e031e81534894ff42ddd63de95c5))
- Add more README badges by [@mihakrumpestar](https://github.com/mihakrumpestar) ([46e3f05](https://github.com/mihakrumpestar/panix/commit/46e3f05c20a9408b309437cdc21209d915a7d59b))
- Improve README, remove funding (since it has not yet been set up) by [@mihakrumpestar](https://github.com/mihakrumpestar) ([6b35664](https://github.com/mihakrumpestar/panix/commit/6b35664132f99caf6200fc6c2ce46079177a0da9))
- Add bootstrap with kexec demo video by [@mihakrumpestar](https://github.com/mihakrumpestar) ([e80dce1](https://github.com/mihakrumpestar/panix/commit/e80dce121dc026ca9904be083359afd18d163c92))
- Fix kexec demo by [@mihakrumpestar](https://github.com/mihakrumpestar) ([e7b80c6](https://github.com/mihakrumpestar/panix/commit/e7b80c6e9bdd5307193e6d43a9e6392ba6925043))
- Fix kexec demo 2 by [@mihakrumpestar](https://github.com/mihakrumpestar) ([78d9207](https://github.com/mihakrumpestar/panix/commit/78d92074ef294c211cf6fce2e529f257710f5a4e))
- Fix kexec demo 3 by [@mihakrumpestar](https://github.com/mihakrumpestar) ([0a523ea](https://github.com/mihakrumpestar/panix/commit/0a523eaabf4f4cf5c15fb25fa95acf7002b34230))
- Update ffmpeg commands by [@mihakrumpestar](https://github.com/mihakrumpestar) ([b5c75fe](https://github.com/mihakrumpestar/panix/commit/b5c75fe420e941c7079f9976928f6bdd46a506a7))

### Features

- Add force_bootstrap and force_bootstrap_kexec options by [@mihakrumpestar](https://github.com/mihakrumpestar) ([c811d02](https://github.com/mihakrumpestar/panix/commit/c811d02e528af2e8a0b2e91e0a48aacedd528240))

### Miscellaneous

- Add new demo video and pic by [@mihakrumpestar](https://github.com/mihakrumpestar) ([edc0bac](https://github.com/mihakrumpestar/panix/commit/edc0bac372560a697f346b6d643c615d46943b68))

## [0.1.1](https://github.com/mihakrumpestar/panix/compare/v0.1.0..v0.1.1) - 2026-03-10

### Bug Fixes

- Make Cliff add links to commits and PRs by [@mihakrumpestar](https://github.com/mihakrumpestar) ([5b57a49](https://github.com/mihakrumpestar/panix/commit/5b57a49c671a8baa009eddf06bfda8ed4853cef0))

## [0.1.0](https://github.com/mihakrumpestar/panix/compare/v0.0.3..v0.1.0) - 2026-03-10

### Bug Fixes

- Add missing permissions to secrets struct, fix: make secrets transefer not use SSH, fix: task release to go generate after new version is written instead of later by [@mihakrumpestar](https://github.com/mihakrumpestar) ([26e11b5](https://github.com/mihakrumpestar/panix/commit/26e11b517e8f1a299cdc9de9a6b2418cd90e7451))
- Add context cancellation in PTY loop, make ActiveSSH thread-safe with atomic pointer, optimize slice appends and xpath building, and stream config loading by [@mihakrumpestar](https://github.com/mihakrumpestar) ([6a996ba](https://github.com/mihakrumpestar/panix/commit/6a996ba5014699a0dfad892c3dd8eee219ff736a))
- Move animation state to PhaseStatus with atomic fields, limit worker pool to 1000 goroutines, and optimize string concatenation with strings.Builder by [@mihakrumpestar](https://github.com/mihakrumpestar) ([1064d23](https://github.com/mihakrumpestar/panix/commit/1064d23a9e78b9d4312fa31fa22086d2269a3bb6))
- Improve error handling with proper wrapping, context, and panic messages by [@mihakrumpestar](https://github.com/mihakrumpestar) ([06754ff](https://github.com/mihakrumpestar/panix/commit/06754fff16c4789acb828da24f02b7c52e655763))
- Resource leaks, race conditions, and improve error handling by [@mihakrumpestar](https://github.com/mihakrumpestar) ([794b15c](https://github.com/mihakrumpestar/panix/commit/794b15c2bd25d41260793f5add3784e6b37fd988))
- Connection leak, infinite loop on shutdown, add bounds checks, and use type-safe omap by [@mihakrumpestar](https://github.com/mihakrumpestar) ([d4f41d9](https://github.com/mihakrumpestar/panix/commit/d4f41d91c4a8e956811c05c5946d47dc7af960f0))
- Correct MachineCount to return count instead of last index, pass kexec-extra-flags as separate arguments to exec.Command, security: validate URLs have http/https scheme and host, fix: close CPU profile file after stopping profiler, refactor: remove unused Pty field from CommandLog by [@mihakrumpestar](https://github.com/mihakrumpestar) ([5dc2441](https://github.com/mihakrumpestar/panix/commit/5dc2441468b30ebd42c020ff9bf1103c2fafead9))
- Make command error be a viewport so it wraps if over-width by [@mihakrumpestar](https://github.com/mihakrumpestar) ([e00f4ae](https://github.com/mihakrumpestar/panix/commit/e00f4ae511848edb84249e673c76030b0c3dcd7d))
- Build logs right timers indentation and don't repeat machine labels by [@mihakrumpestar](https://github.com/mihakrumpestar) ([493a653](https://github.com/mihakrumpestar/panix/commit/493a653c9799f6cfc492a94496c50444e4936e89))
- Filter out json output for drv paths from being rendered in build logs by [@mihakrumpestar](https://github.com/mihakrumpestar) ([56abb66](https://github.com/mihakrumpestar/panix/commit/56abb66745d660ae7cfb7640f3b31b5a0e17524a))
- Release in Taskfile by [@mihakrumpestar](https://github.com/mihakrumpestar) ([3251d74](https://github.com/mihakrumpestar/panix/commit/3251d74413cbe5100195d8487682ee2c21faaf8c))

### Documentation

- Add DiskEncryptionKeys description in schema by [@mihakrumpestar](https://github.com/mihakrumpestar) ([3f68029](https://github.com/mihakrumpestar/panix/commit/3f680290c48951d6f111a36e2de1259f1aa1f144))
- Update docs by [@mihakrumpestar](https://github.com/mihakrumpestar) ([758a0bb](https://github.com/mihakrumpestar/panix/commit/758a0bb9b087ca586ee83512abe5cf44c0fee9a9))

### Features

- Add bootstrap SSH, post-bootstrap hooks, and rename SSH options, fix: copy to clipboard now copys entire content, not just the visible part of viewport by [@mihakrumpestar](https://github.com/mihakrumpestar) ([dbad3e8](https://github.com/mihakrumpestar/panix/commit/dbad3e86248273168b9101578c169be72ce003ce))
- Make kexec part of bootstrap process instead of inspect by [@mihakrumpestar](https://github.com/mihakrumpestar) ([53811b0](https://github.com/mihakrumpestar/panix/commit/53811b0fcc1211dab265e482114644be2d3a1df2))
- Migrate to bubbletea v2 and lipgloss v2: update View() to return tea.View, replace KeyMsg/MouseMsg with v2 types, change viewport fields to methods by [@mihakrumpestar](https://github.com/mihakrumpestar) ([f8f3e71](https://github.com/mihakrumpestar/panix/commit/f8f3e71494a39d92f99b8fa9c3092ee430f587b4))
- Add golangci by [@mihakrumpestar](https://github.com/mihakrumpestar) ([56cdcfb](https://github.com/mihakrumpestar/panix/commit/56cdcfbc5e46eb03e41eac84d041259af1a36864))

### Miscellaneous

- Golangci wsl by [@mihakrumpestar](https://github.com/mihakrumpestar) ([141aac5](https://github.com/mihakrumpestar/panix/commit/141aac578a0a56b3a2bad925d75c4accbc1491a7))
- Golangci wrapcheck by [@mihakrumpestar](https://github.com/mihakrumpestar) ([b9db9eb](https://github.com/mihakrumpestar/panix/commit/b9db9ebebd0b44623c6dd3df902cd2168e951bf5))
- Golangci varnamelen by [@mihakrumpestar](https://github.com/mihakrumpestar) ([f4c8573](https://github.com/mihakrumpestar/panix/commit/f4c85731a5595ec1abb57386d63c7194ded96979))
- Golangci staticcheck by [@mihakrumpestar](https://github.com/mihakrumpestar) ([404df0d](https://github.com/mihakrumpestar/panix/commit/404df0d5e35db6e132e2d809b8ec28ac419202c1))
- Golangci perfsprint by [@mihakrumpestar](https://github.com/mihakrumpestar) ([823a7a5](https://github.com/mihakrumpestar/panix/commit/823a7a57d02f5763bddfce75e13b9e2a6a9efe67))
- Golangci nlreturn by [@mihakrumpestar](https://github.com/mihakrumpestar) ([e2af6fa](https://github.com/mihakrumpestar/panix/commit/e2af6fab1325e1b9819feddb62dbde556adf5356))
- Golangci nilaway by [@mihakrumpestar](https://github.com/mihakrumpestar) ([6eed648](https://github.com/mihakrumpestar/panix/commit/6eed648862ce4956af830d32d80d0f2d65691999))
- Golangci nestif by [@mihakrumpestar](https://github.com/mihakrumpestar) ([2a78394](https://github.com/mihakrumpestar/panix/commit/2a78394be1a8cbd7b5c7f4e9b6a2554c5dbd1e78))
- Golangci mnd by [@mihakrumpestar](https://github.com/mihakrumpestar) ([5d32ddc](https://github.com/mihakrumpestar/panix/commit/5d32ddc8a1889359cb6682248ed3bc2115073999))
- Golangci lll by [@mihakrumpestar](https://github.com/mihakrumpestar) ([20714ab](https://github.com/mihakrumpestar/panix/commit/20714ab48d3d113d2ae8c2fd7f2a8a60e85e596c))
- Golangci intrange by [@mihakrumpestar](https://github.com/mihakrumpestar) ([ffdb03c](https://github.com/mihakrumpestar/panix/commit/ffdb03cf4da45b6a3a068b7915273b7bbfc41451))
- Golangci gosec by [@mihakrumpestar](https://github.com/mihakrumpestar) ([571b884](https://github.com/mihakrumpestar/panix/commit/571b884674f5ba1854f6291369438bd472d541ed))
- Golangci godot by [@mihakrumpestar](https://github.com/mihakrumpestar) ([703ac2a](https://github.com/mihakrumpestar/panix/commit/703ac2adc04509a4fa409ba8542473a62c9f451b))
- Golangci gocritic by [@mihakrumpestar](https://github.com/mihakrumpestar) ([7195669](https://github.com/mihakrumpestar/panix/commit/719566971f098b681bad975a482c453f05f3b98f))
- Golangci gocognit by [@mihakrumpestar](https://github.com/mihakrumpestar) ([70a08ad](https://github.com/mihakrumpestar/panix/commit/70a08ad54314bffd4769e5570001cb3ae4c4044b))
- Golangci varnamelen by [@mihakrumpestar](https://github.com/mihakrumpestar) ([c52b9e3](https://github.com/mihakrumpestar/panix/commit/c52b9e3ce57a6d3cdc68b1acc05b150f3cccb661))
- Golangci gochecknoinits by [@mihakrumpestar](https://github.com/mihakrumpestar) ([54c6be8](https://github.com/mihakrumpestar/panix/commit/54c6be8bdc559573e22933f90d089235f4ad5b76))
- Golangci funlen by [@mihakrumpestar](https://github.com/mihakrumpestar) ([6b8d410](https://github.com/mihakrumpestar/panix/commit/6b8d4106170883decce47b74028527dc35ebebce))
- Golangci forcetypeassert by [@mihakrumpestar](https://github.com/mihakrumpestar) ([59f87e5](https://github.com/mihakrumpestar/panix/commit/59f87e501fa2fb94b3a705f18ec825d54f0ee883))
- Golangci exhaustive by [@mihakrumpestar](https://github.com/mihakrumpestar) ([d06fa76](https://github.com/mihakrumpestar/panix/commit/d06fa76fe183a093466be83d3678b91126f4a37c))
- Golangci errorlint by [@mihakrumpestar](https://github.com/mihakrumpestar) ([39fab0d](https://github.com/mihakrumpestar/panix/commit/39fab0dac7e9731d0c658627bcfe5539e62e705a))
- Golangci errcheck by [@mihakrumpestar](https://github.com/mihakrumpestar) ([43fb4f4](https://github.com/mihakrumpestar/panix/commit/43fb4f46b991dbc5d9b9013d0922c9747a2bcd1d))
- Golangci nilaway by [@mihakrumpestar](https://github.com/mihakrumpestar) ([192e2a7](https://github.com/mihakrumpestar/panix/commit/192e2a72fcc901dae8d01bde6ef0ac581c4e71a8))
- Golangci err113 by [@mihakrumpestar](https://github.com/mihakrumpestar) ([399aaa6](https://github.com/mihakrumpestar/panix/commit/399aaa6761b99107471169c274da928262e7cd50))
- Golangci depguard by [@mihakrumpestar](https://github.com/mihakrumpestar) ([732b403](https://github.com/mihakrumpestar/panix/commit/732b4035fdb8603e1e27a6d2b62aea3ee156eb33))
- Golangci cyclop by [@mihakrumpestar](https://github.com/mihakrumpestar) ([46090c5](https://github.com/mihakrumpestar/panix/commit/46090c5dd7355d30c7eae313ee148f63ed93dfc2))
- Updated deps and go by [@mihakrumpestar](https://github.com/mihakrumpestar) ([faa6086](https://github.com/mihakrumpestar/panix/commit/faa608650a78e68a9108064c64b9b53caa853f3c))
- Golangci for prev commit by [@mihakrumpestar](https://github.com/mihakrumpestar) ([68a70b6](https://github.com/mihakrumpestar/panix/commit/68a70b64f7d3dfe98c928e42f9b72cf21f159594))

### Performance

- Optimize buffer ops, time ops, scrollbar rendering, and spinner timeout by [@mihakrumpestar](https://github.com/mihakrumpestar) ([ca9fe67](https://github.com/mihakrumpestar/panix/commit/ca9fe67ede18987ef891e2275dd966246cbc99e5))
- Add hash-based caching for stats table rendering, reduce CPU by 96.7%; this unfortunately removes spinners from stats table by [@mihakrumpestar](https://github.com/mihakrumpestar) ([afee6fc](https://github.com/mihakrumpestar/panix/commit/afee6fcdfd57c867ececa81a6b67d68b1df6b3d9))
- Add caching for footer, remove scroll percent by [@mihakrumpestar](https://github.com/mihakrumpestar) ([9a69204](https://github.com/mihakrumpestar/panix/commit/9a6920466e1d0d36a6d953552bf18b79ae1eada9))
- Add caching for viewports by [@mihakrumpestar](https://github.com/mihakrumpestar) ([008da0d](https://github.com/mihakrumpestar/panix/commit/008da0d2677f3b52039a98f3977276937b8b258e))
- Add caching for phase status by [@mihakrumpestar](https://github.com/mihakrumpestar) ([0a4e2c0](https://github.com/mihakrumpestar/panix/commit/0a4e2c0d1818c900387ed32eb13fe519fac22cdb))
- Improve performance in view_build_logs by [@mihakrumpestar](https://github.com/mihakrumpestar) ([7299a29](https://github.com/mihakrumpestar/panix/commit/7299a299de4aafb85e98e7f27a72820681b2c1f3))

### Refactor

- Extract SIGINT handler setup into separate function with proper cleanup by [@mihakrumpestar](https://github.com/mihakrumpestar) ([947b40f](https://github.com/mihakrumpestar/panix/commit/947b40fee1e0c41e90edba216c0b31b75a033292))
- Extract magic numbers to constants by [@mihakrumpestar](https://github.com/mihakrumpestar) ([c59e012](https://github.com/mihakrumpestar/panix/commit/c59e012d073b62df34723ef684461048ead7a49b))
- Use PhaseRegistry as single source of truth for phase order/scope in build logs view by [@mihakrumpestar](https://github.com/mihakrumpestar) ([b2ee5ef](https://github.com/mihakrumpestar/panix/commit/b2ee5efbb975b9a07c4a492718707c4d84606985))

### Security

- Replace os.Getenv(HOME) with os.UserHomeDir() by [@mihakrumpestar](https://github.com/mihakrumpestar) ([d704427](https://github.com/mihakrumpestar/panix/commit/d704427ef16ec98e331dc27613ea284a2455be15))

## [0.0.3](https://github.com/mihakrumpestar/panix/compare/v0.0.2..v0.0.3) - 2026-03-01

### Bug Fixes

- Prevent logging because of default log file by [@mihakrumpestar](https://github.com/mihakrumpestar) ([0c13901](https://github.com/mihakrumpestar/panix/commit/0c13901c9bee4d9383e3185b4f0a9044877f309d))
- Proper wait for kexec, add progress to kexec download and decompression by [@mihakrumpestar](https://github.com/mihakrumpestar) ([c306f83](https://github.com/mihakrumpestar/panix/commit/c306f83081a2e7938b511f3768c25c719a458e48))
- Git-cliff release by [@mihakrumpestar](https://github.com/mihakrumpestar) ([ad56b7e](https://github.com/mihakrumpestar/panix/commit/ad56b7e69b621539d9ce5788849c6ca3f5ca2ffc))
- Git-cliff release by [@mihakrumpestar](https://github.com/mihakrumpestar) ([5df39c6](https://github.com/mihakrumpestar/panix/commit/5df39c66b02a6b4461fa368bac2ab02642e79303))

### Documentation

- Add assets by [@mihakrumpestar](https://github.com/mihakrumpestar) ([2bf7227](https://github.com/mihakrumpestar/panix/commit/2bf7227bbc12efd71fe8b86142e4d1d42221aa52))
- Devbox in CONTRIBUTIONG.md by [@mihakrumpestar](https://github.com/mihakrumpestar) ([f8eed68](https://github.com/mihakrumpestar/panix/commit/f8eed68b9e4937d37ccc33750bc48d98c069f00c))
- Improve README and make flags help tags as descriptions in schema by [@mihakrumpestar](https://github.com/mihakrumpestar) ([ee03deb](https://github.com/mihakrumpestar/panix/commit/ee03deb75c7834e618b39fcf340ce8ff6c3b0e45))
- Fix typos by [@mihakrumpestar](https://github.com/mihakrumpestar) ([faacdee](https://github.com/mihakrumpestar/panix/commit/faacdee2a8d1ba98c1b1f56f4b0606a0d78bfe07))

### Features

- Improve README, fix: schema version parsing by [@mihakrumpestar](https://github.com/mihakrumpestar) ([52645a0](https://github.com/mihakrumpestar/panix/commit/52645a0f59a7ccba6f74b60bfeabe84c0d096ce8))
- Migrate back to devbox by [@mihakrumpestar](https://github.com/mihakrumpestar) ([45379db](https://github.com/mihakrumpestar/panix/commit/45379db83a117f7c1697316f520f259983ac5ef2))
- Add git-cliff for releases by [@mihakrumpestar](https://github.com/mihakrumpestar) ([02d8908](https://github.com/mihakrumpestar/panix/commit/02d89087dfcc97f5c60802c0e4068c373fc517ad))

## [0.0.1](https://github.com/mihakrumpestar/panix/compare/v0.0.1) - 2026-02-22

### Bug Fixes

- Fix command output width by [@mihakrumpestar](https://github.com/mihakrumpestar) ([83c31fc](https://github.com/mihakrumpestar/panix/commit/83c31fce14b57cd1bfb7eec5c79c8ba723952c13))

