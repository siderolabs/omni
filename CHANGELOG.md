## [Omni 1.1.0](https://github.com/siderolabs/omni/releases/tag/v1.1.0) (2025-07-24)

Welcome to the v1.1.0 release of Omni!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Multiple Join Token Support

Omni now supports multiple SideroLink join tokens.
It now creates the default non-expiring token, then the user can create more tokens and delete the old ones.


### Config Change History

There is now `MachineConfigDiffs.omni.sidero.dev` resource that keeps the history of
each machine config changes.
It keeps up to 1000 diffs for the last 30 days.


### Contributors

* Artem Chernyshev
* Andrey Smirnov
* Utku Ozdemir
* Oguz Kilcan
* Mateusz Urbanek

### Changes
<details><summary>20 commits</summary>
<p>

* [`c761e347`](https://github.com/siderolabs/omni/commit/c761e34765539bba31f47f1d395387513005a34c) release(v1.0.0-beta.1): prepare release
* [`28158ed8`](https://github.com/siderolabs/omni/commit/28158ed855513548aa4d78bc7e0cf91e6f6d3dc3) fix: ignore `MachineStatus` having no TalosVersion in DNS service
* [`da3f28f6`](https://github.com/siderolabs/omni/commit/da3f28f6b1f0a01c5a90216fa4b5e7aa3c780ac0) chore: support encoding extra docs in `siderolink.RenderJoinConfig`
* [`f4582116`](https://github.com/siderolabs/omni/commit/f458211621f30c2a07720a4773dbba49d2967ce7) fix: allow encoding join tokens using v1 version
* [`80ff037a`](https://github.com/siderolabs/omni/commit/80ff037a84d5e75c28a4836ac258f8ad9ec9fb36) fix: do not try to encode `v1/v2` tokens in siderolink.NewJoinToken
* [`7b7c021d`](https://github.com/siderolabs/omni/commit/7b7c021da8b911780635d86e19baac78e814516e) fix: do not create `JoinTokenUsage` for `PendingMachines`
* [`2c4f34a7`](https://github.com/siderolabs/omni/commit/2c4f34a7da44c3318da326e116a6bb8f5ccd1e65) fix: fix etcd status check in control plane status
* [`b0f76343`](https://github.com/siderolabs/omni/commit/b0f76343100033927a40ea0e604d5be8a84b3592) feat: implement the API for reading resources and their dependency graph
* [`e945cc7b`](https://github.com/siderolabs/omni/commit/e945cc7b8b342f5f79ecc822edd50a57d69d9210) release(v1.0.0-beta.0): prepare release
* [`a2722856`](https://github.com/siderolabs/omni/commit/a27228563a6a649641e885cd39ef55e88d5402c5) chore: enable gRPC keepalive in the Omni client
* [`f8de9a6d`](https://github.com/siderolabs/omni/commit/f8de9a6d96453bf3e5b9f33668fa35169ada24c0) feat: add support for imported cluster secrets
* [`753259c2`](https://github.com/siderolabs/omni/commit/753259c26edd3e1f61a98a69d334212fa7a9a03b) fix: do not filter out noop events in the infra provider state
* [`ab1f7cc7`](https://github.com/siderolabs/omni/commit/ab1f7cc7fab1d111d561dbf0f2239c169bada5aa) feat: implement multiple token support and token management
* [`0e76483b`](https://github.com/siderolabs/omni/commit/0e76483bab6b9f377bf3e3779f3d02d284a9a782) chore: rekres, bump deps, Go, Talos and k8s versions, satisfy linters
* [`e1c1aaea`](https://github.com/siderolabs/omni/commit/e1c1aaea7a304b2efbd318280afe6afbd18487ab) fix: add validation of k8s version
* [`f1b47f08`](https://github.com/siderolabs/omni/commit/f1b47f08d9a808f5aa635d4ff5b642986305d8a2) feat: log and store redacted machine config diffs
* [`a7ac6372`](https://github.com/siderolabs/omni/commit/a7ac63725d0c8de5cdfc6620607f072a032e383a) chore: rewrite join config generation
* [`66c7897b`](https://github.com/siderolabs/omni/commit/66c7897bb8a657e6d7c391dfa7f35401d9a1d123) chore: update zstd module `go.mod` deps
* [`3b701483`](https://github.com/siderolabs/omni/commit/3b7014839a91ef152b7b82c5b8ae0020ea549e31) test: reduce the log verbosity in unit tests
* [`ff32ae4c`](https://github.com/siderolabs/omni/commit/ff32ae4c7f660a0a1c2e8159e0f9bf52c4b76955) fix: gracefully handle logServer shutdown
</p>
</details>

### Changes from siderolabs/crypto
<details><summary>5 commits</summary>
<p>

* [`62a079b`](https://github.com/siderolabs/crypto/commit/62a079b6915dc6dff602dca835fafbeabb6adbce) fix: update TLS config, add tests for TLS interactions
* [`c2b4e26`](https://github.com/siderolabs/crypto/commit/c2b4e26d7d7e45e8269a040fb0251446354ba8ef) fix: remove code duplication and fix Ed255119 CA generation
* [`2a07632`](https://github.com/siderolabs/crypto/commit/2a076326bbdd3da61460197a3fa1a0484a347478) fix: enforce FIPS-140-3 compliance
* [`17107ae`](https://github.com/siderolabs/crypto/commit/17107ae45403a2bcd4fecfb4660b60276652b00d) fix: add generic CSR generator and OpenSSL interop
* [`53659fc`](https://github.com/siderolabs/crypto/commit/53659fc35f6abd4ada7ffa22ef1b148cf93c0f28) refactor: split into files
</p>
</details>

### Dependency Changes

* **github.com/cosi-project/runtime**            v0.10.5 -> v0.10.6
* **github.com/siderolabs/crypto**               v0.5.1 -> v0.6.3
* **github.com/siderolabs/talos/pkg/machinery**  v1.10.1 -> da5a4449f1a9
* **google.golang.org/grpc**                     v1.72.0 -> v1.73.0

Previous release can be found at [v0.52.0](https://github.com/siderolabs/omni/releases/tag/v0.52.0)

## [Omni 1.0.0-beta.1](https://github.com/siderolabs/omni/releases/tag/v1.0.0-beta.1) (2025-07-24)

Welcome to the v1.0.0-beta.1 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Multiple Join Token Support

Omni now supports multiple SideroLink join tokens.
It now creates the default non-expiring token, then the user can create more tokens and delete the old ones.


### Config Change History

There is now `MachineConfigDiffs.omni.sidero.dev` resource that keeps the history of
each machine config changes.
It keeps up to 1000 diffs for the last 30 days.


### Contributors

* Artem Chernyshev
* Andrey Smirnov
* Utku Ozdemir
* Oguz Kilcan
* Mateusz Urbanek

### Changes
<details><summary>19 commits</summary>
<p>

* [`28158ed8`](https://github.com/siderolabs/omni/commit/28158ed855513548aa4d78bc7e0cf91e6f6d3dc3) fix: ignore `MachineStatus` having no TalosVersion in DNS service
* [`da3f28f6`](https://github.com/siderolabs/omni/commit/da3f28f6b1f0a01c5a90216fa4b5e7aa3c780ac0) chore: support encoding extra docs in `siderolink.RenderJoinConfig`
* [`f4582116`](https://github.com/siderolabs/omni/commit/f458211621f30c2a07720a4773dbba49d2967ce7) fix: allow encoding join tokens using v1 version
* [`80ff037a`](https://github.com/siderolabs/omni/commit/80ff037a84d5e75c28a4836ac258f8ad9ec9fb36) fix: do not try to encode `v1/v2` tokens in siderolink.NewJoinToken
* [`7b7c021d`](https://github.com/siderolabs/omni/commit/7b7c021da8b911780635d86e19baac78e814516e) fix: do not create `JoinTokenUsage` for `PendingMachines`
* [`2c4f34a7`](https://github.com/siderolabs/omni/commit/2c4f34a7da44c3318da326e116a6bb8f5ccd1e65) fix: fix etcd status check in control plane status
* [`b0f76343`](https://github.com/siderolabs/omni/commit/b0f76343100033927a40ea0e604d5be8a84b3592) feat: implement the API for reading resources and their dependency graph
* [`e945cc7b`](https://github.com/siderolabs/omni/commit/e945cc7b8b342f5f79ecc822edd50a57d69d9210) release(v1.0.0-beta.0): prepare release
* [`a2722856`](https://github.com/siderolabs/omni/commit/a27228563a6a649641e885cd39ef55e88d5402c5) chore: enable gRPC keepalive in the Omni client
* [`f8de9a6d`](https://github.com/siderolabs/omni/commit/f8de9a6d96453bf3e5b9f33668fa35169ada24c0) feat: add support for imported cluster secrets
* [`753259c2`](https://github.com/siderolabs/omni/commit/753259c26edd3e1f61a98a69d334212fa7a9a03b) fix: do not filter out noop events in the infra provider state
* [`ab1f7cc7`](https://github.com/siderolabs/omni/commit/ab1f7cc7fab1d111d561dbf0f2239c169bada5aa) feat: implement multiple token support and token management
* [`0e76483b`](https://github.com/siderolabs/omni/commit/0e76483bab6b9f377bf3e3779f3d02d284a9a782) chore: rekres, bump deps, Go, Talos and k8s versions, satisfy linters
* [`e1c1aaea`](https://github.com/siderolabs/omni/commit/e1c1aaea7a304b2efbd318280afe6afbd18487ab) fix: add validation of k8s version
* [`f1b47f08`](https://github.com/siderolabs/omni/commit/f1b47f08d9a808f5aa635d4ff5b642986305d8a2) feat: log and store redacted machine config diffs
* [`a7ac6372`](https://github.com/siderolabs/omni/commit/a7ac63725d0c8de5cdfc6620607f072a032e383a) chore: rewrite join config generation
* [`66c7897b`](https://github.com/siderolabs/omni/commit/66c7897bb8a657e6d7c391dfa7f35401d9a1d123) chore: update zstd module `go.mod` deps
* [`3b701483`](https://github.com/siderolabs/omni/commit/3b7014839a91ef152b7b82c5b8ae0020ea549e31) test: reduce the log verbosity in unit tests
* [`ff32ae4c`](https://github.com/siderolabs/omni/commit/ff32ae4c7f660a0a1c2e8159e0f9bf52c4b76955) fix: gracefully handle logServer shutdown
</p>
</details>

### Changes since v1.0.0-beta.0
<details><summary>7 commits</summary>
<p>

* [`28158ed8`](https://github.com/siderolabs/omni/commit/28158ed855513548aa4d78bc7e0cf91e6f6d3dc3) fix: ignore `MachineStatus` having no TalosVersion in DNS service
* [`da3f28f6`](https://github.com/siderolabs/omni/commit/da3f28f6b1f0a01c5a90216fa4b5e7aa3c780ac0) chore: support encoding extra docs in `siderolink.RenderJoinConfig`
* [`f4582116`](https://github.com/siderolabs/omni/commit/f458211621f30c2a07720a4773dbba49d2967ce7) fix: allow encoding join tokens using v1 version
* [`80ff037a`](https://github.com/siderolabs/omni/commit/80ff037a84d5e75c28a4836ac258f8ad9ec9fb36) fix: do not try to encode `v1/v2` tokens in siderolink.NewJoinToken
* [`7b7c021d`](https://github.com/siderolabs/omni/commit/7b7c021da8b911780635d86e19baac78e814516e) fix: do not create `JoinTokenUsage` for `PendingMachines`
* [`2c4f34a7`](https://github.com/siderolabs/omni/commit/2c4f34a7da44c3318da326e116a6bb8f5ccd1e65) fix: fix etcd status check in control plane status
* [`b0f76343`](https://github.com/siderolabs/omni/commit/b0f76343100033927a40ea0e604d5be8a84b3592) feat: implement the API for reading resources and their dependency graph
</p>
</details>

### Changes from siderolabs/crypto
<details><summary>5 commits</summary>
<p>

* [`62a079b`](https://github.com/siderolabs/crypto/commit/62a079b6915dc6dff602dca835fafbeabb6adbce) fix: update TLS config, add tests for TLS interactions
* [`c2b4e26`](https://github.com/siderolabs/crypto/commit/c2b4e26d7d7e45e8269a040fb0251446354ba8ef) fix: remove code duplication and fix Ed255119 CA generation
* [`2a07632`](https://github.com/siderolabs/crypto/commit/2a076326bbdd3da61460197a3fa1a0484a347478) fix: enforce FIPS-140-3 compliance
* [`17107ae`](https://github.com/siderolabs/crypto/commit/17107ae45403a2bcd4fecfb4660b60276652b00d) fix: add generic CSR generator and OpenSSL interop
* [`53659fc`](https://github.com/siderolabs/crypto/commit/53659fc35f6abd4ada7ffa22ef1b148cf93c0f28) refactor: split into files
</p>
</details>

### Dependency Changes

* **github.com/cosi-project/runtime**            v0.10.5 -> v0.10.6
* **github.com/siderolabs/crypto**               v0.5.1 -> v0.6.3
* **github.com/siderolabs/talos/pkg/machinery**  v1.10.1 -> da5a4449f1a9
* **google.golang.org/grpc**                     v1.72.0 -> v1.73.0

Previous release can be found at [v0.52.0](https://github.com/siderolabs/omni/releases/tag/v0.52.0)

## [Omni 1.0.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v1.0.0-beta.0) (2025-07-21)

Welcome to the v1.0.0-beta.0 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Multiple Join Token Support

Omni now supports multiple SideroLink join tokens.
It now creates the default non-expiring token, then the user can create more tokens and delete the old ones.


### Config Change History

There is now `MachineConfigDiffs.omni.sidero.dev` resource that keeps the history of
each machine config changes.
It keeps up to 1000 diffs for the last 30 days.


### Contributors

* Andrey Smirnov
* Artem Chernyshev
* Utku Ozdemir
* Oguz Kilcan
* Mateusz Urbanek

### Changes
<details><summary>11 commits</summary>
<p>

* [`a2722856`](https://github.com/siderolabs/omni/commit/a27228563a6a649641e885cd39ef55e88d5402c5) chore: enable gRPC keepalive in the Omni client
* [`f8de9a6d`](https://github.com/siderolabs/omni/commit/f8de9a6d96453bf3e5b9f33668fa35169ada24c0) feat: add support for imported cluster secrets
* [`753259c2`](https://github.com/siderolabs/omni/commit/753259c26edd3e1f61a98a69d334212fa7a9a03b) fix: do not filter out noop events in the infra provider state
* [`ab1f7cc7`](https://github.com/siderolabs/omni/commit/ab1f7cc7fab1d111d561dbf0f2239c169bada5aa) feat: implement multiple token support and token management
* [`0e76483b`](https://github.com/siderolabs/omni/commit/0e76483bab6b9f377bf3e3779f3d02d284a9a782) chore: rekres, bump deps, Go, Talos and k8s versions, satisfy linters
* [`e1c1aaea`](https://github.com/siderolabs/omni/commit/e1c1aaea7a304b2efbd318280afe6afbd18487ab) fix: add validation of k8s version
* [`f1b47f08`](https://github.com/siderolabs/omni/commit/f1b47f08d9a808f5aa635d4ff5b642986305d8a2) feat: log and store redacted machine config diffs
* [`a7ac6372`](https://github.com/siderolabs/omni/commit/a7ac63725d0c8de5cdfc6620607f072a032e383a) chore: rewrite join config generation
* [`66c7897b`](https://github.com/siderolabs/omni/commit/66c7897bb8a657e6d7c391dfa7f35401d9a1d123) chore: update zstd module `go.mod` deps
* [`3b701483`](https://github.com/siderolabs/omni/commit/3b7014839a91ef152b7b82c5b8ae0020ea549e31) test: reduce the log verbosity in unit tests
* [`ff32ae4c`](https://github.com/siderolabs/omni/commit/ff32ae4c7f660a0a1c2e8159e0f9bf52c4b76955) fix: gracefully handle logServer shutdown
</p>
</details>

### Changes from siderolabs/crypto
<details><summary>5 commits</summary>
<p>

* [`62a079b`](https://github.com/siderolabs/crypto/commit/62a079b6915dc6dff602dca835fafbeabb6adbce) fix: update TLS config, add tests for TLS interactions
* [`c2b4e26`](https://github.com/siderolabs/crypto/commit/c2b4e26d7d7e45e8269a040fb0251446354ba8ef) fix: remove code duplication and fix Ed255119 CA generation
* [`2a07632`](https://github.com/siderolabs/crypto/commit/2a076326bbdd3da61460197a3fa1a0484a347478) fix: enforce FIPS-140-3 compliance
* [`17107ae`](https://github.com/siderolabs/crypto/commit/17107ae45403a2bcd4fecfb4660b60276652b00d) fix: add generic CSR generator and OpenSSL interop
* [`53659fc`](https://github.com/siderolabs/crypto/commit/53659fc35f6abd4ada7ffa22ef1b148cf93c0f28) refactor: split into files
</p>
</details>

### Dependency Changes

* **github.com/cosi-project/runtime**            v0.10.5 -> v0.10.6
* **github.com/siderolabs/crypto**               v0.5.1 -> v0.6.3
* **github.com/siderolabs/talos/pkg/machinery**  v1.10.1 -> da5a4449f1a9
* **google.golang.org/grpc**                     v1.72.0 -> v1.73.0

Previous release can be found at [v0.52.0](https://github.com/siderolabs/omni/releases/tag/v0.52.0)

## [Omni 0.52.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.52.0-beta.0) (2025-07-07)

Welcome to the v0.52.0-beta.0 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Support Fusion Auth

Fusion Auth provider is now supported via SAML.
Additional parameter `--auth-saml-name-id-format` must be set to `urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress`.


### Infra Providers Request ID

Infra providers can now be configured to encode `MachineRequest` ID into the join token.
With that enabled setting the machine UUID in the `MachineRequestStatus` is no longer required in the provider:
Omni will automatically map the `MachineRequest` ID to the node UUID and will populate the field in the status.

This change is useful in the infra providers where it's impossible to get the created machine UUID.


### Allow `talosctl wipe disk` Command

`talosctl wipe disk` can now be used with Omni managed nodes.
Omni will impersonate `os:admin` role for it if the user has write access to the cluster.


### Contributors

* Artem Chernyshev
* Utku Ozdemir
* Orzelius

### Changes
<details><summary>14 commits</summary>
<p>

* [`e1d47496`](https://github.com/siderolabs/omni/commit/e1d474960513543b6312631031b91c2789f465f3) feat: allow `talosctl wipe disk` command
* [`877b3791`](https://github.com/siderolabs/omni/commit/877b379100154f9b8455a47746e068ee89298d4c) fix: update SAML library to the forked version with the ACS parser fix
* [`b1225c93`](https://github.com/siderolabs/omni/commit/b1225c9312b4b98e885e3e90c92e3ff544c3274f) feat: support setting custom name ID format in SAML metadata
* [`c60820f0`](https://github.com/siderolabs/omni/commit/c60820f05ed29aaaa3746b5316f52c11cbcd2899) fix: correctly detect installation status for bare-metal machines
* [`344d0618`](https://github.com/siderolabs/omni/commit/344d0618dd44473967efbeaf96b4c637addb3964) feat: allow encoding the machine request ID into the join tokens
* [`a7fe525c`](https://github.com/siderolabs/omni/commit/a7fe525ce1d0d99e2e6241f8ba7d66e0d27eeee5) test: test updating from old Omni version to the current
* [`abfe93c0`](https://github.com/siderolabs/omni/commit/abfe93c02cb5e8330ba04851f63b715e87f9d9f1) docs: add guide for development on darwin
* [`0ad0a67b`](https://github.com/siderolabs/omni/commit/0ad0a67b041b8ddab5ae43ef175e2a70d540d375) test: save a support bundle when a test suite has failed
* [`9e4f8198`](https://github.com/siderolabs/omni/commit/9e4f8198cdd06aa016ace155072b2283511cdcf8) fix: make sure clipped req/resp content logs are still valid JSONs
* [`1c307300`](https://github.com/siderolabs/omni/commit/1c3073001731267530b1a17d2084a09d3b24a657) feat: show the actual node name in the node overview breadcrumbs
* [`c097b5f1`](https://github.com/siderolabs/omni/commit/c097b5f14dfc809671c273a60c2b3e2b06b5d293) fix: do not try running debug server in the prod builds
* [`8a93c2d5`](https://github.com/siderolabs/omni/commit/8a93c2d5baef913cc991c90ffa0b158467c5dcd6) refactor: bring back the reverted new workload proxy dialing logic
* [`122b7960`](https://github.com/siderolabs/omni/commit/122b79605fec8b9b86950c094c37d8b41a5b72cf) test: run Omni as part of integration tests
</p>
</details>

### Dependency Changes

This release has no dependency changes

Previous release can be found at [v0.51.0](https://github.com/siderolabs/omni/releases/tag/v0.51.0)

## [Omni 0.51.0-beta.2](https://github.com/siderolabs/omni/releases/tag/v0.51.0-beta.2) (2025-06-17)

Welcome to the v0.51.0-beta.2 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### 

Omni can now be configured using `YAML` configuration file in addition to the command line flags.
The parameter `--config-file` can be used for that. Any command line flags have higher priority.


### Contributors

* Artem Chernyshev

### Changes
<details><summary>2 commits</summary>
<p>

* [`493d00ca`](https://github.com/siderolabs/omni/commit/493d00ca54aaff425182de491b58c78f5faa40c2) fix: properly support `--config-path` argument
* [`742faec7`](https://github.com/siderolabs/omni/commit/742faec7001b6d5c04a68966b42c4da52665d94d) fix: do not mark SAML and Auth0 config sections as mutually exclusive
</p>
</details>

### Dependency Changes

This release has no dependency changes

Previous release can be found at [v0.51.0-beta.1](https://github.com/siderolabs/omni/releases/tag/v0.51.0-beta.1)


## [Omni 0.51.0-beta.1](https://github.com/siderolabs/omni/releases/tag/v0.51.0-beta.1) (2025-06-14)

Welcome to the v0.51.0-beta.1 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Contributors

* Utku Ozdemir

### Changes
<details><summary>2 commits</summary>
<p>

* [`4b40dc1`](https://github.com/siderolabs/omni/commit/4b40dc1dcf20c94479b5838fe66d52cb5206160e) fix: do more fixes on config backward-compatibility
* [`3085c3f`](https://github.com/siderolabs/omni/commit/3085c3f73b97ea0cb8e61084ef623913530e0a19) fix: remove `required` config validation from k8s proxy cert and key
</p>
</details>

### Dependency Changes

This release has no dependency changes

Previous release can be found at [v0.51.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.51.0-beta.0)

## [Omni 0.51.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.51.0-beta.0) (2025-06-13)

Welcome to the v0.51.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Contributors

* Utku Ozdemir
* Artem Chernyshev
* Andrew Rynhard
* Andrey Smirnov

### Changes
<details><summary>17 commits</summary>
<p>

* [`7a815ba`](https://github.com/siderolabs/omni/commit/7a815ba1143b4ec6441dfc0e10042183c7fed3c3) fix: prevent zero machine count reports on controller shutdown
* [`33e796e`](https://github.com/siderolabs/omni/commit/33e796e2a4edc9bcc898f5abab30acfb1bdc8713) chore: rekres, bump Go to 1.24.4, SideroLink to v0.3.15
* [`f3cec18`](https://github.com/siderolabs/omni/commit/f3cec18a297003a00bbe632e469df2c335fb9b65) fix: fix exposed service prefix conflict resolution
* [`5e4c10b`](https://github.com/siderolabs/omni/commit/5e4c10b32137caef3806d5559d8201f387d115af) fix: use `0.0.0.0:8095` as the default bind endpoint for Kubernetes
* [`ccd55cc`](https://github.com/siderolabs/omni/commit/ccd55cc8fb5fddaab91ffc817649ca05fa82702b) feat: rewrite Omni config management
* [`05aad4d`](https://github.com/siderolabs/omni/commit/05aad4d86fbf897fbed90ae4b845228dd496cbde) fix: check config patch creation time as well as updated on orphan check
* [`f3783ed`](https://github.com/siderolabs/omni/commit/f3783edcb0a875f321554e8e7aad6fcd4559c496) fix: display unknown power state correctly on the machines screen
* [`7c19c31`](https://github.com/siderolabs/omni/commit/7c19c318e810937959464f4d279d58dbbf672c6e) test: improve workload proxying tests
* [`c9c4c8e`](https://github.com/siderolabs/omni/commit/c9c4c8e10db05e177d139ff07e5b98fac96581bc) test: use `go test` to build and run Omni integration tests
* [`df5a2b9`](https://github.com/siderolabs/omni/commit/df5a2b92f98f8671993669a34ed12493cc871884) fix: bump inmem COSI state history capacity
* [`aa5d89d`](https://github.com/siderolabs/omni/commit/aa5d89d6d41be2b1c97de442b73d9736948f876a) fix: fix panic in maintenance upgrade
* [`404bbd9`](https://github.com/siderolabs/omni/commit/404bbd9357f06a58718e1fc0b0c3a78842f6fe85) chore: allow running Omni programmatically from other Go code
* [`9846622`](https://github.com/siderolabs/omni/commit/98466220bf9a4320fc012e627763593c1c230b3b) fix: fix nil dereference in machine status controller
* [`13bb8b5`](https://github.com/siderolabs/omni/commit/13bb8b54060f02517ed7164ff71e3328d52c2937) docs: update SECURITY.md
* [`178d2ad`](https://github.com/siderolabs/omni/commit/178d2add3a0a77bf88414bd8a38d94226677137d) fix: make sure `powering on` stage is correctly set on infra machines
* [`e5d1b4b`](https://github.com/siderolabs/omni/commit/e5d1b4b0837c3cae9a1ae2347fdc9cf71a35f4fe) fix: properly detect infra provider service accounts
* [`d88bb1d`](https://github.com/siderolabs/omni/commit/d88bb1df064a2069cb89fe7f1e67d518d6199d09) test: use latest Talemu infra provider version in the integration tests
</p>
</details>

### Changes from siderolabs/gen
<details><summary>3 commits</summary>
<p>

* [`dcb2b74`](https://github.com/siderolabs/gen/commit/dcb2b7417879f230a569ce834dad5c89bd09d6bf) feat: add `panicsafe` package
* [`b36ee43`](https://github.com/siderolabs/gen/commit/b36ee43f667a7a56b340a3e769868ff2a609bb5b) feat: make `xyaml.CheckUnknownKeys` public
* [`3e319e7`](https://github.com/siderolabs/gen/commit/3e319e7e52c5a74d1730be8e47952b3d16d91148) feat: implement `xyaml.UnmarshalStrict`
</p>
</details>

### Changes from siderolabs/siderolink
<details><summary>2 commits</summary>
<p>

* [`5f46f65`](https://github.com/siderolabs/siderolink/commit/5f46f6583b9d03f91c9bb5f637149fe466d17bfc) feat: handle panics in goroutines
* [`d09ff45`](https://github.com/siderolabs/siderolink/commit/d09ff45b450a37aa84652fa70b5cd3467ee8243d) fix: race in wait value
</p>
</details>

### Dependency Changes

* **github.com/go-logr/logr**                 v1.4.2 **_new_**
* **github.com/go-playground/validator/v10**  v10.26.0 **_new_**
* **github.com/siderolabs/gen**               v0.8.1 -> v0.8.4
* **github.com/siderolabs/siderolink**        v0.3.14 -> v0.3.15
* **golang.org/x/crypto**                     v0.38.0 -> v0.39.0
* **golang.org/x/net**                        v0.40.0 -> v0.41.0
* **golang.org/x/sync**                       v0.14.0 -> v0.15.0
* **golang.org/x/text**                       v0.25.0 -> v0.26.0

Previous release can be found at [v0.50.0](https://github.com/siderolabs/omni/releases/tag/v0.50.0)

## [Omni 0.50.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.50.0-beta.0) (2025-05-15)

Welcome to the v0.50.0-beta.0 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Contributors

* Noel Georgi
* Artem Chernyshev
* Andrey Smirnov
* Utku Ozdemir
* Aleksandr Gamzin
* Dmitriy Matrenichev
* kalio007

### Changes
<details><summary>10 commits</summary>
<p>

* [`09f44685`](https://github.com/siderolabs/omni/commit/09f446858535c9c4e3d91d1e8a82f9f9bdbc7f4e) fix: pin AWS s3 libs version to 1.72.3
* [`dc753f4e`](https://github.com/siderolabs/omni/commit/dc753f4e756b8a02002bc8077cb6a3f6278988dd) test: bump Talos version used in integration tests to `v1.10`
* [`f21cedc7`](https://github.com/siderolabs/omni/commit/f21cedc7e70dcf86d98b9c1ef5b2a7be61ed8501) chore: introduce COSI state helpers to reduce boiler plate code count
* [`9fcea4ea`](https://github.com/siderolabs/omni/commit/9fcea4eab3ab29f1f27403c441f30e03b1e071ac) test: add unit test for `nextAvailableClusterName` function
* [`c9b62c23`](https://github.com/siderolabs/omni/commit/c9b62c23cc8c618c1245f3828c3bd67f6f48ed03) fix: update go-kubernetes library to the latest version
* [`daaec8df`](https://github.com/siderolabs/omni/commit/daaec8dfa35d3b45088b82d8bb268bd5a00a7008) fix: remove deprecated controller finalizers from the machine classes
* [`eaeff1ea`](https://github.com/siderolabs/omni/commit/eaeff1ea3fd616d7dc5844f3dfd7d7f902d1589a) fix: keep ClusterUUID resource alive until the cluster is destroyed
* [`aa24c7c7`](https://github.com/siderolabs/omni/commit/aa24c7c707506d8bc2919482b140ebf43776631e) fix: fix crash in the SAML ACS handler
* [`ccd5e7e4`](https://github.com/siderolabs/omni/commit/ccd5e7e44f3385531798502518439fb12cc950f4) chore: bump Go deps
* [`47b6fb7c`](https://github.com/siderolabs/omni/commit/47b6fb7cc89af6d71a36c54f4a64e0beaaebed8d) feat(ci): support releasing helm charts
</p>
</details>

### Changes from siderolabs/gen
<details><summary>1 commit</summary>
<p>

* [`7c0324f`](https://github.com/siderolabs/gen/commit/7c0324fee9a7cfbdd117f43702fa273689f0db97) chore: future-proof HashTrieMap
</p>
</details>

### Changes from siderolabs/go-circular
<details><summary>1 commit</summary>
<p>

* [`5b39ef8`](https://github.com/siderolabs/go-circular/commit/5b39ef87df04efeaa47fe6374a8114f39c126122) fix: do not log error if chunk zero was never written
</p>
</details>

### Changes from siderolabs/go-kubernetes
<details><summary>2 commits</summary>
<p>

* [`9070be4`](https://github.com/siderolabs/go-kubernetes/commit/9070be4308e23d969ec4fc49b25dab4a27d512e7) fix: remove DynamicResourceAllocation feature gate
* [`8cb588b`](https://github.com/siderolabs/go-kubernetes/commit/8cb588bc4c93d812de901a6a33e599ba2169cd96) fix: k8s 1.32->1.33 upgrade check
</p>
</details>

### Changes from siderolabs/image-factory
<details><summary>8 commits</summary>
<p>

* [`f930246`](https://github.com/siderolabs/image-factory/commit/f930246105e2e69df53bb38bfc581ce99efc1090) release(v0.7.0): prepare release
* [`5b85f95`](https://github.com/siderolabs/image-factory/commit/5b85f95cb46746fb9e7050fe95f74ba19ffba506) chore: bump deps
* [`cdfab7d`](https://github.com/siderolabs/image-factory/commit/cdfab7ded77a7114cf04e3a292ee63f0c6ef35ee) chore(ci): add an cron ci for talos main integration test
* [`69525ba`](https://github.com/siderolabs/image-factory/commit/69525bae922888cb53f6bdf2f0e8900573c974d7) release(v0.6.9): prepare release
* [`2820cb0`](https://github.com/siderolabs/image-factory/commit/2820cb013326d3f34a309a1dc6a7b6f2d64c1afa) feat(i18n): frontend localization support
* [`f1187bc`](https://github.com/siderolabs/image-factory/commit/f1187bc84911f12fd421056a4ae1a6d8b190da5e) chore: bump deps
* [`ba8640b`](https://github.com/siderolabs/image-factory/commit/ba8640be86296e546540c30ba047b917a783f1b2) chore: bump deps
* [`b8308aa`](https://github.com/siderolabs/image-factory/commit/b8308aa592c9740917145ca8e861e9494b05aa47) chore: bump talos machinery
</p>
</details>

### Changes from siderolabs/siderolink
<details><summary>1 commit</summary>
<p>

* [`d2a79e0`](https://github.com/siderolabs/siderolink/commit/d2a79e0263806b68ff0a44ea9efa58b83fb269ec) fix: clean up device on failure
</p>
</details>

### Dependency Changes

* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.17.72 -> v1.17.49
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.79.2 -> v1.72.3
* **github.com/cosi-project/runtime**                  v0.10.2 -> v0.10.5
* **github.com/cosi-project/state-etcd**               v0.5.1 -> v0.5.2
* **github.com/siderolabs/gen**                        v0.8.0 -> v0.8.1
* **github.com/siderolabs/go-circular**                v0.2.2 -> v0.2.3
* **github.com/siderolabs/go-kubernetes**              v0.2.21 -> v0.2.23
* **github.com/siderolabs/image-factory**              v0.6.8 -> v0.7.0
* **github.com/siderolabs/omni/client**                v0.48.3 -> v0.49.0
* **github.com/siderolabs/siderolink**                 v0.3.13 -> v0.3.14
* **github.com/siderolabs/talos/pkg/machinery**        v1.10.0 -> v1.10.1
* **github.com/zitadel/oidc/v3**                       v3.37.0 -> v3.38.1
* **golang.org/x/crypto**                              v0.37.0 -> v0.38.0
* **golang.org/x/net**                                 v0.39.0 -> v0.40.0
* **golang.org/x/sync**                                v0.13.0 -> v0.14.0
* **golang.org/x/text**                                v0.24.0 -> v0.25.0
* **golang.org/x/tools**                               v0.32.0 -> v0.33.0
* **golang.zx2c4.com/wireguard**                       12269c276173 -> 436f7fdc1670

Previous release can be found at [v0.49.0](https://github.com/siderolabs/omni/releases/tag/v0.49.0)

## [Omni 0.49.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.49.0-beta.0) (2025-05-05)

Welcome to the v0.49.0-beta.0 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Async Discovery Service Cleanup

The machine teardown now no longer blocks on the discovery service being unavailable.
If failed, discovery service removal is now handled async.


### Control Plane Force Delete

Omni now allows forcefully removing the control plane nodes from the cluster, where etcd is not healthy.


### Contributors

* David Anderson
* Utku Ozdemir
* Artem Chernyshev
* Brad Fitzpatrick
* Noel Georgi
* Andrey Smirnov
* AdamEr8
* Andrey Smirnov
* Andrey Smirnov
* Dominic Evans
* Khionu Sybiern
* Nathan Johnson
* Ryan Cox
* Vincent Batts
* ignoramous

### Changes
<details><summary>31 commits</summary>
<p>

* [`84623a52`](https://github.com/siderolabs/omni/commit/84623a52853006ecf087e240cd03cb5c86047974) release(v0.49.0-beta.0): prepare release
* [`68292cc9`](https://github.com/siderolabs/omni/commit/68292cc985ffc9d60aaece45813c0d9edd0caf29) chore: update JS deps, drop `package-lock.json`
* [`d3bbc2f4`](https://github.com/siderolabs/omni/commit/d3bbc2f407224e14e8d8540ead2fa1e4a5fdbeaf) fix: gracefully render exposed services errors
* [`c329668a`](https://github.com/siderolabs/omni/commit/c329668a83fceed0994933afe569225075610a65) fix: correctly encode exposed service redirect URL after auth
* [`7acf2d90`](https://github.com/siderolabs/omni/commit/7acf2d90fa7093a0eb1c27e2e03383d9947c1ee0) feat: update machinery and specs to Talos 1.10.0
* [`ccf4bfb1`](https://github.com/siderolabs/omni/commit/ccf4bfb1ef6c9d0587b5bc010920a84630eaa6f3) fix: use the correct sort order for the automatic install disk selection
* [`34c96f21`](https://github.com/siderolabs/omni/commit/34c96f21e0cca7a548ae0f9b70476edae62a3c48) fix: collect and handle UKI boot information
* [`ff032337`](https://github.com/siderolabs/omni/commit/ff0323373e6fc076f4fc91b68907775a817e7a7d) fix: remove machine set allocation source option
* [`e7ece828`](https://github.com/siderolabs/omni/commit/e7ece8280dc38668d3099eeb62f06ac9554c34fe) fix: disable Talos >= 1.10 for now as Omni isn't ready for it yet
* [`2606693d`](https://github.com/siderolabs/omni/commit/2606693d25f9756468f52790b4d820501570a922) fix: remove "Generated by Omni..." comment from machine config
* [`574a0b05`](https://github.com/siderolabs/omni/commit/574a0b05b66fddc54ec3924733c4734b9454b132) fix: sort Talos versions by semver on the cluster creation screen
* [`fbb80f0b`](https://github.com/siderolabs/omni/commit/fbb80f0b514a195c6294c5070f64a354ff22a2f9) feat: implement async delete from discovery service(s)
* [`1722b4bf`](https://github.com/siderolabs/omni/commit/1722b4bf6e0b1902da0888a31b15059d5bd8ea94) fix: loosen s3 integrity check for etcd backups
* [`1dce4acb`](https://github.com/siderolabs/omni/commit/1dce4acbc6ad845f598d08504de0a9e0b01e6b17) feat: allow force-destroying a node in booting state
* [`3897080a`](https://github.com/siderolabs/omni/commit/3897080a48765cf93c5b399461c41bc06aa36b40) test: add config encoding stability tests
* [`0fc7a16f`](https://github.com/siderolabs/omni/commit/0fc7a16f04e26b6d95ceb296e80fb9ceb3edde59) test: fix the flaky key storage test
* [`71cef7a4`](https://github.com/siderolabs/omni/commit/71cef7a419fc51ce096e34fd7c0ee1bf8f56b3f2) fix: do not add omni api host to kube-apiserver cert SANs
* [`5057ba92`](https://github.com/siderolabs/omni/commit/5057ba92cb2c32f307847faeb80d2a739c839263) chore: rekres, bump deps, satisfy linters, fix generated test headers
* [`9a81546d`](https://github.com/siderolabs/omni/commit/9a81546d21ac6df8229d4a24531d80e77ac1d074) fix: return proper errors for the SideroLink provision API
* [`970dafc2`](https://github.com/siderolabs/omni/commit/970dafc2b03808595a4716fee9343610fd2bcf11) fix: correctly sort versions on the download installation media page
* [`e407b0ab`](https://github.com/siderolabs/omni/commit/e407b0ab432485a43c56078222aeccddfd31125c) fix: move JSON schema forms validation to backend
* [`d96b2bc6`](https://github.com/siderolabs/omni/commit/d96b2bc6d2337a2364e4cc4ddc8e8fb82d4c0e0d) feat: improve logging/debugging of exposed services
* [`21213d8c`](https://github.com/siderolabs/omni/commit/21213d8ce5a2e95262448a588953ea498bf6440d) fix: properly skip the contract config patch removal migration
* [`09a7d482`](https://github.com/siderolabs/omni/commit/09a7d482346a357cebc950b7ec856f5ca24a5a54) fix: add annotations on the `ClusterMachines` to force enable features
* [`282fba43`](https://github.com/siderolabs/omni/commit/282fba439c5cb1efb9d4536b4c633cc39165e126) fix: use correct version contract for machine config generation
* [`3f3f8a98`](https://github.com/siderolabs/omni/commit/3f3f8a985d7e371e82805df98e9b435e613154d4) fix: create config patches to prevent reboot on version contract revert
* [`17129e51`](https://github.com/siderolabs/omni/commit/17129e5184bdbc4a847c61a65cd97b7ee84a6e56) fix: config patch cleanup
* [`d9b5dae3`](https://github.com/siderolabs/omni/commit/d9b5dae3fedfb3df247718559adf2fa3a2fb825c) fix: fix existing alias check for exposed services
* [`09c80dd8`](https://github.com/siderolabs/omni/commit/09c80dd8e0ca80902d44ac156bd542cfdbd5cd7b) fix: mark all exposed services to have a non-explicit alias
* [`3e07a88a`](https://github.com/siderolabs/omni/commit/3e07a88a5db91a39abbc16d9422ed8d7fc4e0bd0) fix: revert workload proxy LB refactoring
* [`b32f5556`](https://github.com/siderolabs/omni/commit/b32f555605c048234715ee82a2b50d47ce6640df) fix: use proper background for the sticky window in the code editor
</p>
</details>

### Changes from siderolabs/go-kubernetes
<details><summary>1 commit</summary>
<p>

* [`2bdbda7`](https://github.com/siderolabs/go-kubernetes/commit/2bdbda70062e7f371f270a430a6e53605259c36d) feat: adjust checks for Kubernetes v1.33.0
</p>
</details>

### Changes from siderolabs/image-factory
<details><summary>6 commits</summary>
<p>

* [`3e56929`](https://github.com/siderolabs/image-factory/commit/3e56929cdf3fbb30a56d3095d7c32d8668c323a3) release(v0.6.8): prepare release
* [`1af002d`](https://github.com/siderolabs/image-factory/commit/1af002d206295d42608d16637a1f1937e3c77cb7) feat: support platform specific installers
* [`e1d19df`](https://github.com/siderolabs/image-factory/commit/e1d19dfbff074ccbec9751ed3c9a3eff8914bf47) chore: add more tests for talos 1.10
* [`0ecde68`](https://github.com/siderolabs/image-factory/commit/0ecde6846e70bfc7a5c4b86e7b63cb2209976f26) fix(ci): image push
* [`2460d03`](https://github.com/siderolabs/image-factory/commit/2460d03f2071b0e2e5357f7acdd1e31258b0e571) fix(ci): image push
* [`a016223`](https://github.com/siderolabs/image-factory/commit/a016223e8cdd3be5a4c1be26c5c03ac3f9fa1eda) feat: pull in new Talos machinery
</p>
</details>

### Changes from siderolabs/tcpproxy
<details><summary>70 commits</summary>
<p>

* [`3d4e7b8`](https://github.com/siderolabs/tcpproxy/commit/3d4e7b860749152f0aefc53594f4c5fb9285c3f3) chore: rename to siderolabs/tcpproxy
* [`6f85d8e`](https://github.com/siderolabs/tcpproxy/commit/6f85d8e975e316d2e825db5c349c33eb8eb627d2) Implement correct half-close sequence for the connections.
* [`8bea9a4`](https://github.com/siderolabs/tcpproxy/commit/8bea9a449198dd0d0184ae0a6770d556dea5e0a0) Add support for TCP_USER_TIMEOUT setting
* [`91f8614`](https://github.com/siderolabs/tcpproxy/commit/91f861402626c6ba93eaa57ee257109c4f07bd00) remove old ACME tls-sni-01 stuff that LetsEncrypt removed March 2019
* [`74ca1dc`](https://github.com/siderolabs/tcpproxy/commit/74ca1dc5d55168d202044c415dcf2e08d80c3fdc) add Proxy.AddSNIRouteFunc to do lookups by SNI dynamically
* [`4e04b92`](https://github.com/siderolabs/tcpproxy/commit/4e04b92f29ea8f8a10d28528a47ecc0f93814473) gofmt for Go 1.19
* [`be3ee21`](https://github.com/siderolabs/tcpproxy/commit/be3ee21c9fa0283869843039aa136fbf9b948bf0) (doc): s/tlsproxy/tcpproxy
* [`2e577fe`](https://github.com/siderolabs/tcpproxy/commit/2e577fef49e2458ca3da06b30409df8f4eacb21e) Modified TestProxyPROXYOut to conform with the fixed version of PROXY protocol header format
* [`0f9bced`](https://github.com/siderolabs/tcpproxy/commit/0f9bceda1a83b4a17e52ba327a6fb2561285ee1a) Fixed HAProxy's PROXY protocol v1 Human-readable header format in DialProxy
* [`2825d76`](https://github.com/siderolabs/tcpproxy/commit/2825d768aaaef27e854631354415484406b1bc92) fix(test): update travis and e2e selfSignedCert fn
* [`b6bb9b5`](https://github.com/siderolabs/tcpproxy/commit/b6bb9b5b82524122bcf27291ede32d1517a14ab8) Update import path to inet.af/tcpproxy
* [`dfa16c6`](https://github.com/siderolabs/tcpproxy/commit/dfa16c61dad2b18a385dfb351adf71566720535b) tlsrouter/README: fix the go get url
* [`f5c09fb`](https://github.com/siderolabs/tcpproxy/commit/f5c09fbedceb69e4b238dec52cdf9f2fe9a815e2) Take advantage of Go 1.11's splice support, unwrap Conns in DialProxy.HandleConn
* [`7f81f77`](https://github.com/siderolabs/tcpproxy/commit/7f81f7701c9b584822030be9a3a701b125a56c91) Work around deadlock with Go tip (at Go rev f3f7bd5)
* [`7efa37f`](https://github.com/siderolabs/tcpproxy/commit/7efa37ff5079eba4a39ddda1b79f65fc81c759e3) Quiet log spam in test.
* [`dbc1514`](https://github.com/siderolabs/tcpproxy/commit/dbc151467a20b4513174bb3d6b1283e9419eb0f9) Adding the HostName field to the Conn struct (#18)
* [`2b928d9`](https://github.com/siderolabs/tcpproxy/commit/2b928d9b07d782cc1a94736979d012792810658f) Link to docs
* [`de1c7de`](https://github.com/siderolabs/tcpproxy/commit/de1c7ded2e6918c5b5b932682e0de144f4f1a31d) Add support for arbitrary matching against HTTP and SNI hostnames.
* [`c6a0996`](https://github.com/siderolabs/tcpproxy/commit/c6a0996ce0f3db7b5c3e16e04c9e664936077c97) Support configurable routing of ACME tls-sni-01 challenges.
* [`815c942`](https://github.com/siderolabs/tcpproxy/commit/815c9425f1ad46ffd3a3fb1bbefc05440072e4a4) Merge matcher and route into an interface that yields a Target.
* [`2065af4`](https://github.com/siderolabs/tcpproxy/commit/2065af4b1e2d181a987a23f64c66f43e474469ff) Support HAProxy's PROXY protocol v1 in DialProxy.
* [`e030359`](https://github.com/siderolabs/tcpproxy/commit/e03035937341374a9be6eb8459ffe4f23bacd185) Fix golint nits by adding docstrings and simplifying execution flow.
* [`6d97c2a`](https://github.com/siderolabs/tcpproxy/commit/6d97c2aa8ea9d9f5a35614d1f4a2a7d6be28ae9a) Correct the package building command, and only deploy for master branch commits.
* [`aa12504`](https://github.com/siderolabs/tcpproxy/commit/aa12504e4e35953c3281989f871e1293eb2114fe) Another attempt to fix Travis.
* [`f6af481`](https://github.com/siderolabs/tcpproxy/commit/f6af481b22698c9c27dd2f6af1881ea995c72046) Make Travis test all packages, and remove the go.universe.tf import path.
* [`d7e343e`](https://github.com/siderolabs/tcpproxy/commit/d7e343ee3d714651cbf09f4d77e56ad24f75eb33) Fix the godoc link to point to google/tcpproxy.
* [`bef9f6a`](https://github.com/siderolabs/tcpproxy/commit/bef9f6aa62487d4adc7d8ddf9e29b9f28810316f) Merge bradfitz's tcpproxy codebase with the software formerly known as tlsrouter.
* [`d86e96a`](https://github.com/siderolabs/tcpproxy/commit/d86e96a9d54bb62b297cf30dd2242b365fe33604) Move tlsrouter's readme to the command's directory.
* [`9e73877`](https://github.com/siderolabs/tcpproxy/commit/9e73877b6b356885077a1b9c0ba349ce33c61438) Switch license to Apache2, add Google copyright headers.
* [`cbf137d`](https://github.com/siderolabs/tcpproxy/commit/cbf137dac7b2c4aa2f45572c1214d07b30742241) Correct the travis build to kinda work.
* [`3eb49e9`](https://github.com/siderolabs/tcpproxy/commit/3eb49e9b3902de95b3c9f5729d51ca7f61f02e5a) Move tlsrouter to cmd/tlsrouter, in preparation for rewrite as a pkg.
* [`af97cdd`](https://github.com/siderolabs/tcpproxy/commit/af97cdd9d95a6cae6a52775ab8d5b3fc456a6817) Fix copy/paste-o in doc example.
* [`3273f40`](https://github.com/siderolabs/tcpproxy/commit/3273f401801fb301dffe0380ae573ee34a4f5c36) Add vendor warning
* [`e387889`](https://github.com/siderolabs/tcpproxy/commit/e3878897bde4f5d532f67738009cf1b9fcd2f408) Add TargetListener
* [`2eb0155`](https://github.com/siderolabs/tcpproxy/commit/2eb0155fac2d41b022bc0a430d13aa3b45825f1d) Start of tcpproxy. No Listener or reverse dialing yet.
* [`c58b44c`](https://github.com/siderolabs/tcpproxy/commit/c58b44c4fc69a3602d751d679c69c07e6bcbe24a) Make golint fail if lint errors are found, and fix said lint.
* [`4621df9`](https://github.com/siderolabs/tcpproxy/commit/4621df99bdd73dbb3995055b9b4f3f062300c892) Clean up the Travis build a bit more, moving more stuff to the deploy stage.
* [`96cc76f`](https://github.com/siderolabs/tcpproxy/commit/96cc76fdcd91148162fc3211dbfd486a86c1cb0f) Test Travis's new build stage support.
* [`bbbede8`](https://github.com/siderolabs/tcpproxy/commit/bbbede8f604a6555c951f5d584ddf4e98f26191a) Make travis fetch the test-only dependency.
* [`4b8641f`](https://github.com/siderolabs/tcpproxy/commit/4b8641f40e04705b8227f245be36457c05ccba2c) Add support for HAProxy's PROXY protocol.
* [`d23eadc`](https://github.com/siderolabs/tcpproxy/commit/d23eadc3a6c89bf5058db893acee26d5f1d7e350) Upload packages based on Go 1.8, not 1.7.
* [`7ef32e3`](https://github.com/siderolabs/tcpproxy/commit/7ef32e3c68ff50a2002528af7ff7676fb58be0a2) Add Go 1.8 to the build matrix.
* [`e07ecec`](https://github.com/siderolabs/tcpproxy/commit/e07ececb94dd7fe786db042337ad2dc0d5a448a6) typo
* [`aa3f9c9`](https://github.com/siderolabs/tcpproxy/commit/aa3f9c9ba10dc5b2d1b79d5de05ae6bf83483334) Remove debug print in acme code.
* [`6664640`](https://github.com/siderolabs/tcpproxy/commit/666464088dba67b6748beea064ae830f3e699d37) Stop testing against Go 1.6.
* [`728b8bc`](https://github.com/siderolabs/tcpproxy/commit/728b8bce14d8241b090ecf89c7f48224d5ba2c74) Add ACME routing support.
* [`a5c2ccd`](https://github.com/siderolabs/tcpproxy/commit/a5c2ccd532db7f26e6f6caff9570f126b9f58713) Use nogroup as the group, not nobody.
* [`a94dbd1`](https://github.com/siderolabs/tcpproxy/commit/a94dbd1d9e69346cbc08462da0b799f4d7d1d51f) Port extra error checking over from netboot.
* [`3cd4412`](https://github.com/siderolabs/tcpproxy/commit/3cd44123fb97589bbb7aa8b0743c124a8ce81c9b) Clean up travis config a bit, and add missing copyright notice.
* [`aded796`](https://github.com/siderolabs/tcpproxy/commit/aded79682ca01ac8c7fb17449d79f5274e727f2d) Add a deploy step to garbage-collect old packagecloud files.
* [`3e6354c`](https://github.com/siderolabs/tcpproxy/commit/3e6354c147b050cb9b008ae44d68fd1d3d385723) Random change to force travis rebuild on latest code.
* [`77fa998`](https://github.com/siderolabs/tcpproxy/commit/77fa9980b9f34a5dd58909748a7bf53d10333bec) Attempt to create a package with no version name.
* [`bfef4ba`](https://github.com/siderolabs/tcpproxy/commit/bfef4ba5a22a178fb4a64f0fe9d98fcfd53edee0) Revert to just debian/jessie. It's the same package anyway.
* [`173db90`](https://github.com/siderolabs/tcpproxy/commit/173db9074b8e6586588af6d63e4a0dabe8f48a73) Try the obvious way to specify a matrix of package tags.
* [`ea58780`](https://github.com/siderolabs/tcpproxy/commit/ea5878082eb53bfe1c26e76671e912079590e058) Limit the deploy to only the go 1.7 build.
* [`a2d0c96`](https://github.com/siderolabs/tcpproxy/commit/a2d0c96158d3810655fb71ead9187f1268541e3f) Skip cleanup so travis doesn't delete the freshly built .deb.
* [`73ee2e7`](https://github.com/siderolabs/tcpproxy/commit/73ee2e798a4464ed94b947b5a6b6a8425b37f99e) Attempt a packagecloud push.
* [`cbd4ea6`](https://github.com/siderolabs/tcpproxy/commit/cbd4ea6ea39c80d520d75e3e1cb140b55d6220fc) Attempt to build a debian package with FPM.
* [`4f5b46f`](https://github.com/siderolabs/tcpproxy/commit/4f5b46f61cba8359944015dfbcbce4b88cc0fd00) Add a systemd unit file to run tlsrouter.
* [`8cc8cac`](https://github.com/siderolabs/tcpproxy/commit/8cc8cac141994b55ac7f2b98ad363b2196d867f4) Document -hello-timeout in README.
* [`e0a0158`](https://github.com/siderolabs/tcpproxy/commit/e0a01587f5d3c412231f18012f3f55743c5aa885) Add slowloris protection, in the form of a ClientHello timeout.
* [`09cc4bb`](https://github.com/siderolabs/tcpproxy/commit/09cc4bb6199e7c8ef49d4c3f5e4077b49f892407) Remove support for SSL 3.0.
* [`c41a68d`](https://github.com/siderolabs/tcpproxy/commit/c41a68d73b757355dbd1f433fc4e2afe161c1f7b) Add tests for hostname matching, and make DNS matches match entire string.
* [`6546db4`](https://github.com/siderolabs/tcpproxy/commit/6546db44e46c75d1ec05fbd47f1396c49705c34d) Fix vet errors in Go 1.6.
* [`e34c2a6`](https://github.com/siderolabs/tcpproxy/commit/e34c2a61afa52bf8cc245c1ff75cca10b231050e) Add more words to README.
* [`b321571`](https://github.com/siderolabs/tcpproxy/commit/b321571464ebd231043ead1e15f23dba1c02970c) Add godoc comments to appease golint.
* [`55ba69d`](https://github.com/siderolabs/tcpproxy/commit/55ba69dad29c3f6b3aec89789fc8a773cd776b28) Add a Travis CI config.
* [`b8a3ed8`](https://github.com/siderolabs/tcpproxy/commit/b8a3ed89ade6a84297914e83559ff8cb1b7c5d33) Add DNS name support to config
* [`0a0a9f6`](https://github.com/siderolabs/tcpproxy/commit/0a0a9f658b3a5aabf24cc9c78f2ff0baef7d5622) Add licensing and contributing information for release.
* [`b1edd90`](https://github.com/siderolabs/tcpproxy/commit/b1edd90c0436159dcf4d3f794121633fb8ed9035) Initial commit.
</p>
</details>

### Dependency Changes

* **github.com/aws/aws-sdk-go-v2/config**              v1.29.9 -> v1.29.14
* **github.com/aws/aws-sdk-go-v2/credentials**         v1.17.62 -> v1.17.67
* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.17.66 -> v1.17.72
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.78.2 -> v1.79.2
* **github.com/containers/image/v5**                   v5.34.2 -> v5.35.0
* **github.com/cosi-project/runtime**                  v0.10.1 -> v0.10.2
* **github.com/crewjam/saml**                          v0.4.14 -> v0.5.1
* **github.com/fsnotify/fsnotify**                     v1.8.0 -> v1.9.0
* **github.com/go-jose/go-jose/v4**                    v4.0.5 -> v4.1.0
* **github.com/golang-jwt/jwt/v4**                     v4.5.1 -> v4.5.2
* **github.com/jonboulle/clockwork**                   v0.5.0 **_new_**
* **github.com/prometheus/client_golang**              v1.21.1 -> v1.22.0
* **github.com/siderolabs/go-kubernetes**              v0.2.20 -> v0.2.21
* **github.com/siderolabs/image-factory**              v0.6.7 -> v0.6.8
* **github.com/siderolabs/omni/client**                v0.47.1 -> v0.48.3
* **github.com/siderolabs/talos/pkg/machinery**        v1.10.0-alpha.2 -> v1.10.0
* **github.com/siderolabs/tcpproxy**                   v0.1.0 **_new_**
* **github.com/zitadel/logging**                       v0.6.1 -> v0.6.2
* **github.com/zitadel/oidc/v3**                       v3.36.1 -> v3.37.0
* **go.etcd.io/etcd/client/pkg/v3**                    v3.5.19 -> v3.5.21
* **go.etcd.io/etcd/client/v3**                        v3.5.19 -> v3.5.21
* **go.etcd.io/etcd/server/v3**                        v3.5.19 -> v3.5.21
* **golang.org/x/crypto**                              v0.36.0 -> v0.37.0
* **golang.org/x/net**                                 v0.37.0 -> v0.39.0
* **golang.org/x/sync**                                v0.12.0 -> v0.13.0
* **golang.org/x/text**                                v0.24.0 **_new_**
* **golang.org/x/tools**                               v0.31.0 -> v0.32.0
* **google.golang.org/grpc**                           v1.71.0 -> v1.72.0
* **google.golang.org/protobuf**                       v1.36.5 -> v1.36.6
* **k8s.io/api**                                       v0.32.3 -> v0.34.0-alpha.0
* **k8s.io/apimachinery**                              v0.32.3 -> v0.34.0-alpha.0
* **k8s.io/client-go**                                 v0.32.3 -> v0.34.0-alpha.0
* **sigs.k8s.io/controller-runtime**                   v0.20.3 -> v0.20.4

Previous release can be found at [v0.48.0](https://github.com/siderolabs/omni/releases/tag/v0.48.0)

## [Omni 0.48.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.48.0-beta.0) (2025-04-03)

Welcome to the v0.48.0-beta.0 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Custom Etcd Backup Throughput

The throughput for etcd backup uploads/downloads can now be limited using the `--etcd-backup-upload-limit` and `--etcd-backup-download-limit` flags.


### Explicit Exposed Service Prefixes

Users can now explicitly specify the prefix for exposed services using the `omni-kube-service-exposer.sidero.dev/prefix` annotation on their Kubernetes Services.

This is useful when users prefer not to have prefixes randomly generated.


### Filter Clusters by Readiness

Clusters can now be filtered by readiness status in the Omni UI.


### Cleanup of Unused Config Patches

User-managed config patches not associated with an existing target (e.g., cluster, machine set, or machine) are now cleaned up after 30 days.


### Contributors

* Artem Chernyshev
* Utku Ozdemir
* Dmitriy Matrenichev
* Andrey Smirnov
* Orzelius
* Luke Milby
* Matt Willsher
* Nick Niehoff
* Noel Georgi

### Changes
<details><summary>52 commits</summary>
<p>

* [`5c4e983`](https://github.com/siderolabs/omni/commit/5c4e983718e1dd27e074991b5032d83ea0853351) fix: restore timeout in `OmniSuite.SetupTest`
* [`72405c7`](https://github.com/siderolabs/omni/commit/72405c7e9fc7d39e6938dc84b84169a44dd679fe) fix: filter out device mapper/lvm disks from block devices
* [`a91bb04`](https://github.com/siderolabs/omni/commit/a91bb0416d6495c5927d93f55d8e0e2fc08ce391) feat: use `<platform>-installer[-secureboot]` as the installer image
* [`77ab722`](https://github.com/siderolabs/omni/commit/77ab7222940b9ab57ba5ae0911f6aadfc283f6ed) chore: rekres, bump Go, regenerate, fix docker-compose targets
* [`9338a1a`](https://github.com/siderolabs/omni/commit/9338a1aa5f14f73f08706fda0c73072ec36133fb) fix: check proper jittered time in unit tests
* [`6978d31`](https://github.com/siderolabs/omni/commit/6978d31aee70d050147d344eb2c0a18c40c86c72) chore: add support for jitter in `EtcdBackupController`
* [`143e7a6`](https://github.com/siderolabs/omni/commit/143e7a69e1b4b03300fb05b35d8621af6212d715) feat: support filtering clusters by readiness
* [`d93ae59`](https://github.com/siderolabs/omni/commit/d93ae5960322e1f298b7a2be7fde607235af65bf) fix: ignore 404 errors when removing disconnected links of a cluster
* [`9abf37c`](https://github.com/siderolabs/omni/commit/9abf37c1b46407f9ee291372a9a490d2dbf7d5b6) fix: use clearer description on the machines page metrics
* [`2a20840`](https://github.com/siderolabs/omni/commit/2a2084018c78e8815457f5dd69d4f984631d80df) fix: correctly parse commas in label selectors
* [`764cec4`](https://github.com/siderolabs/omni/commit/764cec4dc1b0f3582c327950f0acc915e46a2f54) fix: show warning instead of error for etcd quorum being at min
* [`4f00856`](https://github.com/siderolabs/omni/commit/4f008567a0af5ccdc155216b167c8c8b48836079) chore: update dependencies
* [`b6563c2`](https://github.com/siderolabs/omni/commit/b6563c2d217ec530281bcee3371b5e12096528c9) chore: bump default Talos version to 1.9.5, Kubernetes version to 1.32.3
* [`5ef843f`](https://github.com/siderolabs/omni/commit/5ef843f81741d48ee8b07b26d675caa34eeb0fe7) fix: properly display error message when machine class removal failed
* [`b91b673`](https://github.com/siderolabs/omni/commit/b91b673a00b3457b32d2001d8010ced61598a5d1) fix: add more strict security headers to the web page handler
* [`57c005e`](https://github.com/siderolabs/omni/commit/57c005e5d0daeb6e2672f3be73edc434b2e3055c) feat: allow setting exposed service prefixes explicitly
* [`3c55a0b`](https://github.com/siderolabs/omni/commit/3c55a0b0bf05497ee6de9a0adc426ecb4202e99e) fix: do not allow `http[s]` urls in the redirect query
* [`0cd8212`](https://github.com/siderolabs/omni/commit/0cd8212f594724a2e9bb0895b8ddddb286dbf80c) fix: do not select USB sticks by default
* [`3650c60`](https://github.com/siderolabs/omni/commit/3650c60c4ec369290d4e5990a0d92f2be8be6970) fix: duplicate resources declaration in helm deployment
* [`7c50e8b`](https://github.com/siderolabs/omni/commit/7c50e8ba7b39c4c4ad70ebd7f5647eb4fd6ad674) fix: update text and description for SideroLink over GRPC
* [`4dea372`](https://github.com/siderolabs/omni/commit/4dea3725493ca3216841d83d24413c1d0eb81388) chore: add GOEXPERIMENT env to vscode config
* [`e6e9202`](https://github.com/siderolabs/omni/commit/e6e9202c61361115e90644957d32eeaf51da7482) test: fix the timing related flake in resource logger test
* [`3b0e831`](https://github.com/siderolabs/omni/commit/3b0e831dff7694f66062737cf26169540bc35621) fix: do not switch Siderolink GRPC tunnel mode after provisioning
* [`4a8546e`](https://github.com/siderolabs/omni/commit/4a8546e0dcf97ef7984ec38e23fc112f6665b22e) fix: some updated icons were appearing as white
* [`1fb14d2`](https://github.com/siderolabs/omni/commit/1fb14d2b5aabce01d713ea91d5f66cd100ceaaab) fix: do not clip the tooltip in the cluster machine status
* [`63a3c52`](https://github.com/siderolabs/omni/commit/63a3c525ddc48372c135cd691a7bf79439ec3317) chore: update all used icons
* [`1e721e5`](https://github.com/siderolabs/omni/commit/1e721e57c8976275a7a65b4041e27fac616bdb1d) feat: cleanup orphan config patches
* [`f7da5d0`](https://github.com/siderolabs/omni/commit/f7da5d058ef0e7b17bc2bef2928743998d7d0a78) chore: rework `EtcdBackupControllerSuite` to use synctest experiment
* [`a5efd81`](https://github.com/siderolabs/omni/commit/a5efd816a239e6c9e5ea7c0d43c02c04504d7b60) feat: validate incoming packets addresses in siderolink manager
* [`966b99c`](https://github.com/siderolabs/omni/commit/966b99c45c0debbc30fc52f485658a379ddbebe5) chore: rekres to enable separate cache
* [`b1c71f0`](https://github.com/siderolabs/omni/commit/b1c71f02f3b92772117abb87800fdfe3e2b80913) feat: add support for custom throughput for uploads and downloads
* [`86976d3`](https://github.com/siderolabs/omni/commit/86976d353352d6364fcc8dc034b6239d67a06058) perf: move etcd backup status resources into secondary storage
* [`1e67803`](https://github.com/siderolabs/omni/commit/1e67803fa900b9fcb5eb0bd954bf65fb9df66b57) fix: remove force unique token annotation from the link on wipe
* [`9012978`](https://github.com/siderolabs/omni/commit/90129782ad753779c414640ada17ea70e6043adf) chore: replace `InfoIterator` with `iter.Seq2` type
* [`b519c6c`](https://github.com/siderolabs/omni/commit/b519c6c571588f8d1b8ec078918244547174b06c) chore: migrate ConfigPatches above threshold of 2048
* [`b264a41`](https://github.com/siderolabs/omni/commit/b264a412c22ec877199d49388d44401057117478) fix: properly support the PXE and ISO machines in the secure tokens flow
* [`fd2d340`](https://github.com/siderolabs/omni/commit/fd2d340b094db9694e47f6ed4feed3fa54d820a2) fix: exclude `metal-agent` extension from available extensions
* [`c6e5a5f`](https://github.com/siderolabs/omni/commit/c6e5a5fe17980df6be4f9efac7906c9e897d8bc7) chore: enable compression only for `ConfigPatch`
* [`bd264cd`](https://github.com/siderolabs/omni/commit/bd264cd9f53f1329cf4859995d5d6c786a171f61) chore: expose `omni_runtime_cached_resources` metric
* [`e751022`](https://github.com/siderolabs/omni/commit/e751022e8a2cd0d2c240a46d798a2a109d35fd8d) chore: rework `Reconciler` to use proper `http.Transport`
* [`2bb38e3`](https://github.com/siderolabs/omni/commit/2bb38e3876a5c2c80ece1abeaed1774c50ad64db) chore: add `omni_machine_config_patch_size` metrics
* [`075698d`](https://github.com/siderolabs/omni/commit/075698df9d4a91d479e51b3d00d3b52b23a76be1) fix: preserve SideroLink tunnel config on machine allocation
* [`56fbf31`](https://github.com/siderolabs/omni/commit/56fbf3129b24456f1ebac927229d30c545deb0e9) fix: skip applying maintenance config to unsupported machines
* [`bfd24e5`](https://github.com/siderolabs/omni/commit/bfd24e5d0b553e8ec7033eb125c1d69b9d4ef16b) fix: disable `compressConfigsAndMachinePatches`
* [`82d1f09`](https://github.com/siderolabs/omni/commit/82d1f095d4bdc536edcb567d00223c1f7f05d8f4) fix: fix exposed service links on the sidebar
* [`9e7d8fb`](https://github.com/siderolabs/omni/commit/9e7d8fbe9227aab7075c5604fadf97e1109d3f2e) fix: increase log level of the SideroLink GRPC tunnel handler
* [`ad34182`](https://github.com/siderolabs/omni/commit/ad341821e8b9c612b106be298f51abfea1ccbb3c) fix: properly build the search query on the Machines page
* [`517c294`](https://github.com/siderolabs/omni/commit/517c2942eddb9b3daeb493417c9273d190707990) chore: add logging for migrations
* [`aef8b43`](https://github.com/siderolabs/omni/commit/aef8b43cebe23aa261dd9ff0115d4004062ae55d) fix: extensions list hidden on small screens
* [`57cea88`](https://github.com/siderolabs/omni/commit/57cea88f4bc0dc06157c6a579a934b946261edc1) chore: warn if cluster doesn't exist in omnictl talosconfig command
* [`ef32e43`](https://github.com/siderolabs/omni/commit/ef32e434acc09cc0dc582c70afbe970dc047d5ed) fix: increase log level of the SideroLink GRPC tunnel handler
* [`510512e`](https://github.com/siderolabs/omni/commit/510512e7b211acdee671ddb54a1234d4981e448e) fix: properly read the `siderolink-disable-last-endpoint` flag
</p>
</details>

### Changes from siderolabs/discovery-api
<details><summary>1 commit</summary>
<p>

* [`64513a6`](https://github.com/siderolabs/discovery-api/commit/64513a6c4fb31c6a043159d5caea1d153ea133a4) feat: rekres, regenerate proto files
</p>
</details>

### Changes from siderolabs/discovery-client
<details><summary>1 commit</summary>
<p>

* [`b3632c4`](https://github.com/siderolabs/discovery-client/commit/b3632c4a8cd96ae36337e83308ef447361b51537) feat: support extra dial options in the client
</p>
</details>

### Changes from siderolabs/discovery-service
<details><summary>3 commits</summary>
<p>

* [`008fcae`](https://github.com/siderolabs/discovery-service/commit/008fcae3c63000f7f9b94767206e816feb80f1e4) release(v1.0.10): prepare release
* [`6a44f8c`](https://github.com/siderolabs/discovery-service/commit/6a44f8c89b3bd127978b7ab17f17b1bff2d9f5dd) chore: bump dependencies
* [`761d53a`](https://github.com/siderolabs/discovery-service/commit/761d53a418d75438529293da808491774a5104e2) feat: update dependencies
</p>
</details>

### Changes from siderolabs/go-kubernetes
<details><summary>3 commits</summary>
<p>

* [`9ba5654`](https://github.com/siderolabs/go-kubernetes/commit/9ba5654fcec6061322530394e336b68a8c764a1b) fix: fix ignoring alpha/beta version parsing
* [`0fe1db4`](https://github.com/siderolabs/go-kubernetes/commit/0fe1db4603b591883fac9ce4afcab911bc57922c) feat: update for new changes in Kubernetes 1.33.0-alpha.3
* [`804cb44`](https://github.com/siderolabs/go-kubernetes/commit/804cb440c2299488c7c68185c53b91ffdfb8bf32) feat: add support for Kubernetes to 1.33
</p>
</details>

### Changes from siderolabs/go-loadbalancer
<details><summary>1 commit</summary>
<p>

* [`589c33a`](https://github.com/siderolabs/go-loadbalancer/commit/589c33a96ac74a8c0e36b09f534fca62afd6de81) chore: upgrade `upstream.List` and `loadbalancer.TCP` to Go 1.23
</p>
</details>

### Changes from siderolabs/go-pointer
<details><summary>1 commit</summary>
<p>

* [`347ee9b`](https://github.com/siderolabs/go-pointer/commit/347ee9b78f625d420254f4ab01bb1d6174474bf4) chore: rekres, update dependencies
</p>
</details>

### Changes from siderolabs/image-factory
<details><summary>8 commits</summary>
<p>

* [`c6e3fa6`](https://github.com/siderolabs/image-factory/commit/c6e3fa604ac403dfcf6b61a3b762e2ab8fe2505c) release(v0.6.7): prepare release
* [`f896663`](https://github.com/siderolabs/image-factory/commit/f896663ab4586afee44e605e8e9982dfa99dfd08) feat: update Talos to v1.10.0-alpha.1
* [`0931477`](https://github.com/siderolabs/image-factory/commit/09314778e2ac52c9fb61f6e443ee1579dc26268d) release(v0.6.6): prepare release
* [`b80192a`](https://github.com/siderolabs/image-factory/commit/b80192aca0adabdc5f49414854d60c0bb6f778af) feat: refactor platform metadata
* [`4bb43ef`](https://github.com/siderolabs/image-factory/commit/4bb43ef97afddaf324933061a0cb653a00afd669) fix: add imgfree to ipxe boot script
* [`d5f3f5a`](https://github.com/siderolabs/image-factory/commit/d5f3f5a1a6b6ec33fc761890fb7ff446ef6f70db) feat: update for Talos 1.10 current
* [`e727003`](https://github.com/siderolabs/image-factory/commit/e72700352b09df37e651e8579ae4191837070c7a) chore: update go-uefi module
* [`3b302c6`](https://github.com/siderolabs/image-factory/commit/3b302c6a4ca1e2e104b23ae02e1a60e968927731) feat: set secure boot support for nocloud platform
</p>
</details>

### Changes from siderolabs/siderolink
<details><summary>1 commit</summary>
<p>

* [`a7af143`](https://github.com/siderolabs/siderolink/commit/a7af1431e0798541f8d3db0aa70af0e15b2c3eb6) feat: support packets filtering before writing them to the tun device
</p>
</details>

### Dependency Changes

* **github.com/ProtonMail/gopenpgp/v2**                v2.8.2 -> v2.8.3
* **github.com/auth0/go-jwt-middleware/v2**            v2.2.2 -> v2.3.0
* **github.com/aws/aws-sdk-go-v2**                     v1.32.8 -> v1.36.3
* **github.com/aws/aws-sdk-go-v2/config**              v1.28.11 -> v1.29.9
* **github.com/aws/aws-sdk-go-v2/credentials**         v1.17.52 -> v1.17.62
* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.17.49 -> v1.17.66
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.72.3 -> v1.78.2
* **github.com/aws/smithy-go**                         v1.22.1 -> v1.22.3
* **github.com/cenkalti/backoff/v5**                   v5.0.2 **_new_**
* **github.com/containers/image/v5**                   v5.33.0 -> v5.34.2
* **github.com/cosi-project/runtime**                  v0.9.4 -> v0.10.1
* **github.com/emicklei/dot**                          v1.6.4 -> v1.8.0
* **github.com/go-jose/go-jose/v4**                    v4.0.4 -> v4.0.5
* **github.com/google/go-cmp**                         v0.6.0 -> v0.7.0
* **github.com/google/go-containerregistry**           v0.20.2 -> v0.20.3
* **github.com/grpc-ecosystem/grpc-gateway/v2**        v2.26.0 -> v2.26.3
* **github.com/hashicorp/vault/api**                   v1.15.0 -> v1.16.0
* **github.com/hashicorp/vault/api/auth/kubernetes**   v0.8.0 -> v0.9.0
* **github.com/jellydator/ttlcache/v3**                v3.3.0 **_new_**
* **github.com/johannesboyne/gofakes3**                0da3aa9c32ca -> 5c39aecd6999
* **github.com/klauspost/compress**                    v1.18.0 **_new_**
* **github.com/prometheus/client_golang**              v1.20.5 -> v1.21.1
* **github.com/prometheus/common**                     v0.62.0 -> v0.63.0
* **github.com/santhosh-tekuri/jsonschema/v6**         v6.0.1 **_new_**
* **github.com/siderolabs/discovery-api**              v0.1.5 -> v0.1.6
* **github.com/siderolabs/discovery-client**           v0.1.10 -> v0.1.11
* **github.com/siderolabs/discovery-service**          v1.0.9 -> v1.0.10
* **github.com/siderolabs/go-kubernetes**              v0.2.17 -> v0.2.20
* **github.com/siderolabs/go-loadbalancer**            v0.3.4 -> v0.4.0
* **github.com/siderolabs/go-pointer**                 v1.0.0 -> v1.0.1
* **github.com/siderolabs/image-factory**              v0.6.5 -> v0.6.7
* **github.com/siderolabs/omni/client**                v0.45.0 -> v0.47.1
* **github.com/siderolabs/siderolink**                 v0.3.12 -> v0.3.13
* **github.com/siderolabs/talos/pkg/machinery**        v1.10.0-alpha.0 -> v1.10.0-alpha.2
* **github.com/spf13/cobra**                           v1.8.1 -> v1.9.1
* **github.com/stripe/stripe-go/v76**                  v76.25.0 **_new_**
* **github.com/zitadel/oidc/v3**                       v3.34.0 -> v3.36.1
* **go.etcd.io/bbolt**                                 v1.3.11 -> v1.4.0
* **go.etcd.io/etcd/client/pkg/v3**                    v3.5.18 -> v3.5.19
* **go.etcd.io/etcd/client/v3**                        v3.5.18 -> v3.5.19
* **go.etcd.io/etcd/server/v3**                        v3.5.18 -> v3.5.19
* **go.uber.org/mock**                                 v0.5.0 **_new_**
* **golang.org/x/crypto**                              v0.33.0 -> v0.36.0
* **golang.org/x/net**                                 v0.35.0 -> v0.37.0
* **golang.org/x/sync**                                v0.11.0 -> v0.12.0
* **golang.org/x/time**                                v0.11.0 **_new_**
* **golang.org/x/tools**                               v0.29.0 -> v0.31.0
* **google.golang.org/grpc**                           v1.70.0 -> v1.71.0
* **google.golang.org/protobuf**                       v1.36.4 -> v1.36.5
* **k8s.io/api**                                       v0.32.0 -> v0.32.3
* **k8s.io/client-go**                                 v0.32.0 -> v0.32.3
* **sigs.k8s.io/controller-runtime**                   v0.19.4 -> v0.20.3

Previous release can be found at [v0.47.0](https://github.com/siderolabs/omni/releases/tag/v0.47.0)


## [Omni 0.47.0-beta.1](https://github.com/siderolabs/omni/releases/tag/v0.47.0-beta.1) (2025-02-21)

Welcome to the v0.47.0-beta.1 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Contributors

* Artem Chernyshev

### Changes
<details><summary>1 commit</summary>
<p>

* [`d25146a`](https://github.com/siderolabs/omni/commit/d25146a031255475e0379dc4ca301ba916c2c854) fix: fix config compression migration
</p>
</details>

### Dependency Changes

This release has no dependency changes

Previous release can be found at [v0.47.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.47.0-beta.0)


## [Omni 0.47.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.47.0-beta.0) (2025-02-21)

Welcome to the v0.47.0-beta.0 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Manual BMC Configs

BMC configs now can be set manually in the bare-metal infra provider.


### Machine Categories

Machine categories were now moved to the sidebar, which gives a way to filter them by each particular provider.


### Maintenance Configs

Omni now injects KmsgLog and EventSink configs for each joined node.
So the node will work even if only the siderolink config pushed to it.


### Service Accounts UI

Service accounts creation UI now presents the environment variables (endpoint and key) instead of the key only.


### Contributors

* Artem Chernyshev
* Dmitriy Matrenichev
* Utku Ozdemir
* Andrey Smirnov
* Andrew Rynhard
* Ethan Norlander

### Changes
<details><summary>25 commits</summary>
<p>

* [`f7b2cdf`](https://github.com/siderolabs/omni/commit/f7b2cdf0a9928172b987bca1c042ab8ec304f825) test: improve e2e upgrades and e2e templates tests stability
* [`6a807c1`](https://github.com/siderolabs/omni/commit/6a807c12efe0e37c1275a298273f1fb64b0885ca) feat: push a partial config to machines in maintenance mode
* [`1d291c4`](https://github.com/siderolabs/omni/commit/1d291c4e14cd69c88e5507e784e47b4f6f53be7a) fix: adjust the UI layout and get rid of a couple of bugs
* [`5fe3223`](https://github.com/siderolabs/omni/commit/5fe3223999e1700ef053b7d145a67ca03266a870) feat: add a flag to enable secure join token flow
* [`c662c2e`](https://github.com/siderolabs/omni/commit/c662c2e0305dddd0cafe56a782a88b887bc59001) feat: implement the machine categories UI for the sidebar
* [`2cb37d8`](https://github.com/siderolabs/omni/commit/2cb37d8dc0a6861dd21f8c66733c67e1032a8eb6) chore: add `compressConfigsAndMachinePatches` migration back
* [`2108697`](https://github.com/siderolabs/omni/commit/210869725d7f7730fc156d2dfcc76857261d723f) docs: how to download generic talosconfig in omnictl talosconfig help
* [`7e32dcc`](https://github.com/siderolabs/omni/commit/7e32dcc2a6aed90559be03a0f872e9e0ad39efb4) fix: detect more block devices
* [`2e9828a`](https://github.com/siderolabs/omni/commit/2e9828a3fdc5a043c5ec5db6e6246f2365a2d7f2) fix: properly handle duplicate UUID
* [`ff107e5`](https://github.com/siderolabs/omni/commit/ff107e549c9cd96e35996bb225d9783fdc870de9) fix: add workaround to stage upgrades for talos `v1.9.0-v1.9.2`
* [`9bb85f8`](https://github.com/siderolabs/omni/commit/9bb85f80344b47d82cb0f1458fa4257711ffeefb) feat: implement secure node join flow
* [`0cda77b`](https://github.com/siderolabs/omni/commit/0cda77bbce5b5b5a3781bbd189abee00ea314771) chore: bump Go and rekres
* [`0f7563f`](https://github.com/siderolabs/omni/commit/0f7563faa201e4b5941885326c5a994687cd2b67) fix: make the apply config fail if machine has wrong state
* [`6fb1fcd`](https://github.com/siderolabs/omni/commit/6fb1fcd5dfd537c75d41174fe78eac1244e053b2) feat: allow manual bmc configuration for bare metal machines
* [`b654b2c`](https://github.com/siderolabs/omni/commit/b654b2c287d7824578f889dd423cde61e5d013ac) chore: remove unused field in `s3store.Store`
* [`2dc4dae`](https://github.com/siderolabs/omni/commit/2dc4dae4a8894c93ea9a13a3f0654db2a63772fd) chore: omni enable config compression by default
* [`214eece`](https://github.com/siderolabs/omni/commit/214eece7c5d5403276079f0afa040a679d683655) chore: bump deps
* [`651d98e`](https://github.com/siderolabs/omni/commit/651d98ea23daf35f78f3d3a520f5b5eed3575efb) fix: enable IDP initiated SAML logins
* [`951c0de`](https://github.com/siderolabs/omni/commit/951c0de2bb2fed62e414c989007b4d5ac3c322c5) fix: don't forget to close grpc connections in tests
* [`157ceac`](https://github.com/siderolabs/omni/commit/157ceac7f86804c8ca2fba2926a715802a3bbfa9) fix: close grpc connections after their usage is complete
* [`6f014b1`](https://github.com/siderolabs/omni/commit/6f014b1ea164dd0872619422c9db9af3b8141681) fix: fix node resolution cache for nodes in maintenance mode
* [`65244f6`](https://github.com/siderolabs/omni/commit/65244f67c7d8f30b7a146a48ab5514b39fd49d07) test: run the integration tests only for pull requests
* [`ed946b3`](https://github.com/siderolabs/omni/commit/ed946b30a66b8d5cb939e149177981f6e00c6d7a) feat: display OMNI_ENDPOINT in the service account creation UI
* [`7ae5af7`](https://github.com/siderolabs/omni/commit/7ae5af7744c5533f5116270208bea67559e82220) fix: do not compress resources with phase != running
* [`4485296`](https://github.com/siderolabs/omni/commit/4485296e31a100a67b07b5b1c7a9e018b532d1e2) feat: add Stripe machine reporting
</p>
</details>

### Changes from siderolabs/go-circular
<details><summary>2 commits</summary>
<p>

* [`015a398`](https://github.com/siderolabs/go-circular/commit/015a398e79f2853714cd20d1135dc100f18b6c29) fix: replace static buffer allocation on growth
* [`ed8685e`](https://github.com/siderolabs/go-circular/commit/ed8685e0cf9491d9a714e565e0e736439a94a73f) test: add more assertions for write length result
</p>
</details>

### Changes from siderolabs/go-debug
<details><summary>1 commit</summary>
<p>

* [`ea108ca`](https://github.com/siderolabs/go-debug/commit/ea108cacca8940426149e67ba00e414633e4ef3f) chore: add support for Go 1.24
</p>
</details>

### Changes from siderolabs/proto-codec
<details><summary>1 commit</summary>
<p>

* [`3235c29`](https://github.com/siderolabs/proto-codec/commit/3235c2984fa1bb3cd8d38c088127c46dd3d2860e) chore: bump deps
</p>
</details>

### Changes from siderolabs/siderolink
<details><summary>1 commit</summary>
<p>

* [`38e459e`](https://github.com/siderolabs/siderolink/commit/38e459e50c467791c9670a60ef41f58db246715a) chore: bump deps
</p>
</details>

### Dependency Changes

* **github.com/cenkalti/backoff/v4**             v4.3.0 **_new_**
* **github.com/cosi-project/runtime**            v0.9.2 -> v0.9.4
* **github.com/cosi-project/state-etcd**         v0.5.0 -> v0.5.1
* **github.com/grpc-ecosystem/grpc-gateway/v2**  v2.25.1 -> v2.26.0
* **github.com/prometheus/common**               v0.61.0 -> v0.62.0
* **github.com/siderolabs/go-circular**          v0.2.1 -> v0.2.2
* **github.com/siderolabs/go-debug**             v0.4.0 -> v0.5.0
* **github.com/siderolabs/proto-codec**          v0.1.1 -> v0.1.2
* **github.com/siderolabs/siderolink**           v0.3.11 -> v0.3.12
* **github.com/stripe/stripe-go/v74**            v74.30.0 **_new_**
* **go.etcd.io/etcd/client/pkg/v3**              v3.5.17 -> v3.5.18
* **go.etcd.io/etcd/client/v3**                  v3.5.17 -> v3.5.18
* **go.etcd.io/etcd/server/v3**                  v3.5.17 -> v3.5.18
* **golang.org/x/crypto**                        v0.32.0 -> v0.33.0
* **golang.org/x/net**                           v0.34.0 -> v0.35.0
* **golang.org/x/sync**                          v0.10.0 -> v0.11.0
* **google.golang.org/grpc**                     v1.69.4 -> v1.70.0
* **google.golang.org/protobuf**                 v1.36.3 -> v1.36.4

Previous release can be found at [v0.46.0](https://github.com/siderolabs/omni/releases/tag/v0.46.0)

## [Omni 0.46.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.46.0-beta.0) (2025-01-17)

Welcome to the v0.46.0-beta.0 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Bare Metal Infra Provider Support

Omni now supports [bare metal infra provider](https://github.com/siderolabs/omni-infra-provider-bare-metal/).

This provider operates as a standalone service that can be deployed within a bare-metal datacenter network.
It manages machines via IPMI, supports PXE-based booting, and enables machine resets without relying on the Talos API.

Its functionality closely resembles that of Sidero Metal.

For detailed setup instructions, refer to the [documentation](https://omni.siderolabs.com/tutorials/setting-up-the-bare-metal-infrastructure-provider)..


### Machine Categories

The Machines page now categorizes machines based on how they were added to the account:

- Manual: Machines manually added by installing Talos with siderolink parameters.
- Provisioned: Machines created by infrastructure providers (e.g., KubeVirt).
- PXE-Booted: Machines discovered and accepted from the bare-metal infrastructure provider.
- Pending: Machines discovered but not yet accepted from the bare-metal infrastructure provider.


### Contributors

* Utku Ozdemir
* Artem Chernyshev
* Andrey Smirnov
* Dmitriy Matrenichev
* Noel Georgi
* Joakim Nohlgård
* Nico Berlee

### Changes
<details><summary>28 commits</summary>
<p>

* [`8701623`](https://github.com/siderolabs/omni/commit/8701623750dff4640a317aa67a3ff86da8e952fe) release(v0.46.0-beta.0): prepare release
* [`afc9dcf`](https://github.com/siderolabs/omni/commit/afc9dcffd6a356edde7cbdbdc74c09caaf6c766b) feat: introduce resource for infra provider health checks
* [`096f14f`](https://github.com/siderolabs/omni/commit/096f14f9b92cc163fb20f1eb036ad7954f9f559b) feat: calculate not accepted machines count in the machine status ctrl
* [`1495ca0`](https://github.com/siderolabs/omni/commit/1495ca007f302f397f8e4b8391e5e5e2e4e9afaa) feat: implement power states as machine stage events
* [`2a2c648`](https://github.com/siderolabs/omni/commit/2a2c6481414b683f0979a384b3a626feda003e54) feat: bump default Talos version to 1.9.1, Kubernetes to 1.32.0
* [`5db4c8c`](https://github.com/siderolabs/omni/commit/5db4c8c62fa70c1e2750e305eb7e0af921e6da0f) feat: add disk wipe warning when accepting a pending machine
* [`9208587`](https://github.com/siderolabs/omni/commit/920858754e16bfcd5f05f8424331422a5008c7ed) chore: bump Go, dependencies, rekres, regenerate
* [`84c01fd`](https://github.com/siderolabs/omni/commit/84c01fde3e14bb9b5bd875c0dbbf5743f9e93cdb) fix: properly reset `MachineStatus` hostname for deallocated machines
* [`d5e1f85`](https://github.com/siderolabs/omni/commit/d5e1f854dbb0c2463dcb8b79b86959b79128394d) fix: do not allow using static infra providers in the machine classes
* [`d1b3dff`](https://github.com/siderolabs/omni/commit/d1b3dffd4ec394a9c7f672d219de56ff6cf66aa3) fix: fix immutability checks in infra provider state
* [`353a3c0`](https://github.com/siderolabs/omni/commit/353a3c04a25454b6d2b5451a0d19d0d943083682) fix: change the look of the infra provider labels
* [`7052e8b`](https://github.com/siderolabs/omni/commit/7052e8b644a521929d8d399b56552c32b056b29e) fix: enable secure boot checkbox in the UI
* [`394065f`](https://github.com/siderolabs/omni/commit/394065f7f487c5362cd335ee55d8c59001af39ac) feat: implement cordoning infra machines
* [`728897a`](https://github.com/siderolabs/omni/commit/728897aba629b23fd5ed2f110f63931a170dad17) fix: wait for infra machine info to be collected before powering off
* [`1c4f9af`](https://github.com/siderolabs/omni/commit/1c4f9afa079ee214e6dfd12a196180d5b5d14899) feat: implement infra machine reboot
* [`edc47a0`](https://github.com/siderolabs/omni/commit/edc47a0ec02eddf3bfb03d3b0d58648833545f1b) feat: sync infra machine labels onto the machine status
* [`6f10a97`](https://github.com/siderolabs/omni/commit/6f10a975f3c407ca2eff9e338de6a55fc31f63d1) fix: fallback to machine ID correctly if its hostname is missing
* [`3ba096a`](https://github.com/siderolabs/omni/commit/3ba096a06df4d4978ab3d3f748bb5b977e36cf98) fix: bring in new versions of COSI runtime and state-etcd
* [`82da2f4`](https://github.com/siderolabs/omni/commit/82da2f4894ef9ddfb01ab68ad43bb4e01f651dc1) fix: never remove etcd members which ID is discovered at least once
* [`3dd7e93`](https://github.com/siderolabs/omni/commit/3dd7e939729bd857837598885009203b00796bf2) fix: display nodes sidebar
* [`06aa266`](https://github.com/siderolabs/omni/commit/06aa2664b274c42f3b05a779a5b38937512eee8f) fix: etcd lease handlining
* [`34dd2ae`](https://github.com/siderolabs/omni/commit/34dd2ae070eec946c2d5128890e9bac52946c386) feat: properly handle powered off machines in the UI and machine classes
* [`1d8c754`](https://github.com/siderolabs/omni/commit/1d8c754abb98fcb5d697608c300fdc2f32a25915) fix: do not preserve extensions on Talos agent mode
* [`1f81400`](https://github.com/siderolabs/omni/commit/1f814006903b22415c736f636051e119989a8464) fix: run destroy validations on teardown
* [`6190568`](https://github.com/siderolabs/omni/commit/6190568b4700669dd86f2a3e7a128df974623398) fix: allow accepting rejected infra machines
* [`3332684`](https://github.com/siderolabs/omni/commit/3332684ec25389a48105c05e0c1d731fcfbdac2c) fix: correctly handle input finalizers in `InfraMachineController`
* [`b7c3c50`](https://github.com/siderolabs/omni/commit/b7c3c5025df97ad6891965777dd42165323ec54d) feat: add support for Zitadel IdP
* [`e8aee8e`](https://github.com/siderolabs/omni/commit/e8aee8ed86c0eaf3f8a3a79a6d02bc531ce22589) feat: implement the machine categories UI
</p>
</details>

### Changes from siderolabs/discovery-service
<details><summary>1 commit</summary>
<p>

* [`7c1129e`](https://github.com/siderolabs/discovery-service/commit/7c1129e3e77a3e19e00386a4e00f8bfae5043abe) chore: bump deps
</p>
</details>

### Changes from siderolabs/gen
<details><summary>1 commit</summary>
<p>

* [`5ae3afe`](https://github.com/siderolabs/gen/commit/5ae3afee65490ca9f4bd32ea41803ab3a17cad7e) chore: update hashtriemap implementation from the latest upstream
</p>
</details>

### Changes from siderolabs/go-talos-support
<details><summary>1 commit</summary>
<p>

* [`0f784bd`](https://github.com/siderolabs/go-talos-support/commit/0f784bd58b320543663679693c817515067f3021) fix: avoid deadlock on context cancel
</p>
</details>

### Changes from siderolabs/image-factory
<details><summary>7 commits</summary>
<p>

* [`a4932a2`](https://github.com/siderolabs/image-factory/commit/a4932a284e909dc64f93fc5c5ff57bdf9e2e324b) chore: reduce memory usage
* [`1729190`](https://github.com/siderolabs/image-factory/commit/172919025e6ed5cbaf95ca2b1d9b149c6ae26c76) chore: support gcr.io keychain for registry auth
* [`1389813`](https://github.com/siderolabs/image-factory/commit/1389813533812ba1999f3158e3128499dca28177) release(v0.6.4): prepare release
* [`b7c7c16`](https://github.com/siderolabs/image-factory/commit/b7c7c161a20c6de3a85c5824b1cc1b3fd62ca014) fix: secureboot pxe
* [`67eb663`](https://github.com/siderolabs/image-factory/commit/67eb663d8016c05a4c32ab1046b3814da5f74fe8) release(v0.6.3): prepare release
* [`46f4104`](https://github.com/siderolabs/image-factory/commit/46f41046f02da991491f1378bf29eab67a18d91f) feat: update to Talos 1.9.0-beta.1
* [`cbf8cc9`](https://github.com/siderolabs/image-factory/commit/cbf8cc9af3d8e5e5ea29bf5f15f45ec90ff65e7d) feat: add Turing RK1 as option
</p>
</details>

### Dependency Changes

* **filippo.io/age**                                   v1.2.0 -> v1.2.1
* **github.com/ProtonMail/gopenpgp/v2**                v2.8.1 -> v2.8.2
* **github.com/aws/aws-sdk-go-v2**                     v1.32.6 -> v1.32.8
* **github.com/aws/aws-sdk-go-v2/config**              v1.28.6 -> v1.28.11
* **github.com/aws/aws-sdk-go-v2/credentials**         v1.17.47 -> v1.17.52
* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.17.43 -> v1.17.49
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.71.0 -> v1.72.3
* **github.com/cosi-project/runtime**                  v0.7.5 -> v0.9.0
* **github.com/cosi-project/state-etcd**               v0.4.1 -> v0.5.0
* **github.com/grpc-ecosystem/grpc-gateway/v2**        v2.24.0 -> v2.25.1
* **github.com/jonboulle/clockwork**                   7e524bd2b238 -> v0.5.0
* **github.com/prometheus/common**                     v0.60.1 -> v0.61.0
* **github.com/siderolabs/discovery-service**          v1.0.8 -> v1.0.9
* **github.com/siderolabs/gen**                        v0.7.0 -> v0.8.0
* **github.com/siderolabs/go-talos-support**           v0.1.1 -> v0.1.2
* **github.com/siderolabs/image-factory**              v0.6.2 -> v0.6.5
* **github.com/siderolabs/omni/client**                v0.42.1 -> v0.45.0
* **github.com/siderolabs/talos/pkg/machinery**        v1.9.0-beta.1 -> v1.10.0-alpha.0
* **github.com/zitadel/oidc/v3**                       v3.33.1 -> v3.34.0
* **golang.org/x/crypto**                              v0.29.0 -> v0.32.0
* **golang.org/x/net**                                 v0.31.0 -> v0.34.0
* **golang.org/x/tools**                               v0.27.0 -> v0.29.0
* **golang.zx2c4.com/wireguard/wgctrl**                925a1e7659e6 -> a9ab2273dd10
* **google.golang.org/grpc**                           v1.68.0 -> v1.69.4
* **google.golang.org/protobuf**                       v1.35.2 -> v1.36.2
* **k8s.io/api**                                       v0.32.0-rc.1 -> v0.32.0
* **k8s.io/apimachinery**                              v0.32.0-rc.1 -> v0.32.0
* **k8s.io/client-go**                                 v0.32.0-rc.1 -> v0.32.0
* **sigs.k8s.io/controller-runtime**                   v0.19.3 -> v0.19.4

Previous release can be found at [v0.45.0](https://github.com/siderolabs/omni/releases/tag/v0.45.0)

## [Omni 0.45.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.45.0-beta.0) (2024-12-12)

Welcome to the v0.45.0-beta.0 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Service Account Management UI

It is now possible to view, create, delete and edit service account in the Omni web UI.


### New SBC Support

Turing RK1 SBC installation media can now be downloaded from Omni.


### User Management CLI

`omnictl` now has new commands for user management to make it consistent with the UI:

- `omnictl user list`
- `omnictl user create [email] --role [role]
- `omnictl user delete [email]`
- `omnictl user set-role email --role [role]`


### Contributors

* Utku Ozdemir
* Noel Georgi
* Andrey Smirnov
* Artem Chernyshev
* Dmitriy Matrenichev
* Christopher Gill
* Nico Berlee

### Changes
<details><summary>23 commits</summary>
<p>

* [`99693cf`](https://github.com/siderolabs/omni/commit/99693cf0b2954787ccdb6a627d3d532149a1c5ad) test: assert power on/off status in static infra provider tests
* [`471831c`](https://github.com/siderolabs/omni/commit/471831cb4966f6c2574059d566234fc76c0e2525) test: assert machine labels in static infra provider tests
* [`8aeff65`](https://github.com/siderolabs/omni/commit/8aeff65edc5272d7b5c572a052de9d85f1832599) feat: update Talos machinery to final 1.9.0-beta.1
* [`a7b603e`](https://github.com/siderolabs/omni/commit/a7b603e496c9e446d1089de049604127f033cd11) feat: implement CLI commands for user management
* [`bbbf6f2`](https://github.com/siderolabs/omni/commit/bbbf6f2c770914a6c5c98ecec39113dfa832d122) feat: add Turing RK1 SoM to SBCs dropdown
* [`d8e3aad`](https://github.com/siderolabs/omni/commit/d8e3aadb1b48e6dc3761097d265fc8a905a17747) chore: handle renamed drm extensions
* [`6f3ce0d`](https://github.com/siderolabs/omni/commit/6f3ce0d2a154da28eb14866c2297d0953e2df54d) fix: regenerate wipe id of infra machines only once per de-allocation
* [`ce40338`](https://github.com/siderolabs/omni/commit/ce403382d64cdfe53d9d3d2c80e5cfd6fe47af0a) feat: add `rejected` state to infra machine acceptance status
* [`8a64ba7`](https://github.com/siderolabs/omni/commit/8a64ba77b0ce508c890125d0e55b6eb0ed5a598f) chore: bump COSI runtime to `v0.7.5`
* [`815b2b0`](https://github.com/siderolabs/omni/commit/815b2b0a7e292b9f971ef766bff151beef4d0a21) feat: allow specifying extra kernel args on infra machines
* [`95c22be`](https://github.com/siderolabs/omni/commit/95c22be714c4aa512ba05f2b36dbd394b25ba37a) chore: bump deps
* [`e84b10a`](https://github.com/siderolabs/omni/commit/e84b10a9af29ae5248897f1d3a311a7801f906eb) fix: fix panic in `ConfigPatchRequestController`
* [`ac362f9`](https://github.com/siderolabs/omni/commit/ac362f9727f8259969690e65e75923531a4c4aa2) fix: ignore `Unimplemented` errors in `MetaDelete` calls
* [`377b550`](https://github.com/siderolabs/omni/commit/377b55095e3b45a77fa0e97a5ce88b687a9f5929) feat: update Talos machinery to v1.9.0-beta.0
* [`776bc65`](https://github.com/siderolabs/omni/commit/776bc65b7ca09bf4e5102d0c05cfafa7ce58b389) test: add static infra provider (bare-metal provider) integration tests
* [`d879c6e`](https://github.com/siderolabs/omni/commit/d879c6ef819b807e093b30091e83f547d3fc7426) chore: bump discovery service to `v1.0.8`
* [`5a26d4c`](https://github.com/siderolabs/omni/commit/5a26d4c7ac9d2403d781844f2865797650a8ecd5) feat: add resources and controllers for bare metal infra provider
* [`033e051`](https://github.com/siderolabs/omni/commit/033e051994203cd9878da0542b3b8714dc7452f5) chore: bump Go to 1.23.3, rekres, regenerate sources, make linters happy
* [`9085e82`](https://github.com/siderolabs/omni/commit/9085e82822d406ed39797523f6e46eb07dadde07) fix: use the custom image factory host in backend and frontend
* [`7fd2817`](https://github.com/siderolabs/omni/commit/7fd2817d05ff5e3fa3bfbfa0dd3436cb5a7f942d) chore: deprecate Talos 1.4
* [`d46fe7e`](https://github.com/siderolabs/omni/commit/d46fe7e8ad55b5d02dcdb39682add96bb323cf4a) fix: fix compose.yaml typo
* [`e4586f4`](https://github.com/siderolabs/omni/commit/e4586f4a3449a9c0f2ee033458a588095362fdab) fix: properly set up provider for autoprovision tests
* [`05ab993`](https://github.com/siderolabs/omni/commit/05ab993d3da6aaae92b566e25adf3b83db09acd3) fix: properly map config patch requests in the infra provision ctrl
</p>
</details>

### Changes from siderolabs/crypto
<details><summary>1 commit</summary>
<p>

* [`0d45dee`](https://github.com/siderolabs/crypto/commit/0d45deefbcdd4bd6b6e549433b859083df55fc16) chore: bump deps
</p>
</details>

### Changes from siderolabs/discovery-service
<details><summary>1 commit</summary>
<p>

* [`2bb245a`](https://github.com/siderolabs/discovery-service/commit/2bb245aa38c1d59b671d5fb25b6fa802f408c521) fix: do not register storage metric collectors if it is not enabled
</p>
</details>

### Changes from siderolabs/go-kubernetes
<details><summary>4 commits</summary>
<p>

* [`06f07ab`](https://github.com/siderolabs/go-kubernetes/commit/06f07ab00042411a20344ebc539bb02b123f7a6a) chore: add authorization config api version
* [`5ca8ab1`](https://github.com/siderolabs/go-kubernetes/commit/5ca8ab18d87a601f69134d988a81389f9bedc581) chore: kube-apiserver authorization config file support
* [`0f62a7e`](https://github.com/siderolabs/go-kubernetes/commit/0f62a7e3c006d56601764088011d5dd20f70a7a5) feat: add one more deprecation/removal for v1.32
* [`87d2e8e`](https://github.com/siderolabs/go-kubernetes/commit/87d2e8e664c3e3e64403bcfcfe2f8691f60c6481) feat: add one more deprecation for 1.32.0-beta.0
</p>
</details>

### Changes from siderolabs/image-factory
<details><summary>11 commits</summary>
<p>

* [`d0dcfe5`](https://github.com/siderolabs/image-factory/commit/d0dcfe52bea5f5a6f2f0856c9044478c91087669) release(v0.6.2): prepare release
* [`a8cdc21`](https://github.com/siderolabs/image-factory/commit/a8cdc21f87c5a8b9b5e36ca992b59e9274b199a6) feat: update dependencies for Talos 1.9
* [`b7f7fd3`](https://github.com/siderolabs/image-factory/commit/b7f7fd32cf4f26528c5aef2e45411780e1abbbb3) chore: add hash errata for tarball headers
* [`370c137`](https://github.com/siderolabs/image-factory/commit/370c13708a20300b9ba9bb78113b933a40474e83) fix: vmware build assets on non-amd64
* [`c102c95`](https://github.com/siderolabs/image-factory/commit/c102c95df616113654d67cca8df402ec1f996306) chore: alias i915/amdgpu extensions to new name
* [`b7b4c71`](https://github.com/siderolabs/image-factory/commit/b7b4c71117449ec72cdd3ee26c011e73a5f30737) release(v0.6.1): prepare release
* [`96c8455`](https://github.com/siderolabs/image-factory/commit/96c845517aeda1ea7b4c80c6203ce7e5643f33ab) chore: bump generated data
* [`cc1074b`](https://github.com/siderolabs/image-factory/commit/cc1074b2b72506612fcfcf5d2fa9e3c439dc2181) release(v0.6.0): prepare release
* [`0ca8240`](https://github.com/siderolabs/image-factory/commit/0ca82406f2de2b9bed4519dc96105816d528fb38) fix: secureboot iso gen
* [`8e66370`](https://github.com/siderolabs/image-factory/commit/8e66370f4df437ba5c2df2290657a8272c74183f) feat: hide Talos metal agent extension on the UI
* [`d98b007`](https://github.com/siderolabs/image-factory/commit/d98b00764522849dc226140e89216c254164da22) feat: reword wizard using GitHub Copilot
</p>
</details>

### Dependency Changes

* **github.com/ProtonMail/gopenpgp/v2**                v2.7.5 -> v2.8.1
* **github.com/aws/aws-sdk-go-v2**                     v1.32.3 -> v1.32.6
* **github.com/aws/aws-sdk-go-v2/config**              v1.28.1 -> v1.28.6
* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.17.35 -> v1.17.43
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.66.2 -> v1.71.0
* **github.com/aws/smithy-go**                         v1.22.0 -> v1.22.1
* **github.com/containers/image/v5**                   v5.32.2 -> v5.33.0
* **github.com/cosi-project/runtime**                  v0.7.1 -> v0.7.5
* **github.com/cosi-project/state-etcd**               v0.4.0 -> v0.4.1
* **github.com/emicklei/dot**                          v1.6.2 -> v1.6.4
* **github.com/grpc-ecosystem/grpc-gateway/v2**        v2.23.0 -> v2.24.0
* **github.com/johannesboyne/gofakes3**                2db7ccb81e19 -> 0da3aa9c32ca
* **github.com/siderolabs/crypto**                     v0.5.0 -> v0.5.1
* **github.com/siderolabs/discovery-service**          v1.0.7 -> v1.0.8
* **github.com/siderolabs/go-kubernetes**              v0.2.14 -> v0.2.17
* **github.com/siderolabs/image-factory**              v0.5.0 -> v0.6.2
* **github.com/siderolabs/talos/pkg/machinery**        v1.8.2 -> v1.9.0-beta.1
* **github.com/stretchr/testify**                      v1.9.0 -> v1.10.0
* **github.com/zitadel/oidc/v3**                       v3.32.1 -> v3.33.1
* **go.etcd.io/etcd/client/pkg/v3**                    v3.5.16 -> v3.5.17
* **go.etcd.io/etcd/client/v3**                        v3.5.16 -> v3.5.17
* **go.etcd.io/etcd/server/v3**                        v3.5.16 -> v3.5.17
* **golang.org/x/crypto**                              v0.28.0 -> v0.29.0
* **golang.org/x/net**                                 v0.30.0 -> v0.31.0
* **golang.org/x/sync**                                v0.8.0 -> v0.10.0
* **golang.org/x/tools**                               v0.26.0 -> v0.27.0
* **google.golang.org/grpc**                           v1.67.1 -> v1.68.0
* **google.golang.org/protobuf**                       v1.35.1 -> v1.35.2
* **k8s.io/api**                                       v0.31.2 -> v0.32.0-rc.1
* **k8s.io/apimachinery**                              v0.31.2 -> v0.32.0-rc.1
* **k8s.io/client-go**                                 v0.31.2 -> v0.32.0-rc.1
* **sigs.k8s.io/controller-runtime**                   v0.19.1 -> v0.19.3

Previous release can be found at [v0.44.0](https://github.com/siderolabs/omni/releases/tag/v0.44.0)

## [Omni 0.44.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.44.0-beta.0) (2024-11-06)

Welcome to the v0.44.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Automatically Resolve Cluster in `talosctl`

`talosctl` command now works without `--cluster` flag when using instance wide Talos config.
Omni will automatically resolve the correct cluster.


### Reset Removed Machines

Omni will now try to wipe Talos installation from the machines which are removed from the instance.


### Contributors

* Artem Chernyshev
* Dmitriy Matrenichev
* Andrey Smirnov
* Noel Georgi
* Tijmen Blaauw - van den Brink

### Changes
<details><summary>25 commits</summary>
<p>

* [`fe0fc17`](https://github.com/siderolabs/omni/commit/fe0fc1763b6a45d43f9b75cb5d665448cf5ce6c9) feat: support creating config patches in the infrastructure providers
* [`3e8bc8d`](https://github.com/siderolabs/omni/commit/3e8bc8d8cab4c8419b7bde2291dfa2436c0c3b8a) feat: enable watch retries on Omni side
* [`23ccdb5`](https://github.com/siderolabs/omni/commit/23ccdb50b82eae7192bf45385052bbff696fd2c0) chore: bump dependencies
* [`be3e67c`](https://github.com/siderolabs/omni/commit/be3e67ce57dc630a236ddb58a97a85123544f308) fix: include NodeJS types in the frontend build
* [`abaee03`](https://github.com/siderolabs/omni/commit/abaee033da9dbad86822487a477bf3110d73942d) fix: make web UI show favicon
* [`cc59192`](https://github.com/siderolabs/omni/commit/cc5919273b62320034a13c3d75902d29e94fd499) feat: reset machine when it's removed from Omni
* [`900987b`](https://github.com/siderolabs/omni/commit/900987bf510a58e7f2823c31dbce211d5110cfc1) test: disable secure boot in e2e tests
* [`58159e4`](https://github.com/siderolabs/omni/commit/58159e419c6ed5546bf8235a31719a9f0c50e87f) feat: automatically resolve cluster in `talosctl` calls
* [`8da2328`](https://github.com/siderolabs/omni/commit/8da23286240bd505233d056c7c134ae3dd999d77) fix: remove `MaintenanceConfigPatchController` finalizers
* [`21455d9`](https://github.com/siderolabs/omni/commit/21455d928594e8f3c9e4bea93e1444e8074e9590) fix: properly show the current manifests in the bootstrap manifest sync
* [`c904e3a`](https://github.com/siderolabs/omni/commit/c904e3a6d0f29f8c807ad8d01534643c8dd70882) chore: do not audit log `GET` requests to k8s
* [`62917e7`](https://github.com/siderolabs/omni/commit/62917e7890a67cd195c1d93166a7401c81178b6f) fix: use proper selectors in the MachineClass create UI
* [`4b4088d`](https://github.com/siderolabs/omni/commit/4b4088d38de9b74bba1c8072525ec32171958ea3) fix: do not read Talos versions from the image registry ever
* [`b3dc48a`](https://github.com/siderolabs/omni/commit/b3dc48ad335e1fc7202a72693b73251e6a0cf886) chore: bump dependencies
* [`9d0a512`](https://github.com/siderolabs/omni/commit/9d0a5121f381f3823d75666046117e1c5cec63b0) fix: filter block devices with UNKNOWN type
* [`8c737ba`](https://github.com/siderolabs/omni/commit/8c737ba699ffb7c1d8d7ad71a7047c0376980a02) fix: fetch Talos version from the image factory
* [`83554e5`](https://github.com/siderolabs/omni/commit/83554e55967843badd146269e8946f75ed01c602) fix: do not set empty initial labels in the UI
* [`98315a9`](https://github.com/siderolabs/omni/commit/98315a938c9f976ef1d509f65b855c58d16c4576) fix: do not build acompat docker image
* [`cd1f2bd`](https://github.com/siderolabs/omni/commit/cd1f2bd34ccec0f26e3620fa6577d5bb84bb4d53) fix: build arm64 integration tests executable
* [`c754cdc`](https://github.com/siderolabs/omni/commit/c754cdc0d76b9385ab81d515fbce0644228d8fe4) feat: support insecure localhost infra provider access mode
* [`284e8b5`](https://github.com/siderolabs/omni/commit/284e8b5077cc08d78fff6d346422eaee8c0dac11) fix: introduce timeout in the etcd healthchecks in the machine set ctrl
* [`d7b92e7`](https://github.com/siderolabs/omni/commit/d7b92e773430ec568c8330d21c90806d49b4ea06) chore: remove `ip_address` field from audit log `session`
* [`18b13ea`](https://github.com/siderolabs/omni/commit/18b13ea67b1129792853cc8d158eac021ab4babb) chore: add basic helm chart
* [`1544b9c`](https://github.com/siderolabs/omni/commit/1544b9ca07fcc81a47d01159e6c12a6a4cd22c5f) chore: move from Codec to CodecV2
* [`1c12dfc`](https://github.com/siderolabs/omni/commit/1c12dfca93bb63977db65ac89fecfe36dc6d88a7) fix: properly return error from `config.Init`
</p>
</details>

### Changes from siderolabs/crypto
<details><summary>1 commit</summary>
<p>

* [`58b2f92`](https://github.com/siderolabs/crypto/commit/58b2f9291c7e763a7210cfa681f88a7fa2230bf3) chore: use HTTP/2 ALPN by default
</p>
</details>

### Changes from siderolabs/discovery-api
<details><summary>1 commit</summary>
<p>

* [`005e92c`](https://github.com/siderolabs/discovery-api/commit/005e92cf4ad0059334bfd35285a97c85f12aa263) chore: rekres and regen
</p>
</details>

### Changes from siderolabs/discovery-client
<details><summary>1 commit</summary>
<p>

* [`b74fb90`](https://github.com/siderolabs/discovery-client/commit/b74fb9039fcfd8db9d6becf3044f9f41f387ea27) fix: allow custom TLS config for the client
</p>
</details>

### Changes from siderolabs/discovery-service
<details><summary>4 commits</summary>
<p>

* [`b8da986`](https://github.com/siderolabs/discovery-service/commit/b8da986b5ab4acf029df40f0116d1f020c370a3e) fix: reduce memory allocations (logger)
* [`3367c7b`](https://github.com/siderolabs/discovery-service/commit/3367c7b34912ac742dd6fe8e3fe758f61225cddf) chore: add proto-codec/codec
* [`efbb10b`](https://github.com/siderolabs/discovery-service/commit/efbb10bdfd3c027c5c1942b34e1b803d8f8fa10a) fix: properly parse peer address
* [`cf39974`](https://github.com/siderolabs/discovery-service/commit/cf39974104bbfc291289736847cf05e3a205301e) feat: support direct TLS serving
</p>
</details>

### Changes from siderolabs/gen
<details><summary>3 commits</summary>
<p>

* [`e847d2a`](https://github.com/siderolabs/gen/commit/e847d2ace9ede4a17283426dfbc8229121f2909b) chore: add more utilities to xiter
* [`f3c5a2b`](https://github.com/siderolabs/gen/commit/f3c5a2b5aba74e4935d073a0135c4904ef3bbfef) chore: add `Empty` and `Empty2` iterators
* [`c53b90b`](https://github.com/siderolabs/gen/commit/c53b90b4a418b8629d938af06900249ce5acd9e6) chore: add packages xiter/xstrings/xbytes
</p>
</details>

### Changes from siderolabs/go-circular
<details><summary>1 commit</summary>
<p>

* [`9a0f7b0`](https://github.com/siderolabs/go-circular/commit/9a0f7b02c80ad6c2d953b2d3dd388c56e89363ea) fix: multiple data race issues
</p>
</details>

### Changes from siderolabs/go-kubernetes
<details><summary>3 commits</summary>
<p>

* [`e56a7f6`](https://github.com/siderolabs/go-kubernetes/commit/e56a7f65808b90058df16a4133f19484beeedc31) fix: update deprecations based on Kubernetes 1.32.0-alpha.3
* [`381f251`](https://github.com/siderolabs/go-kubernetes/commit/381f251662eaae9b48470ce00f504c2c64187612) feat: update for Kubernetes 1.32
* [`0e767c5`](https://github.com/siderolabs/go-kubernetes/commit/0e767c5350afc2e11ac5dca718cdc3f8853c52f7) chore: k8s 1.31 kube-scheduler health endpoints
</p>
</details>

### Changes from siderolabs/grpc-proxy
<details><summary>2 commits</summary>
<p>

* [`de1c628`](https://github.com/siderolabs/grpc-proxy/commit/de1c6286b7d16d8485bf8bb55c8783c8773851a0) fix: copy data from big frame msg
* [`ef47ec7`](https://github.com/siderolabs/grpc-proxy/commit/ef47ec77d2a9f0f42e713d456943dfe9ee86a629) chore: upgrade Codec implementations and usages to Codec2
</p>
</details>

### Changes from siderolabs/proto-codec
<details><summary>3 commits</summary>
<p>

* [`0d84c65`](https://github.com/siderolabs/proto-codec/commit/0d84c652784543012f43f8c8d4358c160b27577e) chore: add support for gogo protobuf generator
* [`19f8d2e`](https://github.com/siderolabs/proto-codec/commit/19f8d2e5840c19937c60cee0c681343ab658f678) chore: add kres
* [`e038bb4`](https://github.com/siderolabs/proto-codec/commit/e038bb42f2be8b80ca09e46bb8704be06a413919) Initial commit
</p>
</details>

### Changes from siderolabs/siderolink
<details><summary>2 commits</summary>
<p>

* [`1893385`](https://github.com/siderolabs/siderolink/commit/1893385fe45bf110357a770d31b06f5d79403065) fix: initialize tls listener properly
* [`6c8fa1f`](https://github.com/siderolabs/siderolink/commit/6c8fa1fcaa069a82aea9c24fdd0627ab4b220f5e) feat: allow listening over TLS for SideroLink API
</p>
</details>

### Dependency Changes

* **github.com/adrg/xdg**                              v0.5.0 -> v0.5.3
* **github.com/aws/aws-sdk-go-v2**                     v1.30.4 -> v1.32.3
* **github.com/aws/aws-sdk-go-v2/config**              v1.27.31 -> v1.28.1
* **github.com/aws/aws-sdk-go-v2/credentials**         v1.17.30 -> v1.17.42
* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.17.16 -> v1.17.35
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.61.0 -> v1.66.2
* **github.com/aws/smithy-go**                         v1.20.4 -> v1.22.0
* **github.com/cosi-project/runtime**                  v0.6.3 -> v0.7.1
* **github.com/cosi-project/state-etcd**               v0.3.2 -> v0.4.0
* **github.com/fsnotify/fsnotify**                     v1.7.0 -> v1.8.0
* **github.com/golang-jwt/jwt/v4**                     v4.5.0 -> v4.5.1
* **github.com/grpc-ecosystem/grpc-gateway/v2**        v2.22.0 -> v2.23.0
* **github.com/hashicorp/vault/api**                   v1.14.0 -> v1.15.0
* **github.com/hashicorp/vault/api/auth/kubernetes**   v0.7.0 -> v0.8.0
* **github.com/johannesboyne/gofakes3**                edd0227ffc37 -> 2db7ccb81e19
* **github.com/jonboulle/clockwork**                   fc59783b0293 -> 7e524bd2b238
* **github.com/prometheus/client_golang**              v1.20.2 -> v1.20.5
* **github.com/prometheus/common**                     v0.57.0 -> v0.60.1
* **github.com/siderolabs/crypto**                     v0.4.4 -> v0.5.0
* **github.com/siderolabs/discovery-api**              v0.1.4 -> v0.1.5
* **github.com/siderolabs/discovery-client**           v0.1.9 -> v0.1.10
* **github.com/siderolabs/discovery-service**          v1.0.3 -> v1.0.7
* **github.com/siderolabs/gen**                        v0.5.0 -> v0.7.0
* **github.com/siderolabs/go-circular**                v0.2.0 -> v0.2.1
* **github.com/siderolabs/go-kubernetes**              v0.2.11 -> v0.2.14
* **github.com/siderolabs/grpc-proxy**                 v0.4.1 -> v0.5.1
* **github.com/siderolabs/omni/client**                v0.39.1 -> v0.42.1
* **github.com/siderolabs/proto-codec**                v0.1.1 **_new_**
* **github.com/siderolabs/siderolink**                 v0.3.9 -> v0.3.11
* **github.com/siderolabs/talos/pkg/machinery**        v1.8.0 -> v1.8.2
* **github.com/zitadel/logging**                       v0.6.0 -> v0.6.1
* **github.com/zitadel/oidc/v3**                       v3.28.2 -> v3.32.1
* **go.etcd.io/etcd/client/pkg/v3**                    v3.5.15 -> v3.5.16
* **go.etcd.io/etcd/client/v3**                        v3.5.15 -> v3.5.16
* **go.etcd.io/etcd/server/v3**                        v3.5.15 -> v3.5.16
* **golang.org/x/crypto**                              v0.26.0 -> v0.28.0
* **golang.org/x/net**                                 v0.28.0 -> v0.30.0
* **golang.org/x/tools**                               v0.24.0 -> v0.26.0
* **google.golang.org/grpc**                           v1.66.0 -> v1.67.1
* **google.golang.org/protobuf**                       v1.34.2 -> v1.35.1
* **k8s.io/api**                                       v0.31.0 -> v0.31.2
* **k8s.io/client-go**                                 v0.31.0 -> v0.31.2
* **sigs.k8s.io/controller-runtime**                   v0.19.0 -> v0.19.1

Previous release can be found at [v0.43.0](https://github.com/siderolabs/omni/releases/tag/v0.43.0)

## [Omni 0.43.0](https://github.com/siderolabs/omni/releases/tag/v0.43.0) (2024-10-11)

Welcome to the v0.43.0 release of Omni!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### `gRPC` Tunnel

`gRPC` tunnel for wireguard can now be enabled when downloading the installation media from Omni.


### Talos Diagnostics

Omni now shows [Talos diagnostics information](https://www.talos.dev/v1.8/introduction/what-is-new/#diagnostics) for Talos >= 1.8.0.


### Contributors

* Artem Chernyshev
* Utku Ozdemir
* Dmitriy Matrenichev
* Andrey Smirnov
* Birger J. Nordølum
* Justin Garrison

### Changes
<details><summary>27 commits</summary>
<p>

* [`397f204`](https://github.com/siderolabs/omni/commit/397f204865f0912ffe65505f3bfd28683b3292ac) feat: display pending machine requests in the machine sets
* [`0d66194`](https://github.com/siderolabs/omni/commit/0d66194bd1e7b40bc7e19e9d663fc313f06ad0b7) release(v0.43.0-beta.0): prepare release
* [`4084b6e`](https://github.com/siderolabs/omni/commit/4084b6e9d7aeb09c7bce58d3b1d8db90b5e30f43) fix: get proper IP from peer metadata
* [`d547889`](https://github.com/siderolabs/omni/commit/d547889b7b9bbdd4af724fd85d22517ee403797a) fix: filter requests in the infra provision controller
* [`d1c9d9d`](https://github.com/siderolabs/omni/commit/d1c9d9df4a94ac37f2e498644d378315d0b7bb47) chore: set `peer.address` to inform about IP status
* [`23a4092`](https://github.com/siderolabs/omni/commit/23a4092af534062131c23fcc012d82f36e62822e) chore: refactor code
* [`5630d83`](https://github.com/siderolabs/omni/commit/5630d83e5d340630204f45c8b72ac84966293ecf) fix: ignore parse errors in the log parser
* [`8334c59`](https://github.com/siderolabs/omni/commit/8334c59482d36702bd6b61511227b448e3e0557c) chore: add a way to get provider data in the infra provider
* [`cc71fb6`](https://github.com/siderolabs/omni/commit/cc71fb624a511308e7044c53b9c84fe7db78252b) feat: support auto provisioned machine classes
* [`41c3bd5`](https://github.com/siderolabs/omni/commit/41c3bd523210182ab9916061ddf4737fe79e2f40) fix: support whitespaces in the label selectors
* [`99191c6`](https://github.com/siderolabs/omni/commit/99191c645a9c493174783a5381294caaa4c40dd6) feat: integrate with Talos diagnostics
* [`dcf89d9`](https://github.com/siderolabs/omni/commit/dcf89d9d1166a65b17e5d696c63ed9e6ee6ea4f0) feat: update Omni for Talos 1.8 machinery
* [`a04b07f`](https://github.com/siderolabs/omni/commit/a04b07f3096e6b5ca045077cb4ae09b9027fb469) test: fix the error message in infra test
* [`3e3e53b`](https://github.com/siderolabs/omni/commit/3e3e53b3368577b4ecb26201db4dacfdf2150e2f) chore: fix capitalization of wireguard
* [`f69ff37`](https://github.com/siderolabs/omni/commit/f69ff3761cd879dd7403c9ff2aa1bbf3273eb78f) feat: make infra provider report back it's information: schema, name
* [`7555312`](https://github.com/siderolabs/omni/commit/7555312bdcebff4970a0c9dc93675d1deb957e70) fix: get rid of the exceptions in the ui
* [`8e48723`](https://github.com/siderolabs/omni/commit/8e4872393e6603698114a674df44fc4b287e787b) feat: support attaching machine sets to a machine request sets
* [`bb2f52d`](https://github.com/siderolabs/omni/commit/bb2f52d13bd2752c2d63bcfe9025c4e070d2481c) chore: drop machine class status and machine set pressure resources
* [`3ef1f85`](https://github.com/siderolabs/omni/commit/3ef1f85f58edf346204e076e25b057ce14eeffed) fix: call deprovision only after the machine request status is deleted
* [`423f729`](https://github.com/siderolabs/omni/commit/423f7294009105c3f58b4df5409f0803e30040ea) chore: bump default versions: Talos `1.7.6`, Kubernetes `1.30.5`
* [`c4a4151`](https://github.com/siderolabs/omni/commit/c4a4151d7a5d9030ec82fab434932f3d002e59cf) feat: allow specifying grpc tunnel option explicitly for install media
* [`bb14ed6`](https://github.com/siderolabs/omni/commit/bb14ed6dacf7b6356f65f3fc2f47d96cc5cdedb3) fix: parse machine labels and extensions as slices in `omnictl download`
* [`9e033d7`](https://github.com/siderolabs/omni/commit/9e033d7c10beced3031e41c9613b1065edf5ceab) docs: update omni template so docs are easier
* [`4c329db`](https://github.com/siderolabs/omni/commit/4c329dba6799184e83fc164a49296e74e92ea80a) fix: update COSI runtime
* [`81e08eb`](https://github.com/siderolabs/omni/commit/81e08eb38bb3677d62e9d0e9bbfe9eca11f7a51c) test: run infra integration tests against Talemu provider
* [`f83cf3b`](https://github.com/siderolabs/omni/commit/f83cf3b210cbf40f1c75b1eb5012e235fad2e923) fix: pin apexcharts version to 3.45.2
* [`e3d46f9`](https://github.com/siderolabs/omni/commit/e3d46f949c10c4d3b6cdc79919d0cfcfef3ec4a3) feat: implement compression of config fields on resources
</p>
</details>

### Changes since v0.43.0-beta.0
<details><summary>1 commit</summary>
<p>

* [`397f204`](https://github.com/siderolabs/omni/commit/397f204865f0912ffe65505f3bfd28683b3292ac) feat: display pending machine requests in the machine sets
</p>
</details>

### Dependency Changes

* **github.com/cosi-project/runtime**            v0.6.1 -> v0.6.3
* **github.com/cosi-project/state-etcd**         v0.3.1 -> v0.3.2
* **github.com/santhosh-tekuri/jsonschema/v5**   v5.3.1 **_new_**
* **github.com/siderolabs/talos/pkg/machinery**  6f7c3a8e5c63 -> v1.8.0

Previous release can be found at [v0.42.0](https://github.com/siderolabs/omni/releases/tag/v0.42.0)

## [Omni 0.43.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.43.0-beta.0) (2024-10-09)

Welcome to the v0.43.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### `gRPC` Tunnel

`gRPC` tunnel for wireguard can now be enabled when downloading the installation media from Omni.


### Talos Diagnostics

Omni now shows [Talos diagnostics information](https://www.talos.dev/v1.8/introduction/what-is-new/#diagnostics) for Talos >= 1.8.0.


### Contributors

* Artem Chernyshev
* Utku Ozdemir
* Dmitriy Matrenichev
* Andrey Smirnov
* Birger J. Nordølum
* Justin Garrison

### Changes
<details><summary>25 commits</summary>
<p>

* [`4084b6e`](https://github.com/siderolabs/omni/commit/4084b6e9d7aeb09c7bce58d3b1d8db90b5e30f43) fix: get proper IP from peer metadata
* [`d547889`](https://github.com/siderolabs/omni/commit/d547889b7b9bbdd4af724fd85d22517ee403797a) fix: filter requests in the infra provision controller
* [`d1c9d9d`](https://github.com/siderolabs/omni/commit/d1c9d9df4a94ac37f2e498644d378315d0b7bb47) chore: set `peer.address` to inform about IP status
* [`23a4092`](https://github.com/siderolabs/omni/commit/23a4092af534062131c23fcc012d82f36e62822e) chore: refactor code
* [`5630d83`](https://github.com/siderolabs/omni/commit/5630d83e5d340630204f45c8b72ac84966293ecf) fix: ignore parse errors in the log parser
* [`8334c59`](https://github.com/siderolabs/omni/commit/8334c59482d36702bd6b61511227b448e3e0557c) chore: add a way to get provider data in the infra provider
* [`cc71fb6`](https://github.com/siderolabs/omni/commit/cc71fb624a511308e7044c53b9c84fe7db78252b) feat: support auto provisioned machine classes
* [`41c3bd5`](https://github.com/siderolabs/omni/commit/41c3bd523210182ab9916061ddf4737fe79e2f40) fix: support whitespaces in the label selectors
* [`99191c6`](https://github.com/siderolabs/omni/commit/99191c645a9c493174783a5381294caaa4c40dd6) feat: integrate with Talos diagnostics
* [`dcf89d9`](https://github.com/siderolabs/omni/commit/dcf89d9d1166a65b17e5d696c63ed9e6ee6ea4f0) feat: update Omni for Talos 1.8 machinery
* [`a04b07f`](https://github.com/siderolabs/omni/commit/a04b07f3096e6b5ca045077cb4ae09b9027fb469) test: fix the error message in infra test
* [`3e3e53b`](https://github.com/siderolabs/omni/commit/3e3e53b3368577b4ecb26201db4dacfdf2150e2f) chore: fix capitalization of wireguard
* [`f69ff37`](https://github.com/siderolabs/omni/commit/f69ff3761cd879dd7403c9ff2aa1bbf3273eb78f) feat: make infra provider report back it's information: schema, name
* [`7555312`](https://github.com/siderolabs/omni/commit/7555312bdcebff4970a0c9dc93675d1deb957e70) fix: get rid of the exceptions in the ui
* [`8e48723`](https://github.com/siderolabs/omni/commit/8e4872393e6603698114a674df44fc4b287e787b) feat: support attaching machine sets to a machine request sets
* [`bb2f52d`](https://github.com/siderolabs/omni/commit/bb2f52d13bd2752c2d63bcfe9025c4e070d2481c) chore: drop machine class status and machine set pressure resources
* [`3ef1f85`](https://github.com/siderolabs/omni/commit/3ef1f85f58edf346204e076e25b057ce14eeffed) fix: call deprovision only after the machine request status is deleted
* [`423f729`](https://github.com/siderolabs/omni/commit/423f7294009105c3f58b4df5409f0803e30040ea) chore: bump default versions: Talos `1.7.6`, Kubernetes `1.30.5`
* [`c4a4151`](https://github.com/siderolabs/omni/commit/c4a4151d7a5d9030ec82fab434932f3d002e59cf) feat: allow specifying grpc tunnel option explicitly for install media
* [`bb14ed6`](https://github.com/siderolabs/omni/commit/bb14ed6dacf7b6356f65f3fc2f47d96cc5cdedb3) fix: parse machine labels and extensions as slices in `omnictl download`
* [`9e033d7`](https://github.com/siderolabs/omni/commit/9e033d7c10beced3031e41c9613b1065edf5ceab) docs: update omni template so docs are easier
* [`4c329db`](https://github.com/siderolabs/omni/commit/4c329dba6799184e83fc164a49296e74e92ea80a) fix: update COSI runtime
* [`81e08eb`](https://github.com/siderolabs/omni/commit/81e08eb38bb3677d62e9d0e9bbfe9eca11f7a51c) test: run infra integration tests against Talemu provider
* [`f83cf3b`](https://github.com/siderolabs/omni/commit/f83cf3b210cbf40f1c75b1eb5012e235fad2e923) fix: pin apexcharts version to 3.45.2
* [`e3d46f9`](https://github.com/siderolabs/omni/commit/e3d46f949c10c4d3b6cdc79919d0cfcfef3ec4a3) feat: implement compression of config fields on resources
</p>
</details>

### Dependency Changes

* **github.com/cosi-project/runtime**            v0.6.1 -> v0.6.3
* **github.com/cosi-project/state-etcd**         v0.3.1 -> v0.3.2
* **github.com/santhosh-tekuri/jsonschema/v5**   v5.3.1 **_new_**
* **github.com/siderolabs/talos/pkg/machinery**  6f7c3a8e5c63 -> v1.8.0

Previous release can be found at [v0.42.0](https://github.com/siderolabs/omni/releases/tag/v0.42.0)

## [Omni 0.42.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.42.0-beta.0) (2024-09-06)

Welcome to the v0.42.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Allow multiple IP's in `siderolink-wireguard-advertised-addr` flag

The `siderolink-wireguard-advertised-addr` flag now accepts multiple IP addresses separated by commas. This is useful
when you have multiple IPs (IPv4 and IPv6) on the host machine and want to allow Talos nodes to connect to the Omni
using any of them.


### Audit log

It is now possible to get the audit log from the Omni. By default it's disabled. To enable, pass
`--audit-log-dir <dir>` to the Omni. The audit log will be written to the specified directory, separated by day.

Retention is set to 30 days (including the current day). The audit log is written in JSON format, where each entry is
a JSON object.

There are two ways to get audit log, and for both you need Admin role:
1. By using the UI: Simply click "Download audit log" in the main menu.
2. Using `omnictl audit-log` command. This command will stream the audit log from the Omni to the local machine stdout.


### Cluster Sort

Cluster list on Clusters page can now be sorted by name or creation date.
Before it was always sorted by creation date (newest first).


### TLS Cert Reload

Omni service can now reload the TLS certs without restart.


### Contributors

* Dmitriy Matrenichev
* Artem Chernyshev
* Andrey Smirnov
* Utku Ozdemir
* Noel Georgi
* Justin Garrison

### Changes
<details><summary>27 commits</summary>
<p>

* [`c076c3c`](https://github.com/siderolabs/omni/commit/c076c3cbf1e3e9e376447bc093155793bdbc9353) fix: filter readonly, CD and loop devices for 1.8
* [`0360422`](https://github.com/siderolabs/omni/commit/03604222ea9574f789f93b9b6a300f4777aecbe3) feat: support passing extra data through the siderolink join token
* [`381021e`](https://github.com/siderolabs/omni/commit/381021ee2f448c6b1757249295860bf252bba30e) fix: calculate requested and connected machines in the `ClusterStatus`
* [`7abb0f5`](https://github.com/siderolabs/omni/commit/7abb0f535354c1d82641f2cf2246b3f7d809019d) chore: bump deps
* [`464f699`](https://github.com/siderolabs/omni/commit/464f69913793922d0c9fd79f6566e0a7a437ea3b) chore: rename `CloudProvider` to `InfraProvider`
* [`bfe036e`](https://github.com/siderolabs/omni/commit/bfe036e136f6a80b0e9681e18e95f97de39d22c8) chore: allow to specify `start` and `end` time for `audit-log`
* [`e2f5795`](https://github.com/siderolabs/omni/commit/e2f579578941ef26f395edf20e7c0ab7a01e4dff) chore: allow multiple IP's for `siderolink-wireguard-advertised-addr` flag
* [`3c1defe`](https://github.com/siderolabs/omni/commit/3c1defe807979c61fb7736c3a0c7482f03f0f982) fix: fix spelling for hover text
* [`76ba670`](https://github.com/siderolabs/omni/commit/76ba670121f615ad98cd9b0459599978827ecca3) chore: allow users with admin role to download audit log from UI
* [`e8d578a`](https://github.com/siderolabs/omni/commit/e8d578a7ace02ff99cbe28291c8407391587284f) fix: add siderolink connection params to the infra provider interface
* [`4a82cd0`](https://github.com/siderolabs/omni/commit/4a82cd0e8f8369f23c2134966b388ed6c49afcd7) chore: rewrite renamed extension names on Talos version updates
* [`56c0394`](https://github.com/siderolabs/omni/commit/56c0394b32a8493c770cc7c0851815e3b38070a7) fix: always remove finalizers from the `ClusterMachineStatus`
* [`ce45042`](https://github.com/siderolabs/omni/commit/ce45042e08666da7c9bcd84f632d752091b5950a) feat: implement `MachineRequestSets` and support links cleanup flow
* [`85aaf1c`](https://github.com/siderolabs/omni/commit/85aaf1c87cee5c8c1c1bc54d83d2b3e952538b68) feat: support sorting cluster by name, creation time
* [`95c8210`](https://github.com/siderolabs/omni/commit/95c8210475ecbbc5ae50fc50687478e340257061) feat: implement base infra provider library
* [`a32a6fa`](https://github.com/siderolabs/omni/commit/a32a6fa44b6b9bf530b4a87a63dd0b8a2b34c13b) feat: reload TLS certs without restart
* [`00ae084`](https://github.com/siderolabs/omni/commit/00ae08486a691df08075705b4aa7602a97efe002) fix: delete upgrade meta key from nodes after upgrades
* [`3f5c0f8`](https://github.com/siderolabs/omni/commit/3f5c0f83b9ff11b68bb3de0de921c3a36e875d74) chore: enable 'github.com/planetscale/vtprotobuf' encoding
* [`34a8c36`](https://github.com/siderolabs/omni/commit/34a8c36a8b657e47b9b8f5db164ef2c30df8f6a3) chore: rekres to get BUSL license change date updated on releases
* [`bf188e4`](https://github.com/siderolabs/omni/commit/bf188e4ac118bfe9f46fa9938204d66c2a7bf538) chore: implement audit log reader
* [`5d48547`](https://github.com/siderolabs/omni/commit/5d48547c7fe55800fcb9fdd21108c0756ab35aa5) chore: use range-over-func iterators for resource iteration
* [`dc349c1`](https://github.com/siderolabs/omni/commit/dc349c177869f414a9c159b5a6cc58133444790c) chore: do a full generate with latest deps
* [`67f2e8d`](https://github.com/siderolabs/omni/commit/67f2e8dfc56d512c9b0a0d5838256396bfb37505) chore: print error on closing secondary storage backing store
* [`89e8a62`](https://github.com/siderolabs/omni/commit/89e8a623409cda3827e7fd72563d881c5d31c341) fix: pass the logger to machine logs circular buffer
* [`d2387d9`](https://github.com/siderolabs/omni/commit/d2387d98dd08f056b3ddb5102ff581ebb0be6b9b) fix: use a separate phase for the extensions installation
* [`cbfe7c9`](https://github.com/siderolabs/omni/commit/cbfe7c9d9f0b697ad902e4c16788ff890778e87d) chore: add periodic cleanup of old log files
* [`aea900f`](https://github.com/siderolabs/omni/commit/aea900f13abc461f24cb756d7f96578b1a118901) fix: display machines in tearing down state
</p>
</details>

### Changes from siderolabs/discovery-service
<details><summary>1 commit</summary>
<p>

* [`270f257`](https://github.com/siderolabs/discovery-service/commit/270f2575e71bc0ade00d1c58c2787c01d285dd74) chore: bump deps
</p>
</details>

### Changes from siderolabs/go-api-signature
<details><summary>2 commits</summary>
<p>

* [`8807c5e`](https://github.com/siderolabs/go-api-signature/commit/8807c5e8c84e78f382ee62d8425f4bfd85a1e547) fix: account for time truncation to a second resolution
* [`1b35ea8`](https://github.com/siderolabs/go-api-signature/commit/1b35ea8d3a334418aa273159ea5732ae0625a317) chore: bump deps and fix data race
</p>
</details>

### Changes from siderolabs/go-debug
<details><summary>1 commit</summary>
<p>

* [`c8f9b12`](https://github.com/siderolabs/go-debug/commit/c8f9b12c041a3242472ad56b970487432552d2be) chore: add support for Go 1.23
</p>
</details>

### Changes from siderolabs/go-talos-support
<details><summary>3 commits</summary>
<p>

* [`58f4f0f`](https://github.com/siderolabs/go-talos-support/commit/58f4f0fde6be11e5d5da37ceaab52286b4b0be05) chore: bump Go dependencies
* [`f9d46fd`](https://github.com/siderolabs/go-talos-support/commit/f9d46fd8a607a928dc0382f308ad577f36b0a8b8) fix: add `dns-resolve-cache` to the list of logs gathered
* [`69891cf`](https://github.com/siderolabs/go-talos-support/commit/69891cf046628969e651fc751e433aad86ec22c4) chore: remove containerd dependency
</p>
</details>

### Changes from siderolabs/image-factory
<details><summary>9 commits</summary>
<p>

* [`fe9134a`](https://github.com/siderolabs/image-factory/commit/fe9134a1bdf33543fe555466e6734f07356f6fc2) release(v0.5.0): prepare release
* [`7f09750`](https://github.com/siderolabs/image-factory/commit/7f0975004a30977841affba1c0c9ea3e79241eb7) feat: update to Talos 1.8
* [`b985abc`](https://github.com/siderolabs/image-factory/commit/b985abcc18ea555e6621735b0f5c85f44d7f5348) fix: cache generated system extension image correctly
* [`9687413`](https://github.com/siderolabs/image-factory/commit/9687413a9a85744c8d8254d6f8604c6a7854c244) fix: set SOURCE_DATA_EPOCH
* [`fef0833`](https://github.com/siderolabs/image-factory/commit/fef08331b7163a90e9063a21190597dc9c7ecb74) chore: add in new helios64 overlay
* [`03bd46f`](https://github.com/siderolabs/image-factory/commit/03bd46f7916a61184466c77f6586b587f39fb10a) feat: support inclusion on well-known UEFI SecureBoot certs
* [`608a6f0`](https://github.com/siderolabs/image-factory/commit/608a6f02ef685edc32c92fad5d111d18447eb91f) chore: alias nvidia extensions to lts versions
* [`8b4e0d9`](https://github.com/siderolabs/image-factory/commit/8b4e0d9e9819c7d4c8a533198bed167d56950035) chore: make metatadata pkg public
* [`7a4de58`](https://github.com/siderolabs/image-factory/commit/7a4de58b40f865aa0e1cac580836655a9c078df7) chore: build multi-arch image
</p>
</details>

### Dependency Changes

* **github.com/auth0/go-jwt-middleware/v2**            v2.2.1 -> v2.2.2
* **github.com/aws/aws-sdk-go-v2**                     v1.30.3 -> v1.30.4
* **github.com/aws/aws-sdk-go-v2/config**              v1.27.27 -> v1.27.31
* **github.com/aws/aws-sdk-go-v2/credentials**         v1.17.27 -> v1.17.30
* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.17.10 -> v1.17.16
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.58.3 -> v1.61.0
* **github.com/aws/smithy-go**                         v1.20.3 -> v1.20.4
* **github.com/containers/image/v5**                   v5.32.1 -> v5.32.2
* **github.com/cosi-project/runtime**                  v0.5.5 -> v0.6.1
* **github.com/cosi-project/state-etcd**               v0.3.0 -> v0.3.1
* **github.com/fsnotify/fsnotify**                     v1.7.0 **_new_**
* **github.com/grpc-ecosystem/grpc-gateway/v2**        v2.21.0 -> v2.22.0
* **github.com/prometheus/client_golang**              v1.19.1 -> v1.20.2
* **github.com/prometheus/common**                     v0.55.0 -> v0.57.0
* **github.com/siderolabs/discovery-service**          74bca2da5cc8 -> v1.0.3
* **github.com/siderolabs/go-api-signature**           v0.3.4 -> v0.3.6
* **github.com/siderolabs/go-debug**                   v0.3.0 -> v0.4.0
* **github.com/siderolabs/go-talos-support**           v0.1.0 -> v0.1.1
* **github.com/siderolabs/image-factory**              v0.4.2 -> v0.5.0
* **github.com/siderolabs/talos/pkg/machinery**        v1.8.0-alpha.1 -> 6f7c3a8e5c63
* **github.com/zitadel/oidc/v3**                       v3.27.0 -> v3.28.2
* **go.etcd.io/bbolt**                                 v1.3.10 -> v1.3.11
* **google.golang.org/grpc**                           v1.65.0 -> v1.66.0
* **sigs.k8s.io/controller-runtime**                   v0.18.5 -> v0.19.0

Previous release can be found at [v0.41.0](https://github.com/siderolabs/omni/releases/tag/v0.41.0)

## [Omni 0.41.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.41.0-beta.0) (2024-08-16)

Welcome to the v0.41.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Contributors

* Artem Chernyshev
* Andrey Smirnov
* Dmitriy Matrenichev
* Utku Ozdemir
* Brant Gurganus

### Changes
<details><summary>15 commits</summary>
<p>

* [`1cb1080`](https://github.com/siderolabs/omni/commit/1cb1080f0a12c3ad32f06016855c4247b3573943) feat: bump kube-service-exposer to v0.2.0
* [`dd510e9`](https://github.com/siderolabs/omni/commit/dd510e9b1256019e2d7abca1fc1e62425c99a924) fix: properly cleanup tearing down exposed services
* [`0bec3e4`](https://github.com/siderolabs/omni/commit/0bec3e4461989c0b1a1c6490c958064a8cd7cb2f) chore: bump deps
* [`6080c25`](https://github.com/siderolabs/omni/commit/6080c251c66c3bc9bd64def5aab5acc475a542c2) test: fix several flaky tests
* [`99f9317`](https://github.com/siderolabs/omni/commit/99f93179bd64cb6e97ea9e2ec287590fc98aa814) chore: implement audit log for several types
* [`ee73083`](https://github.com/siderolabs/omni/commit/ee7308376aa05411f11e739148989ac2b1403463) fix: properly remove `MachineSetNode` finalizer in the controller
* [`16b008b`](https://github.com/siderolabs/omni/commit/16b008beb03fc5afa5951418eb908a7c97c0c611) fix: increase LRU cache size for Talos and Kubernetes clients
* [`36c7b10`](https://github.com/siderolabs/omni/commit/36c7b107649b77a7a093f40af9e1aa506b89ba52) fix: skip reconciling redacted machine config on no input changes
* [`f0b44b1`](https://github.com/siderolabs/omni/commit/f0b44b1aa08421c2d33878d231ddf4df4c18a0b9) fix: add gRPC read buffer pool for etcd client
* [`b1fceea`](https://github.com/siderolabs/omni/commit/b1fceeac08fd78bf9758eca3d2d477459ae2c21d) fix: properly handle ExposedService resource finalizers
* [`5e35cbe`](https://github.com/siderolabs/omni/commit/5e35cbe57242bc2b16043f6c451d5157d5c9628b) fix: fix nil pointer dereference in workload proxy reconciler
* [`4746652`](https://github.com/siderolabs/omni/commit/4746652fcb57eb66ce282637a92907507f7b6419) docs: add a stringArray example for extensions
* [`7536191`](https://github.com/siderolabs/omni/commit/75361911114f7257a26fa4f87440b202a121cce6) chore: implement labels extractor controller for more efficient code
* [`7df58fe`](https://github.com/siderolabs/omni/commit/7df58fe686387dec0dcd31725fc71e2fe26b40a8) chore: add request label to the links created by the cloud provider
* [`d194d59`](https://github.com/siderolabs/omni/commit/d194d59be8c6a4a4f729eee356fb049dfc87c55c) feat: implement audit log
</p>
</details>

### Dependency Changes

* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.17.8 -> v1.17.10
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.58.2 -> v1.58.3
* **github.com/containers/image/v5**                   v5.31.1 -> v5.32.1
* **github.com/go-jose/go-jose/v4**                    v4.0.3 -> v4.0.4
* **github.com/google/go-containerregistry**           v0.20.1 -> v0.20.2
* **github.com/grpc-ecosystem/grpc-gateway/v2**        v2.20.0 -> v2.21.0
* **github.com/johannesboyne/gofakes3**                99de01ee122d -> edd0227ffc37
* **github.com/prometheus/common**                     v0.55.0 **_new_**
* **github.com/zitadel/oidc/v3**                       v3.26.0 -> v3.27.0
* **go.etcd.io/etcd/client/pkg/v3**                    v3.5.14 -> v3.5.15
* **go.etcd.io/etcd/client/v3**                        v3.5.14 -> v3.5.15
* **go.etcd.io/etcd/server/v3**                        v3.5.14 -> v3.5.15
* **golang.org/x/crypto**                              v0.25.0 -> v0.26.0
* **golang.org/x/net**                                 v0.27.0 -> v0.28.0
* **golang.org/x/sync**                                v0.7.0 -> v0.8.0
* **golang.org/x/tools**                               v0.22.0 -> v0.24.0
* **k8s.io/api**                                       v0.30.3 -> v0.31.0
* **k8s.io/apimachinery**                              v0.30.3 -> v0.31.0
* **k8s.io/client-go**                                 v0.30.3 -> v0.31.0
* **sigs.k8s.io/controller-runtime**                   v0.18.4 -> v0.18.5

Previous release can be found at [v0.40.0](https://github.com/siderolabs/omni/releases/tag/v0.40.0)

## [Omni 0.40.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.40.0-beta.0) (2024-07-26)

Welcome to the v0.40.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Support Bundle

Support downloading cluster support bundle from the UI.


### Show Total Items

Display total number of clusters and machines on the corresponding pages.
Also show some basic stats there: the number of clusters not ready and allocated/available machines.


### Contributors

* Artem Chernyshev
* Andrey Smirnov
* Dmitriy Matrenichev
* Utku Ozdemir
* Jubblin
* Maxime Brunet
* Sam
* Spencer Smith

### Changes
<details><summary>27 commits</summary>
<p>

* [`8ef319c`](https://github.com/siderolabs/omni/commit/8ef319cf608d05f89c57a4d1cb5cde442c452711) chore: bump kube-service-exposer version
* [`743e67f`](https://github.com/siderolabs/omni/commit/743e67f55ae4a7cfc4f4e32d12157b86da2761e9) chore: bump state-etcd module version
* [`6759925`](https://github.com/siderolabs/omni/commit/67599253375a9cc431ad3e1e1bc82e08cb28f853) chore: deprecate Talos 1.3
* [`5dd5259`](https://github.com/siderolabs/omni/commit/5dd52593ee80291407ab0ba158b8af0b04c433ef) chore: add rotating log for audit data
* [`6f6e1a6`](https://github.com/siderolabs/omni/commit/6f6e1a675191d0b7e5f94be3089b6a66200bb651) fix: do not allow deleting machine classes which are used anywhere
* [`aeb9322`](https://github.com/siderolabs/omni/commit/aeb9322cca0678ebbe5c5f16e22ac17ea107c3dc) fix: preserve labels on the `MachineClass` when editing it in the UI
* [`641328c`](https://github.com/siderolabs/omni/commit/641328c6d4230e8c6ed24ebbac0296b67897f433) feat: show machine/cluster stats and total counts
* [`ad74f85`](https://github.com/siderolabs/omni/commit/ad74f8527901d4825d14c72ce34ffc6ebe055f29) chore: bump deps
* [`19a72be`](https://github.com/siderolabs/omni/commit/19a72be550dcc3838619903290006fba516ffbd8) feat: add support bundle download button to cluster overview
* [`d76f8bd`](https://github.com/siderolabs/omni/commit/d76f8bdf593c281dfb01518cdbd2a15b04c8a80d) test: enable Talemu tests
* [`f67579f`](https://github.com/siderolabs/omni/commit/f67579f14039c462aea3902fb0a1406a88610ca5) fix: properly update `ClusterMachineIdentity` resource
* [`d8e804f`](https://github.com/siderolabs/omni/commit/d8e804fac5e7c2ffb5b4bc0ebcfa60928a90b267) fix: use proper finalizer chain in the `MachineClassStatusController`
* [`67bcc75`](https://github.com/siderolabs/omni/commit/67bcc75b83c79fecbf4648b6b87cd42e75c19440) feat: compute machineclass machine requirement (pressure)
* [`23fb0c1`](https://github.com/siderolabs/omni/commit/23fb0c1827ec4de38fa14c44e245a5c368fdf042) fix: make image pre pull failure block the kubernetes update
* [`b8db949`](https://github.com/siderolabs/omni/commit/b8db949ba3e0498348b7f3fd3fed7c4b893611a6) chore: bump dependencies
* [`e484bca`](https://github.com/siderolabs/omni/commit/e484bca4d81d238cddb131cfa637f16333b143b0) fix: improve resource deletion reliability, fix support bundle tests
* [`6f73f58`](https://github.com/siderolabs/omni/commit/6f73f58502dd786c882ac8a6e2d82f95ec59e239) fix: properly display icons on Safari browser
* [`276c3f4`](https://github.com/siderolabs/omni/commit/276c3f46b8e1491ee564ec1313611d93839dee81) fix: use proper check for the machine set teardown flow
* [`4cfc0e6`](https://github.com/siderolabs/omni/commit/4cfc0e6dd0bf45767bcbd17eb813544153d0beed) chore: rework auth.* keys, add `ctxstore` package
* [`76263e1`](https://github.com/siderolabs/omni/commit/76263e12a478b7d2214c6d074edfd4e13f805e05) fix: do not rely on `MachineStatus` updates when checking maintenance
* [`d271a8a`](https://github.com/siderolabs/omni/commit/d271a8afe93d521a8ae7a29d03a23a09a64bb576) fix: do not expect LB to be healthy when scaling down workers
* [`085bc2e`](https://github.com/siderolabs/omni/commit/085bc2e2780a444ce6c0354789776d5c4ba04d13) fix: add finalizer on `MachineSetNode` resource in the controller
* [`cbfb898`](https://github.com/siderolabs/omni/commit/cbfb898d7953b099d0eac619e0a146ee59f10bed) fix: add missing `return err` in the maintenance config drop migration
* [`a1a1d08`](https://github.com/siderolabs/omni/commit/a1a1d08f82f3246fd1b437c44abc5c3dc8293e8c) chore: bump deps
* [`4369338`](https://github.com/siderolabs/omni/commit/4369338e4912254b27618988951f57790e5bc156) fix: update Talos machine config schema to v1.7
* [`b93ac81`](https://github.com/siderolabs/omni/commit/b93ac8179f4d799156e53c55ff8f78d6e8fedf18) fix: provide cached access to the state via Omni API
* [`7602fde`](https://github.com/siderolabs/omni/commit/7602fde0df6bde02f2fc04655acc2ca0d35ba298) fix: update compose to fix missing information
</p>
</details>

### Changes from siderolabs/go-api-signature
<details><summary>1 commit</summary>
<p>

* [`4bf0f02`](https://github.com/siderolabs/go-api-signature/commit/4bf0f025dd94a8117997028d35c8b4497de497b4) fix: get rid of data race in the key sign interceptor
</p>
</details>

### Changes from siderolabs/go-kubernetes
<details><summary>2 commits</summary>
<p>

* [`ee8c6b8`](https://github.com/siderolabs/go-kubernetes/commit/ee8c6b8a5bb2c2c45e961d0f08faa5673905545c) fix: add one more removed feature gate for 1.31
* [`37dd61f`](https://github.com/siderolabs/go-kubernetes/commit/37dd61fad48b9f4bb6bce5a0a361a247228e86d2) feat: add support for Kubernetes 1.31
</p>
</details>

### Changes from siderolabs/grpc-proxy
<details><summary>5 commits</summary>
<p>

* [`ec3b59c`](https://github.com/siderolabs/grpc-proxy/commit/ec3b59c869000243e9794d162354c83738475a32) fix: address all gRPC deprecations
* [`02f82db`](https://github.com/siderolabs/grpc-proxy/commit/02f82db9c921eea3a48184bc4a4cf83a98b5b227) chore: rekres, bump deps
* [`62b29be`](https://github.com/siderolabs/grpc-proxy/commit/62b29beccb302d80e7a1b25acf86d755a769970b) chore: rekres, update dependencies
* [`2decdd1`](https://github.com/siderolabs/grpc-proxy/commit/2decdd1f77e64b61761e27c077ec3a420bfb2781) chore: add no-op github workflow
* [`77d7adc`](https://github.com/siderolabs/grpc-proxy/commit/77d7adc7105b6132b1352bf9e737bacc47fba5e5) chore: bump deps
</p>
</details>

### Changes from siderolabs/image-factory
<details><summary>4 commits</summary>
<p>

* [`e5686e2`](https://github.com/siderolabs/image-factory/commit/e5686e2596bd25f12cfbd3d386415108c2d91481) release(v0.4.2): prepare release
* [`1a2b64a`](https://github.com/siderolabs/image-factory/commit/1a2b64a87a1667eb92e6e11dfb8ec29b5ebd712d) feat: add Rock4 SE board to the mix of supported boards
* [`d07a780`](https://github.com/siderolabs/image-factory/commit/d07a78086d0ccf3a9e3c7ce4f2bd402953f1cf6b) fix: update wizard-versions.html
* [`f73a61e`](https://github.com/siderolabs/image-factory/commit/f73a61e28584219de5bee6d86ce53ba8ffa66643) fix: update misreported error
</p>
</details>

### Dependency Changes

* **github.com/adrg/xdg**                              v0.4.0 -> v0.5.0
* **github.com/aws/aws-sdk-go-v2**                     v1.30.0 -> v1.30.3
* **github.com/aws/aws-sdk-go-v2/config**              v1.27.21 -> v1.27.27
* **github.com/aws/aws-sdk-go-v2/credentials**         v1.17.21 -> v1.17.27
* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.17.1 -> v1.17.8
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.56.1 -> v1.58.2
* **github.com/aws/smithy-go**                         v1.20.2 -> v1.20.3
* **github.com/cosi-project/runtime**                  v0.5.0 -> v0.5.5
* **github.com/cosi-project/state-etcd**               v0.2.9 -> v0.3.0
* **github.com/go-jose/go-jose/v4**                    v4.0.2 -> v4.0.3
* **github.com/google/go-containerregistry**           v0.19.2 -> v0.20.1
* **github.com/siderolabs/go-api-signature**           v0.3.3 -> v0.3.4
* **github.com/siderolabs/go-kubernetes**              v0.2.9 -> v0.2.11
* **github.com/siderolabs/grpc-proxy**                 v0.4.0 -> v0.4.1
* **github.com/siderolabs/image-factory**              v0.4.1 -> v0.4.2
* **github.com/siderolabs/omni/client**                000000000000 -> v0.39.1
* **github.com/siderolabs/talos/pkg/machinery**        4feb94ca0997 -> v1.8.0-alpha.1
* **github.com/zitadel/oidc/v3**                       v3.25.1 -> v3.26.0
* **golang.org/x/crypto**                              v0.24.0 -> v0.25.0
* **golang.org/x/net**                                 v0.26.0 -> v0.27.0
* **google.golang.org/grpc**                           v1.64.0 -> v1.65.0
* **k8s.io/api**                                       v0.30.2 -> v0.30.3
* **k8s.io/client-go**                                 v0.30.2 -> v0.30.3

Previous release can be found at [v0.39.0](https://github.com/siderolabs/omni/releases/tag/v0.39.0)

## [Omni 0.40.0](https://github.com/siderolabs/omni/releases/tag/v0.40.0) (2024-07-26)

Welcome to the v0.40.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Support Bundle

Support downloading cluster support bundle from the UI.


### Show Total Items

Display total number of clusters and machines on the corresponding pages.
Also show some basic stats there: the number of clusters not ready and allocated/available machines.


### Contributors

* Artem Chernyshev
* Andrey Smirnov
* Dmitriy Matrenichev
* Utku Ozdemir
* Jubblin
* Maxime Brunet
* Sam
* Spencer Smith

### Changes
<details><summary>27 commits</summary>
<p>

* [`8ef319c`](https://github.com/siderolabs/omni/commit/8ef319cf608d05f89c57a4d1cb5cde442c452711) chore: bump kube-service-exposer version
* [`743e67f`](https://github.com/siderolabs/omni/commit/743e67f55ae4a7cfc4f4e32d12157b86da2761e9) chore: bump state-etcd module version
* [`6759925`](https://github.com/siderolabs/omni/commit/67599253375a9cc431ad3e1e1bc82e08cb28f853) chore: deprecate Talos 1.3
* [`5dd5259`](https://github.com/siderolabs/omni/commit/5dd52593ee80291407ab0ba158b8af0b04c433ef) chore: add rotating log for audit data
* [`6f6e1a6`](https://github.com/siderolabs/omni/commit/6f6e1a675191d0b7e5f94be3089b6a66200bb651) fix: do not allow deleting machine classes which are used anywhere
* [`aeb9322`](https://github.com/siderolabs/omni/commit/aeb9322cca0678ebbe5c5f16e22ac17ea107c3dc) fix: preserve labels on the `MachineClass` when editing it in the UI
* [`641328c`](https://github.com/siderolabs/omni/commit/641328c6d4230e8c6ed24ebbac0296b67897f433) feat: show machine/cluster stats and total counts
* [`ad74f85`](https://github.com/siderolabs/omni/commit/ad74f8527901d4825d14c72ce34ffc6ebe055f29) chore: bump deps
* [`19a72be`](https://github.com/siderolabs/omni/commit/19a72be550dcc3838619903290006fba516ffbd8) feat: add support bundle download button to cluster overview
* [`d76f8bd`](https://github.com/siderolabs/omni/commit/d76f8bdf593c281dfb01518cdbd2a15b04c8a80d) test: enable Talemu tests
* [`f67579f`](https://github.com/siderolabs/omni/commit/f67579f14039c462aea3902fb0a1406a88610ca5) fix: properly update `ClusterMachineIdentity` resource
* [`d8e804f`](https://github.com/siderolabs/omni/commit/d8e804fac5e7c2ffb5b4bc0ebcfa60928a90b267) fix: use proper finalizer chain in the `MachineClassStatusController`
* [`67bcc75`](https://github.com/siderolabs/omni/commit/67bcc75b83c79fecbf4648b6b87cd42e75c19440) feat: compute machineclass machine requirement (pressure)
* [`23fb0c1`](https://github.com/siderolabs/omni/commit/23fb0c1827ec4de38fa14c44e245a5c368fdf042) fix: make image pre pull failure block the kubernetes update
* [`b8db949`](https://github.com/siderolabs/omni/commit/b8db949ba3e0498348b7f3fd3fed7c4b893611a6) chore: bump dependencies
* [`e484bca`](https://github.com/siderolabs/omni/commit/e484bca4d81d238cddb131cfa637f16333b143b0) fix: improve resource deletion reliability, fix support bundle tests
* [`6f73f58`](https://github.com/siderolabs/omni/commit/6f73f58502dd786c882ac8a6e2d82f95ec59e239) fix: properly display icons on Safari browser
* [`276c3f4`](https://github.com/siderolabs/omni/commit/276c3f46b8e1491ee564ec1313611d93839dee81) fix: use proper check for the machine set teardown flow
* [`4cfc0e6`](https://github.com/siderolabs/omni/commit/4cfc0e6dd0bf45767bcbd17eb813544153d0beed) chore: rework auth.* keys, add `ctxstore` package
* [`76263e1`](https://github.com/siderolabs/omni/commit/76263e12a478b7d2214c6d074edfd4e13f805e05) fix: do not rely on `MachineStatus` updates when checking maintenance
* [`d271a8a`](https://github.com/siderolabs/omni/commit/d271a8afe93d521a8ae7a29d03a23a09a64bb576) fix: do not expect LB to be healthy when scaling down workers
* [`085bc2e`](https://github.com/siderolabs/omni/commit/085bc2e2780a444ce6c0354789776d5c4ba04d13) fix: add finalizer on `MachineSetNode` resource in the controller
* [`cbfb898`](https://github.com/siderolabs/omni/commit/cbfb898d7953b099d0eac619e0a146ee59f10bed) fix: add missing `return err` in the maintenance config drop migration
* [`a1a1d08`](https://github.com/siderolabs/omni/commit/a1a1d08f82f3246fd1b437c44abc5c3dc8293e8c) chore: bump deps
* [`4369338`](https://github.com/siderolabs/omni/commit/4369338e4912254b27618988951f57790e5bc156) fix: update Talos machine config schema to v1.7
* [`b93ac81`](https://github.com/siderolabs/omni/commit/b93ac8179f4d799156e53c55ff8f78d6e8fedf18) fix: provide cached access to the state via Omni API
* [`7602fde`](https://github.com/siderolabs/omni/commit/7602fde0df6bde02f2fc04655acc2ca0d35ba298) fix: update compose to fix missing information
</p>
</details>

### Changes from siderolabs/go-api-signature
<details><summary>1 commit</summary>
<p>

* [`4bf0f02`](https://github.com/siderolabs/go-api-signature/commit/4bf0f025dd94a8117997028d35c8b4497de497b4) fix: get rid of data race in the key sign interceptor
</p>
</details>

### Changes from siderolabs/go-kubernetes
<details><summary>2 commits</summary>
<p>

* [`ee8c6b8`](https://github.com/siderolabs/go-kubernetes/commit/ee8c6b8a5bb2c2c45e961d0f08faa5673905545c) fix: add one more removed feature gate for 1.31
* [`37dd61f`](https://github.com/siderolabs/go-kubernetes/commit/37dd61fad48b9f4bb6bce5a0a361a247228e86d2) feat: add support for Kubernetes 1.31
</p>
</details>

### Changes from siderolabs/grpc-proxy
<details><summary>5 commits</summary>
<p>

* [`ec3b59c`](https://github.com/siderolabs/grpc-proxy/commit/ec3b59c869000243e9794d162354c83738475a32) fix: address all gRPC deprecations
* [`02f82db`](https://github.com/siderolabs/grpc-proxy/commit/02f82db9c921eea3a48184bc4a4cf83a98b5b227) chore: rekres, bump deps
* [`62b29be`](https://github.com/siderolabs/grpc-proxy/commit/62b29beccb302d80e7a1b25acf86d755a769970b) chore: rekres, update dependencies
* [`2decdd1`](https://github.com/siderolabs/grpc-proxy/commit/2decdd1f77e64b61761e27c077ec3a420bfb2781) chore: add no-op github workflow
* [`77d7adc`](https://github.com/siderolabs/grpc-proxy/commit/77d7adc7105b6132b1352bf9e737bacc47fba5e5) chore: bump deps
</p>
</details>

### Changes from siderolabs/image-factory
<details><summary>4 commits</summary>
<p>

* [`e5686e2`](https://github.com/siderolabs/image-factory/commit/e5686e2596bd25f12cfbd3d386415108c2d91481) release(v0.4.2): prepare release
* [`1a2b64a`](https://github.com/siderolabs/image-factory/commit/1a2b64a87a1667eb92e6e11dfb8ec29b5ebd712d) feat: add Rock4 SE board to the mix of supported boards
* [`d07a780`](https://github.com/siderolabs/image-factory/commit/d07a78086d0ccf3a9e3c7ce4f2bd402953f1cf6b) fix: update wizard-versions.html
* [`f73a61e`](https://github.com/siderolabs/image-factory/commit/f73a61e28584219de5bee6d86ce53ba8ffa66643) fix: update misreported error
</p>
</details>

### Dependency Changes

* **github.com/adrg/xdg**                              v0.4.0 -> v0.5.0
* **github.com/aws/aws-sdk-go-v2**                     v1.30.0 -> v1.30.3
* **github.com/aws/aws-sdk-go-v2/config**              v1.27.21 -> v1.27.27
* **github.com/aws/aws-sdk-go-v2/credentials**         v1.17.21 -> v1.17.27
* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.17.1 -> v1.17.8
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.56.1 -> v1.58.2
* **github.com/aws/smithy-go**                         v1.20.2 -> v1.20.3
* **github.com/cosi-project/runtime**                  v0.5.0 -> v0.5.5
* **github.com/cosi-project/state-etcd**               v0.2.9 -> v0.3.0
* **github.com/go-jose/go-jose/v4**                    v4.0.2 -> v4.0.3
* **github.com/google/go-containerregistry**           v0.19.2 -> v0.20.1
* **github.com/siderolabs/go-api-signature**           v0.3.3 -> v0.3.4
* **github.com/siderolabs/go-kubernetes**              v0.2.9 -> v0.2.11
* **github.com/siderolabs/grpc-proxy**                 v0.4.0 -> v0.4.1
* **github.com/siderolabs/image-factory**              v0.4.1 -> v0.4.2
* **github.com/siderolabs/omni/client**                000000000000 -> v0.39.1
* **github.com/siderolabs/talos/pkg/machinery**        4feb94ca0997 -> v1.8.0-alpha.1
* **github.com/zitadel/oidc/v3**                       v3.25.1 -> v3.26.0
* **golang.org/x/crypto**                              v0.24.0 -> v0.25.0
* **golang.org/x/net**                                 v0.26.0 -> v0.27.0
* **google.golang.org/grpc**                           v1.64.0 -> v1.65.0
* **k8s.io/api**                                       v0.30.2 -> v0.30.3
* **k8s.io/client-go**                                 v0.30.2 -> v0.30.3

Previous release can be found at [v0.39.0](https://github.com/siderolabs/omni/releases/tag/v0.39.0)

## [Omni 0.39.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.39.0-beta.0) (2024-07-04)

Welcome to the v0.39.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Equinix Metal

Equinix metal is now available as a download/PXE option in the UI/CLI.


### Exposed Services Reliability

Exposed services proxy now provides more reliable connection to the underlying services for slower networks in the cluster.
Also if some nodes are down the proxy will evict them from the routing.


### Insecure Node Access

It is now possible to access nodes running in maintenance mode using `talosctl`.
Omni account wide `talosconfig` and at least `Operator` user role is required for that.
No `--insecure` flag should be set.


### Maintenance Talos Updates

Machine's Talos version can now be updated without adding the machine to a cluster.
Either `talosctl upgrade -n <uuid>` or the UI (Machines page) can be used for that.


### Contributors

* David Anderson
* Artem Chernyshev
* Brad Fitzpatrick
* Utku Ozdemir
* Andrey Smirnov
* Dmitriy Matrenichev
* AdamEr8
* Andrey Smirnov
* Andrey Smirnov
* Dominic Evans
* Khionu Sybiern
* Nathan Johnson
* Ryan Cox
* Vincent Batts
* ignoramous

### Changes
<details><summary>28 commits</summary>
<p>

* [`116ae97`](https://github.com/siderolabs/omni/commit/116ae972123cb24e4c491cf9fd2469342feb4f4d) release(v0.39.0-beta.0): prepare release
* [`26a61be`](https://github.com/siderolabs/omni/commit/26a61be1379a5a3e6e82a1542604dcd57b6bccee) fix: add resource caches for missing resource types
* [`5d953e4`](https://github.com/siderolabs/omni/commit/5d953e407bcc8c09519204b0a983ca4509931199) fix: do not re-create peer on the remote addr change
* [`08717d9`](https://github.com/siderolabs/omni/commit/08717d9e7a8138b16afe0dea36e2e283d35b3ef9) fix: get rid of config patches for the maintenance configs
* [`b910c20`](https://github.com/siderolabs/omni/commit/b910c20e20551f0dbce7479fbbfa3b763752703c) chore: add resource throughput metrics
* [`9671551`](https://github.com/siderolabs/omni/commit/9671551cb66165c994349ecdfa1e4fa5563fdf63) fix: use proper permissions for cluster taint resource
* [`09a8b36`](https://github.com/siderolabs/omni/commit/09a8b36b3b4d03d1b41f1ac13078630b357bdf65) fix: enable etcd client keep-alives by default
* [`5e46841`](https://github.com/siderolabs/omni/commit/5e468413a9dffcf3615c4b55dfbf5a133dc4e2f1) chore: add `go.work` file
* [`3810ccb`](https://github.com/siderolabs/omni/commit/3810ccb03f85f1728562c800692eb59da8010bae) fix: properly clean up stale Talos gRPC backends
* [`80d9277`](https://github.com/siderolabs/omni/commit/80d9277eea06978b4444e97a90339ae74bd6a685) feat: bump service exposer version to 1.1.3
* [`20b08ea`](https://github.com/siderolabs/omni/commit/20b08eaf3ac095522b3f2aa0f01c7e335caa56b9) fix: allow changing machine set node mgmt mode if it has no nodes
* [`c9b8b3f`](https://github.com/siderolabs/omni/commit/c9b8b3f6ccaa2f638ab9d1a63dc0b3aa9c3d8790) feat: add `Equinix metal` option in the download installation media
* [`5460134`](https://github.com/siderolabs/omni/commit/5460134f77466c8a75f8809af6dc18ee5b4589b0) chore: bump dependencies
* [`cd8bac4`](https://github.com/siderolabs/omni/commit/cd8bac4117b99665dbd3ff763ab921327bd0097f) feat: read real IP from the provision API gRPC requests
* [`b47acf2`](https://github.com/siderolabs/omni/commit/b47acf2e0f647d128581ee62b05e741ba44f4826) feat: support insecure access to the nodes running in maintenance
* [`2f05ab0`](https://github.com/siderolabs/omni/commit/2f05ab0cb41c046a3de0b7fe044d343ee69d132a) feat: show `N/∞` in the machine set if unlim allocation policy is used
* [`dc7c2b3`](https://github.com/siderolabs/omni/commit/dc7c2b3e3f89a9d76cdc5d23b4110948a15709dd) fix: detect the old vs. new URL format correctly on workload proxying
* [`e9bca13`](https://github.com/siderolabs/omni/commit/e9bca13f8f5eef06fe9200ee1ec8dbada14db3b8) feat: use tcp loadbalancer for exposed services
* [`17f7168`](https://github.com/siderolabs/omni/commit/17f71685c9257e91f68ac8a3ed485e104da54c8b) chore: bump COSI runtime version, use its task runner
* [`85424da`](https://github.com/siderolabs/omni/commit/85424da98eed1a8c49d39e7d51d57583d607e40b) fix: do better handling of small screens
* [`8b16da3`](https://github.com/siderolabs/omni/commit/8b16da39991b2128fbe988e606d966ff383d5a1e) fix: use proper `z-index` for the tooltip component
* [`92afd42`](https://github.com/siderolabs/omni/commit/92afd423ec37be35d8d91eb029cd9cf2cbac5985) chore: replace append with slices pkg functions
* [`ccc9d22`](https://github.com/siderolabs/omni/commit/ccc9d22bf5f49563b632951d510b49310a36d773) chore: update runtime and go-api-signature modules
* [`551286e`](https://github.com/siderolabs/omni/commit/551286e9bae4c7f6077145c35c3d65bf8bd24406) chore: bump go to 1.22.4, rekres
* [`271bb70`](https://github.com/siderolabs/omni/commit/271bb70b121d625767c98eb93e8093a9bd2f9fcb) chore: migrate to oidc v3
* [`6dcfd4c`](https://github.com/siderolabs/omni/commit/6dcfd4c9799d9ad6aa0d283f5f7302f45cb42943) feat: handle all goroutine panics gracefully
* [`c565666`](https://github.com/siderolabs/omni/commit/c565666113286ce6038aa6c59fd89483e8531c5a) feat: provide cleaner UI for the machine sets/machines lists
* [`e69df41`](https://github.com/siderolabs/omni/commit/e69df41eef2179f9fcc6f24b46d3832ccc271d03) fix: redo EtcdManualBackupShouldBeCreated
</p>
</details>

### Changes from siderolabs/go-api-signature
<details><summary>1 commit</summary>
<p>

* [`782aac0`](https://github.com/siderolabs/go-api-signature/commit/782aac0d69752fe7c6eba36bae8d1383ffdc0b04) chore: bump deps
</p>
</details>

### Changes from siderolabs/go-loadbalancer
<details><summary>1 commit</summary>
<p>

* [`0639758`](https://github.com/siderolabs/go-loadbalancer/commit/0639758a06785c0c8c65e18774b81d85ab40acdf) chore: bump deps
</p>
</details>

### Changes from siderolabs/siderolink
<details><summary>1 commit</summary>
<p>

* [`e76747b`](https://github.com/siderolabs/siderolink/commit/e76747ba523b336ab8b9143293c920ff64bc4f14) chore: migrate to rtnetlink/2
</p>
</details>

### Changes from siderolabs/tcpproxy
<details><summary>70 commits</summary>
<p>

* [`3d4e7b8`](https://github.com/siderolabs/tcpproxy/commit/3d4e7b860749152f0aefc53594f4c5fb9285c3f3) chore: rename to siderolabs/tcpproxy
* [`6f85d8e`](https://github.com/siderolabs/tcpproxy/commit/6f85d8e975e316d2e825db5c349c33eb8eb627d2) Implement correct half-close sequence for the connections.
* [`8bea9a4`](https://github.com/siderolabs/tcpproxy/commit/8bea9a449198dd0d0184ae0a6770d556dea5e0a0) Add support for TCP_USER_TIMEOUT setting
* [`91f8614`](https://github.com/siderolabs/tcpproxy/commit/91f861402626c6ba93eaa57ee257109c4f07bd00) remove old ACME tls-sni-01 stuff that LetsEncrypt removed March 2019
* [`74ca1dc`](https://github.com/siderolabs/tcpproxy/commit/74ca1dc5d55168d202044c415dcf2e08d80c3fdc) add Proxy.AddSNIRouteFunc to do lookups by SNI dynamically
* [`4e04b92`](https://github.com/siderolabs/tcpproxy/commit/4e04b92f29ea8f8a10d28528a47ecc0f93814473) gofmt for Go 1.19
* [`be3ee21`](https://github.com/siderolabs/tcpproxy/commit/be3ee21c9fa0283869843039aa136fbf9b948bf0) (doc): s/tlsproxy/tcpproxy
* [`2e577fe`](https://github.com/siderolabs/tcpproxy/commit/2e577fef49e2458ca3da06b30409df8f4eacb21e) Modified TestProxyPROXYOut to conform with the fixed version of PROXY protocol header format
* [`0f9bced`](https://github.com/siderolabs/tcpproxy/commit/0f9bceda1a83b4a17e52ba327a6fb2561285ee1a) Fixed HAProxy's PROXY protocol v1 Human-readable header format in DialProxy
* [`2825d76`](https://github.com/siderolabs/tcpproxy/commit/2825d768aaaef27e854631354415484406b1bc92) fix(test): update travis and e2e selfSignedCert fn
* [`b6bb9b5`](https://github.com/siderolabs/tcpproxy/commit/b6bb9b5b82524122bcf27291ede32d1517a14ab8) Update import path to inet.af/tcpproxy
* [`dfa16c6`](https://github.com/siderolabs/tcpproxy/commit/dfa16c61dad2b18a385dfb351adf71566720535b) tlsrouter/README: fix the go get url
* [`f5c09fb`](https://github.com/siderolabs/tcpproxy/commit/f5c09fbedceb69e4b238dec52cdf9f2fe9a815e2) Take advantage of Go 1.11's splice support, unwrap Conns in DialProxy.HandleConn
* [`7f81f77`](https://github.com/siderolabs/tcpproxy/commit/7f81f7701c9b584822030be9a3a701b125a56c91) Work around deadlock with Go tip (at Go rev f3f7bd5)
* [`7efa37f`](https://github.com/siderolabs/tcpproxy/commit/7efa37ff5079eba4a39ddda1b79f65fc81c759e3) Quiet log spam in test.
* [`dbc1514`](https://github.com/siderolabs/tcpproxy/commit/dbc151467a20b4513174bb3d6b1283e9419eb0f9) Adding the HostName field to the Conn struct (#18)
* [`2b928d9`](https://github.com/siderolabs/tcpproxy/commit/2b928d9b07d782cc1a94736979d012792810658f) Link to docs
* [`de1c7de`](https://github.com/siderolabs/tcpproxy/commit/de1c7ded2e6918c5b5b932682e0de144f4f1a31d) Add support for arbitrary matching against HTTP and SNI hostnames.
* [`c6a0996`](https://github.com/siderolabs/tcpproxy/commit/c6a0996ce0f3db7b5c3e16e04c9e664936077c97) Support configurable routing of ACME tls-sni-01 challenges.
* [`815c942`](https://github.com/siderolabs/tcpproxy/commit/815c9425f1ad46ffd3a3fb1bbefc05440072e4a4) Merge matcher and route into an interface that yields a Target.
* [`2065af4`](https://github.com/siderolabs/tcpproxy/commit/2065af4b1e2d181a987a23f64c66f43e474469ff) Support HAProxy's PROXY protocol v1 in DialProxy.
* [`e030359`](https://github.com/siderolabs/tcpproxy/commit/e03035937341374a9be6eb8459ffe4f23bacd185) Fix golint nits by adding docstrings and simplifying execution flow.
* [`6d97c2a`](https://github.com/siderolabs/tcpproxy/commit/6d97c2aa8ea9d9f5a35614d1f4a2a7d6be28ae9a) Correct the package building command, and only deploy for master branch commits.
* [`aa12504`](https://github.com/siderolabs/tcpproxy/commit/aa12504e4e35953c3281989f871e1293eb2114fe) Another attempt to fix Travis.
* [`f6af481`](https://github.com/siderolabs/tcpproxy/commit/f6af481b22698c9c27dd2f6af1881ea995c72046) Make Travis test all packages, and remove the go.universe.tf import path.
* [`d7e343e`](https://github.com/siderolabs/tcpproxy/commit/d7e343ee3d714651cbf09f4d77e56ad24f75eb33) Fix the godoc link to point to google/tcpproxy.
* [`bef9f6a`](https://github.com/siderolabs/tcpproxy/commit/bef9f6aa62487d4adc7d8ddf9e29b9f28810316f) Merge bradfitz's tcpproxy codebase with the software formerly known as tlsrouter.
* [`d86e96a`](https://github.com/siderolabs/tcpproxy/commit/d86e96a9d54bb62b297cf30dd2242b365fe33604) Move tlsrouter's readme to the command's directory.
* [`9e73877`](https://github.com/siderolabs/tcpproxy/commit/9e73877b6b356885077a1b9c0ba349ce33c61438) Switch license to Apache2, add Google copyright headers.
* [`cbf137d`](https://github.com/siderolabs/tcpproxy/commit/cbf137dac7b2c4aa2f45572c1214d07b30742241) Correct the travis build to kinda work.
* [`3eb49e9`](https://github.com/siderolabs/tcpproxy/commit/3eb49e9b3902de95b3c9f5729d51ca7f61f02e5a) Move tlsrouter to cmd/tlsrouter, in preparation for rewrite as a pkg.
* [`af97cdd`](https://github.com/siderolabs/tcpproxy/commit/af97cdd9d95a6cae6a52775ab8d5b3fc456a6817) Fix copy/paste-o in doc example.
* [`3273f40`](https://github.com/siderolabs/tcpproxy/commit/3273f401801fb301dffe0380ae573ee34a4f5c36) Add vendor warning
* [`e387889`](https://github.com/siderolabs/tcpproxy/commit/e3878897bde4f5d532f67738009cf1b9fcd2f408) Add TargetListener
* [`2eb0155`](https://github.com/siderolabs/tcpproxy/commit/2eb0155fac2d41b022bc0a430d13aa3b45825f1d) Start of tcpproxy. No Listener or reverse dialing yet.
* [`c58b44c`](https://github.com/siderolabs/tcpproxy/commit/c58b44c4fc69a3602d751d679c69c07e6bcbe24a) Make golint fail if lint errors are found, and fix said lint.
* [`4621df9`](https://github.com/siderolabs/tcpproxy/commit/4621df99bdd73dbb3995055b9b4f3f062300c892) Clean up the Travis build a bit more, moving more stuff to the deploy stage.
* [`96cc76f`](https://github.com/siderolabs/tcpproxy/commit/96cc76fdcd91148162fc3211dbfd486a86c1cb0f) Test Travis's new build stage support.
* [`bbbede8`](https://github.com/siderolabs/tcpproxy/commit/bbbede8f604a6555c951f5d584ddf4e98f26191a) Make travis fetch the test-only dependency.
* [`4b8641f`](https://github.com/siderolabs/tcpproxy/commit/4b8641f40e04705b8227f245be36457c05ccba2c) Add support for HAProxy's PROXY protocol.
* [`d23eadc`](https://github.com/siderolabs/tcpproxy/commit/d23eadc3a6c89bf5058db893acee26d5f1d7e350) Upload packages based on Go 1.8, not 1.7.
* [`7ef32e3`](https://github.com/siderolabs/tcpproxy/commit/7ef32e3c68ff50a2002528af7ff7676fb58be0a2) Add Go 1.8 to the build matrix.
* [`e07ecec`](https://github.com/siderolabs/tcpproxy/commit/e07ececb94dd7fe786db042337ad2dc0d5a448a6) typo
* [`aa3f9c9`](https://github.com/siderolabs/tcpproxy/commit/aa3f9c9ba10dc5b2d1b79d5de05ae6bf83483334) Remove debug print in acme code.
* [`6664640`](https://github.com/siderolabs/tcpproxy/commit/666464088dba67b6748beea064ae830f3e699d37) Stop testing against Go 1.6.
* [`728b8bc`](https://github.com/siderolabs/tcpproxy/commit/728b8bce14d8241b090ecf89c7f48224d5ba2c74) Add ACME routing support.
* [`a5c2ccd`](https://github.com/siderolabs/tcpproxy/commit/a5c2ccd532db7f26e6f6caff9570f126b9f58713) Use nogroup as the group, not nobody.
* [`a94dbd1`](https://github.com/siderolabs/tcpproxy/commit/a94dbd1d9e69346cbc08462da0b799f4d7d1d51f) Port extra error checking over from netboot.
* [`3cd4412`](https://github.com/siderolabs/tcpproxy/commit/3cd44123fb97589bbb7aa8b0743c124a8ce81c9b) Clean up travis config a bit, and add missing copyright notice.
* [`aded796`](https://github.com/siderolabs/tcpproxy/commit/aded79682ca01ac8c7fb17449d79f5274e727f2d) Add a deploy step to garbage-collect old packagecloud files.
* [`3e6354c`](https://github.com/siderolabs/tcpproxy/commit/3e6354c147b050cb9b008ae44d68fd1d3d385723) Random change to force travis rebuild on latest code.
* [`77fa998`](https://github.com/siderolabs/tcpproxy/commit/77fa9980b9f34a5dd58909748a7bf53d10333bec) Attempt to create a package with no version name.
* [`bfef4ba`](https://github.com/siderolabs/tcpproxy/commit/bfef4ba5a22a178fb4a64f0fe9d98fcfd53edee0) Revert to just debian/jessie. It's the same package anyway.
* [`173db90`](https://github.com/siderolabs/tcpproxy/commit/173db9074b8e6586588af6d63e4a0dabe8f48a73) Try the obvious way to specify a matrix of package tags.
* [`ea58780`](https://github.com/siderolabs/tcpproxy/commit/ea5878082eb53bfe1c26e76671e912079590e058) Limit the deploy to only the go 1.7 build.
* [`a2d0c96`](https://github.com/siderolabs/tcpproxy/commit/a2d0c96158d3810655fb71ead9187f1268541e3f) Skip cleanup so travis doesn't delete the freshly built .deb.
* [`73ee2e7`](https://github.com/siderolabs/tcpproxy/commit/73ee2e798a4464ed94b947b5a6b6a8425b37f99e) Attempt a packagecloud push.
* [`cbd4ea6`](https://github.com/siderolabs/tcpproxy/commit/cbd4ea6ea39c80d520d75e3e1cb140b55d6220fc) Attempt to build a debian package with FPM.
* [`4f5b46f`](https://github.com/siderolabs/tcpproxy/commit/4f5b46f61cba8359944015dfbcbce4b88cc0fd00) Add a systemd unit file to run tlsrouter.
* [`8cc8cac`](https://github.com/siderolabs/tcpproxy/commit/8cc8cac141994b55ac7f2b98ad363b2196d867f4) Document -hello-timeout in README.
* [`e0a0158`](https://github.com/siderolabs/tcpproxy/commit/e0a01587f5d3c412231f18012f3f55743c5aa885) Add slowloris protection, in the form of a ClientHello timeout.
* [`09cc4bb`](https://github.com/siderolabs/tcpproxy/commit/09cc4bb6199e7c8ef49d4c3f5e4077b49f892407) Remove support for SSL 3.0.
* [`c41a68d`](https://github.com/siderolabs/tcpproxy/commit/c41a68d73b757355dbd1f433fc4e2afe161c1f7b) Add tests for hostname matching, and make DNS matches match entire string.
* [`6546db4`](https://github.com/siderolabs/tcpproxy/commit/6546db44e46c75d1ec05fbd47f1396c49705c34d) Fix vet errors in Go 1.6.
* [`e34c2a6`](https://github.com/siderolabs/tcpproxy/commit/e34c2a61afa52bf8cc245c1ff75cca10b231050e) Add more words to README.
* [`b321571`](https://github.com/siderolabs/tcpproxy/commit/b321571464ebd231043ead1e15f23dba1c02970c) Add godoc comments to appease golint.
* [`55ba69d`](https://github.com/siderolabs/tcpproxy/commit/55ba69dad29c3f6b3aec89789fc8a773cd776b28) Add a Travis CI config.
* [`b8a3ed8`](https://github.com/siderolabs/tcpproxy/commit/b8a3ed89ade6a84297914e83559ff8cb1b7c5d33) Add DNS name support to config
* [`0a0a9f6`](https://github.com/siderolabs/tcpproxy/commit/0a0a9f658b3a5aabf24cc9c78f2ff0baef7d5622) Add licensing and contributing information for release.
* [`b1edd90`](https://github.com/siderolabs/tcpproxy/commit/b1edd90c0436159dcf4d3f794121633fb8ed9035) Initial commit.
</p>
</details>

### Dependency Changes

* **filippo.io/age**                                   6ad4560f4afc -> v1.2.0
* **github.com/aws/aws-sdk-go-v2**                     v1.27.0 -> v1.30.0
* **github.com/aws/aws-sdk-go-v2/config**              v1.27.16 -> v1.27.21
* **github.com/aws/aws-sdk-go-v2/credentials**         v1.17.16 -> v1.17.21
* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.16.21 -> v1.17.1
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.54.3 -> v1.56.1
* **github.com/containers/image/v5**                   v5.31.0 -> v5.31.1
* **github.com/cosi-project/runtime**                  v0.4.6 -> v0.5.0
* **github.com/go-jose/go-jose/v4**                    v4.0.2 **_new_**
* **github.com/google/go-containerregistry**           v0.19.1 -> v0.19.2
* **github.com/siderolabs/go-api-signature**           v0.3.2 -> v0.3.3
* **github.com/siderolabs/go-loadbalancer**            v0.3.3 -> v0.3.4
* **github.com/siderolabs/siderolink**                 v0.3.8 -> v0.3.9
* **github.com/siderolabs/tcpproxy**                   v0.1.0 **_new_**
* **github.com/spf13/cobra**                           v1.8.0 -> v1.8.1
* **github.com/zitadel/oidc/v3**                       v3.25.1 **_new_**
* **golang.org/x/crypto**                              v0.23.0 -> v0.24.0
* **golang.org/x/net**                                 v0.25.0 -> v0.26.0
* **golang.org/x/tools**                               v0.21.0 -> v0.22.0
* **google.golang.org/protobuf**                       v1.34.1 -> v1.34.2
* **k8s.io/api**                                       v0.30.1 -> v0.30.2
* **k8s.io/client-go**                                 v0.30.1 -> v0.30.2
* **k8s.io/klog/v2**                                   v2.120.1 -> v2.130.1
* **sigs.k8s.io/controller-runtime**                   v0.18.3 -> v0.18.4

Previous release can be found at [v0.38.0](https://github.com/siderolabs/omni/releases/tag/v0.38.0)

## [Omni 0.38.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.38.0-beta.0) (2024-06-18)

Welcome to the v0.38.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Break-Glass Configs

Omni now allows getting raw Talos and Kubernetes configs that can allow bypassing Omni when
accessing the cluster.

It has a couple of limitations:

- It is available only if is enabled for the account.
- Only `os:operator` role Talosconfig level is available.
- The cluster will be marked as tainted for the time being, which doesn't affect anything, but is
the signal that Omni no longer fully controls secrets rotation.


### Exposed Services DNS Name

Exposed services now use new naming schema, so it shouldn't be affected by slow DNS updates.

The domain name patter is now: `<hash>-<account>.proxy-us.siderolabs.io`.


### Kubeconfig Authcode-Keyboard

It is now possible to generate `kubeconfig` with `--grant-type=authcode-keyboard` and Omni
supports that mode.
This mode will print a URL and ask for a one time code instead of starting a local HTTP server on port `8000`.
Clicking the URL will open the same Omni page as usual, but will present you the one time code instead of doing redirect.

This mode is useful for remote machine `kubectl` execution and removes the need to set up ssh port-forwarding.


### Machine Logs

Machine logs are now stored using new persitent circular buffer library, which has better write efficiency.


### Pending Updates

Omni UI now shows pending config changes which are not applied due to locked machines in the cluster.


### Contributors

* Artem Chernyshev
* Utku Ozdemir
* Andrey Smirnov
* Dmitriy Matrenichev
* Andrey Smirnov
* Grzegorz Rozniecki

### Changes
<details><summary>30 commits</summary>
<p>

* [`4109996`](https://github.com/siderolabs/omni/commit/4109996e5639b9823d3d18b4e9f5b4bb8a323c8e) fix: make `MachineSetNode` controller select only connected machines
* [`2457897`](https://github.com/siderolabs/omni/commit/2457897e937beff1e84627722a7b865348953239) fix: use un-cached list in the `MachineSetNodeController`
* [`73529c2`](https://github.com/siderolabs/omni/commit/73529c2da88aa331de7298f40f08f5fbdbd4fa24) fix: display descriptions when show description checkbox is clicked
* [`6a59d63`](https://github.com/siderolabs/omni/commit/6a59d6388fd91caa53a370cbb4f8f3f2175a3156) fix: generate schematics with the extensions, meta and kernel args
* [`87a7750`](https://github.com/siderolabs/omni/commit/87a7750dfff13a6db6eaa29b9f5d6fc56dfeba5e) chore: add Akamai installation media
* [`fa64b46`](https://github.com/siderolabs/omni/commit/fa64b4633cf917353e120218d43cfbd1b78a0609) fix: skip invalid machines in `TalosUpgradeStatusController`
* [`22bb2cc`](https://github.com/siderolabs/omni/commit/22bb2cc7de67251702d429069fa97928e96ef8bf) fix: use proper types in the machine status and snapshot controllers
* [`a2b7b53`](https://github.com/siderolabs/omni/commit/a2b7b530c9ad0f18bbfced5508142925e9c5588e) feat: use the new domain scheme for exposed services
* [`4ecb175`](https://github.com/siderolabs/omni/commit/4ecb175b095bc6615a366c1d390ad58b7cec2384) fix: handle panics in Omni and Talos UI watches
* [`6286340`](https://github.com/siderolabs/omni/commit/6286340e38363aefadada987e9ac865fedab38d1) fix: properly delete the item from the cached items slice
* [`63ad5bd`](https://github.com/siderolabs/omni/commit/63ad5bd1ef28935caaf5187b417123f90ac3179d) feat: provide a way to getadmin `talosconfig` and `kubeconfig`
* [`fa21349`](https://github.com/siderolabs/omni/commit/fa21349f472b23dcd9c1f68be60057b5d5c9b5ea) fix: properly generate maintenance config patches
* [`2e64c31`](https://github.com/siderolabs/omni/commit/2e64c3152fd0d0275418ed32ecf5a9662755eab4) fix: ignore not found `ClusterMachine` in the migrations
* [`a2c3802`](https://github.com/siderolabs/omni/commit/a2c38022060cd379b0bb6344cf1bc5635a1d081c) fix: validate user email on creation
* [`73d0d3b`](https://github.com/siderolabs/omni/commit/73d0d3b09bfaf08f13382a9baf032cddd27c2f14) fix: properly detect `authcode-keyboard` oidc mode
* [`b7a0620`](https://github.com/siderolabs/omni/commit/b7a06208e9ae7476f49108b40a4e6f117304b731) feat: use circular buffer's new persistence option for machine logs
* [`7eec6b9`](https://github.com/siderolabs/omni/commit/7eec6b9e7a2b8a239242f47e31c2bc31f0e3acdf) chore: bump COSI runtime to 0.4.5
* [`4d23186`](https://github.com/siderolabs/omni/commit/4d231866542df1e5e6cf932312b33f58d615f07c) feat: show pending config updates due to locked machine
* [`f98cf51`](https://github.com/siderolabs/omni/commit/f98cf51a76797baff600cbcfbd25a28e7c2a6b7c) fix: ignore not found in the `MachineStatus` and `MachineStatusSnapshot`
* [`ce6e15a`](https://github.com/siderolabs/omni/commit/ce6e15a368696edf071598908f329e845d78292f) fix: proper time adjustment to fix flaky TestEtcdManualBackup
* [`27491ea`](https://github.com/siderolabs/omni/commit/27491ea85e726dc448f39fb27c6d17073d000bd7) chore: upgrade github.com/containers/image to v5
* [`3f75f91`](https://github.com/siderolabs/omni/commit/3f75f916087382ec8b102cc960f8e56c0876f200) fix: change Transport.Address field to Transport.Address method
* [`e12cfa8`](https://github.com/siderolabs/omni/commit/e12cfa8444e101f192d658e52e7e170b8fad8f31) feat: support authcode login in `kubectl oidc-login`
* [`2fcd0fd`](https://github.com/siderolabs/omni/commit/2fcd0fdac43914c4e1234b4c2615b29805c8bc35) fix: properly update the pulled images count if some images are skipped
* [`5a4251c`](https://github.com/siderolabs/omni/commit/5a4251c99285bb807b63034705143842d1923c83) test: fix a data race in `MachineStatusSnapshotController` unit tests
* [`0965091`](https://github.com/siderolabs/omni/commit/09650914b9b7729ff7810ec5a86179f791278694) test: fix flaky test in `ClusterMachineConfigStatus` unit tests
* [`b7d48aa`](https://github.com/siderolabs/omni/commit/b7d48aa61efe532f57e85455adfd70b6bb544a42) chore: small fixes
* [`a6c8b47`](https://github.com/siderolabs/omni/commit/a6c8b47442e225f0b4b85b33944bac37002e5897) fix: pass through the `talosctl -n` args if they cannot be resolved
* [`3bab8bf`](https://github.com/siderolabs/omni/commit/3bab8bf0891a3910582e3f431683ce3351e54bb5) chore: migrate to Vite and Bun to build the frontend
* [`37c1a97`](https://github.com/siderolabs/omni/commit/37c1a971e74cb3a6e4342487604876fd8a8f627f) fix: use proper routing on the config patch view and edit pages
</p>
</details>

### Changes from siderolabs/discovery-client
<details><summary>13 commits</summary>
<p>

* [`ca662d2`](https://github.com/siderolabs/discovery-client/commit/ca662d218418eb50eb22d84560c290bef4369702) feat: export default GRPC dial options for the client
* [`7a767fa`](https://github.com/siderolabs/discovery-client/commit/7a767fa89005209f5f39b2f5891ca7b169f52d89) chore: bump Go, deps and rekres
* [`f4095a1`](https://github.com/siderolabs/discovery-client/commit/f4095a109d3947d1a1f470446ef40e1b386aeaf1) chore: bump discovery API to v0.1.4
* [`fbb1cea`](https://github.com/siderolabs/discovery-client/commit/fbb1cea89609242e20f6cb35b4bfec12ade4144e) fix: keepalive interval calculation
* [`ff8f4be`](https://github.com/siderolabs/discovery-client/commit/ff8f4be618f077f91ce1f9b8240c050719623582) fix: enable gRPC keepalives
* [`9ba5f03`](https://github.com/siderolabs/discovery-client/commit/9ba5f033a47d41448153962c5fe22db2d9a8a00c) chore: app optional ControlPlane data
* [`269a832`](https://github.com/siderolabs/discovery-client/commit/269a832ce9e35d4edeeddba2a23cf5682a2ca425) chore: rekres, update discovery api
* [`a5c19c6`](https://github.com/siderolabs/discovery-client/commit/a5c19c65f4833a104ac68f35a3c0f8f37be8fe87) feat: provide public IP discovered from the server
* [`230f317`](https://github.com/siderolabs/discovery-client/commit/230f317a8e6e9542b82efcbac9f5cd7b9cff34b6) fix: reconnect the client on update failure
* [`ac5ab32`](https://github.com/siderolabs/discovery-client/commit/ac5ab32d1350332e837eea76f02a2225ce17c626) feat: support deleting an affiliate
* [`27a5bee`](https://github.com/siderolabs/discovery-client/commit/27a5beeccc45c82222fee5a70a2318b21cf87ac6) chore: rekres
* [`a9a5e9b`](https://github.com/siderolabs/discovery-client/commit/a9a5e9bfddaa670e0fb4f57510167d377cf09b07) feat: initial client code
* [`98eb999`](https://github.com/siderolabs/discovery-client/commit/98eb9999c0c76d2f93378108b7e22de6bcae6e81) chore: initial commit
</p>
</details>

### Dependency Changes

* **github.com/containers/image/v5**          v5.31.0 **_new_**
* **github.com/cosi-project/runtime**         15e9d678159d -> v0.4.6
* **github.com/siderolabs/discovery-client**  v0.1.9 **_new_**

Previous release can be found at [v0.37.0](https://github.com/siderolabs/omni/releases/tag/v0.37.0)

## [Omni 0.37.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.37.0-beta.0) (2024-06-04)

Welcome to the v0.37.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Node Overview Page

Node overview page now displays more information about the node.
That includes:

- Machine stage.
- Unmet health check conditions of the Talos `MachineStatus`.
- CPU, memory and secure boot information.
- The list of labels added to the machine.


### Patches UI

The UI now has the page that shows config patches define for a machine.
It includes both cluster level and account level machine patches.


### Secureboot Support Added

Omni now fully supports secureboot enabled machines.


### Service Events

Node overview page service list now displays information about each service events.
If a service fails to start it will be possible to see why in the UI.


### Contributors

* Artem Chernyshev
* Andrey Smirnov
* Utku Ozdemir
* Dmitriy Matrenichev
* Christian Hüning
* Mattias Cockburn
* Petr Krutov

### Changes
<details><summary>22 commits</summary>
<p>

* [`800762d`](https://github.com/siderolabs/omni/commit/800762dc150f11be3eb94d8382d1c47dc24b4cf8) chore: rewrite `MachineStatus` to use `QController`
* [`ed26122`](https://github.com/siderolabs/omni/commit/ed26122ce0cccd7812e35c351666ca845154dfb9) fix: implement the controller for handling machine status snapshot
* [`6aa2140`](https://github.com/siderolabs/omni/commit/6aa21409e5ed3f68758a7f03fa13cde76f0ff22f) feat: display more data on the node overview page
* [`5654a49`](https://github.com/siderolabs/omni/commit/5654a494063ec59c8f6e776d662022a0cefc53e8) chore: add renovate.json
* [`82abb2b`](https://github.com/siderolabs/omni/commit/82abb2ba536512473d2392bff602adc4cc5dbed8) chore: bump deps
* [`c635827`](https://github.com/siderolabs/omni/commit/c6358272732628b01e38efdccbcfd766205ed7da) test: do not use epoch millis in service account names
* [`22e3acf`](https://github.com/siderolabs/omni/commit/22e3acf2eaef19b28d0bed951790a08b545084ed) chore: bump default Talos version to 1.7.4
* [`a67d1fb`](https://github.com/siderolabs/omni/commit/a67d1fb30b3de8e0f25d740a28e1535295377420) fix: always generate siderolink connection config for all machines
* [`9bce82a`](https://github.com/siderolabs/omni/commit/9bce82ad769d0042993302bf08cebf6abcbe7d2c) fix: ignore MachineStatus events timestamps as they're not reliable
* [`ccca5b5`](https://github.com/siderolabs/omni/commit/ccca5b5f66461ce07b621a3366b676673f92f102) fix: bump siderolink module version
* [`f38b7e5`](https://github.com/siderolabs/omni/commit/f38b7e54a69f4a27d6897900f13e8169bbe1a484) feat: enable ALPN for machine API
* [`48cc03a`](https://github.com/siderolabs/omni/commit/48cc03a7c3650dd180225dfb2ec88c6770c91885) fix: retry affiliate deletes
* [`55afa59`](https://github.com/siderolabs/omni/commit/55afa59033898499d24180c2b1d9a1b568905e43) feat: add secure boot support
* [`0bd2a42`](https://github.com/siderolabs/omni/commit/0bd2a420e85901ba2e4d4137823bafc391896020) docs: fix a typo in the on-prem installation link
* [`247c165`](https://github.com/siderolabs/omni/commit/247c16550f6aa2ad7d5682c115862e67c979cfb3) fix: improve wording in authentication error messages
* [`e2f8407`](https://github.com/siderolabs/omni/commit/e2f8407cebe43ad14bc5d5b415a327e3c63286a5) chore: run rekres
* [`4a8ebbf`](https://github.com/siderolabs/omni/commit/4a8ebbf19f9ea0decb047aeaf656c2d9d6e58759) chore: enable codecov and rekres
* [`2f1ab0d`](https://github.com/siderolabs/omni/commit/2f1ab0df457c5119a2c1aa3b5d0e9a7aa90e675f) feat: show service events on the node overview page
* [`c68a836`](https://github.com/siderolabs/omni/commit/c68a8369045e99169208996f5dc7ed66a07a2791) fix: use proper name for fetching existing extension configuration
* [`4b747f0`](https://github.com/siderolabs/omni/commit/4b747f0380563a40ce9abfe435e573fde8c98184) feat: add dedicated patch pages for machines and cluster machines
* [`4bd0331`](https://github.com/siderolabs/omni/commit/4bd0331958088c16a1b5c2e7cc5c92368cf00e95) fix: get rid of duplicating label completion options
* [`631f5c5`](https://github.com/siderolabs/omni/commit/631f5c570cea9535933657e133c940d65bb29626) chore: always build frontend on BUILDPLATFORM
</p>
</details>

### Changes from siderolabs/go-circular
<details><summary>3 commits</summary>
<p>

* [`cbce5c3`](https://github.com/siderolabs/go-circular/commit/cbce5c3e47d1c6a26a588cbb6f77af2f9bc3e5b7) feat: add persistence support
* [`3c48c53`](https://github.com/siderolabs/go-circular/commit/3c48c53c1449b2b5e5ddde14e0351d93a351b021) feat: implement extra compressed chunks
* [`835f04c`](https://github.com/siderolabs/go-circular/commit/835f04c9ba6083ef451b5bbba748200202d1a0a9) chore: rekres, update dependencies
</p>
</details>

### Changes from siderolabs/go-tail
<details><summary>1 commit</summary>
<p>

* [`7cb7294`](https://github.com/siderolabs/go-tail/commit/7cb7294b8af33175bc463c84493776e6e4da9c4f) fix: remove unexpected short read error
</p>
</details>

### Changes from siderolabs/image-factory
<details><summary>10 commits</summary>
<p>

* [`819432c`](https://github.com/siderolabs/image-factory/commit/819432ca6d6247c2948929d666281136842b2594) release(v0.4.1): prepare release
* [`4f3206b`](https://github.com/siderolabs/image-factory/commit/4f3206bb2d402029b15930e3cb105485a2f5303e) release(v0.4.0): prepare release
* [`b0b6bff`](https://github.com/siderolabs/image-factory/commit/b0b6bffc36355b235cdee065a4bb3827cf27264e) feat: implement wizard-like UI for the Image Factory
* [`8ccd284`](https://github.com/siderolabs/image-factory/commit/8ccd284b885bd3246bc41b898c82eddd4aecd5ad) feat: allow key-based image verification as option
* [`4643056`](https://github.com/siderolabs/image-factory/commit/46430564a05d1430837acfd9e5d080c400e7456d) chore: rekres/update dependencies
* [`116721a`](https://github.com/siderolabs/image-factory/commit/116721a73640c80a78e88b59ce0b71e2c16bc2f3) fix: workaround extension name inconsistencies
* [`f5bc497`](https://github.com/siderolabs/image-factory/commit/f5bc4976c8cb068dfdf21f4cd15c9abd9e145628) release(v0.3.3): prepare release
* [`221b442`](https://github.com/siderolabs/image-factory/commit/221b44249f6c635a9e8cb8b7b941401aa50d4b75) feat: support zstd compression
* [`40a13c5`](https://github.com/siderolabs/image-factory/commit/40a13c5ce810feb65a3fbe8622ae0b32568ebe10) release(v0.3.2): prepare release
* [`2fe6825`](https://github.com/siderolabs/image-factory/commit/2fe682511c2be12486f5da8fc6612f5c3ed1ebf7) fix: generation of overlay installer images
</p>
</details>

### Changes from siderolabs/siderolink
<details><summary>1 commit</summary>
<p>

* [`3a587fc`](https://github.com/siderolabs/siderolink/commit/3a587fcf9dbb259e216495496a523faaea427d04) fix: do not ever skip updates which have remove flag
</p>
</details>

### Dependency Changes

* **github.com/auth0/go-jwt-middleware/v2**            v2.2.0 -> v2.2.1
* **github.com/aws/aws-sdk-go-v2**                     v1.26.1 -> v1.27.0
* **github.com/aws/aws-sdk-go-v2/config**              v1.27.10 -> v1.27.16
* **github.com/aws/aws-sdk-go-v2/credentials**         v1.17.10 -> v1.17.16
* **github.com/aws/aws-sdk-go-v2/feature/s3/manager**  v1.15.8 -> v1.16.21
* **github.com/aws/aws-sdk-go-v2/service/s3**          v1.47.6 -> v1.54.3
* **github.com/cosi-project/runtime**                  v0.4.2 -> v0.4.3
* **github.com/emicklei/dot**                          v1.6.1 -> v1.6.2
* **github.com/hashicorp/vault/api**                   v1.10.0 -> v1.14.0
* **github.com/hashicorp/vault/api/auth/kubernetes**   v0.5.0 -> v0.7.0
* **github.com/johannesboyne/gofakes3**                f005f5cc03aa -> 99de01ee122d
* **github.com/prometheus/client_golang**              v1.19.0 -> v1.19.1
* **github.com/siderolabs/go-circular**                v0.1.0 -> v0.2.0
* **github.com/siderolabs/go-tail**                    v0.1.0 -> v0.1.1
* **github.com/siderolabs/image-factory**              v0.3.1 -> v0.4.1
* **github.com/siderolabs/siderolink**                 v0.3.7 -> v0.3.8
* **github.com/siderolabs/talos/pkg/machinery**        v1.7.2 -> 4feb94ca0997
* **github.com/zitadel/logging**                       v0.5.0 -> v0.6.0
* **go.etcd.io/bbolt**                                 v1.3.9 -> v1.3.10
* **go.etcd.io/etcd/client/pkg/v3**                    v3.5.13 -> v3.5.14
* **go.etcd.io/etcd/client/v3**                        v3.5.13 -> v3.5.14
* **go.etcd.io/etcd/server/v3**                        v3.5.13 -> v3.5.14
* **golang.org/x/tools**                               v0.20.0 -> v0.21.0
* **google.golang.org/grpc**                           v1.63.2 -> v1.64.0
* **k8s.io/api**                                       v0.30.0-rc.1 -> v0.30.1
* **k8s.io/apimachinery**                              v0.30.0-rc.1 -> v0.30.1
* **k8s.io/client-go**                                 v0.30.0-rc.1 -> v0.30.1
* **sigs.k8s.io/controller-runtime**                   v0.16.3 -> v0.18.3

Previous release can be found at [v0.36.0](https://github.com/siderolabs/omni/releases/tag/v0.36.0)

## [Omni 0.36.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.36.0-beta.0) (2024-05-20)

Welcome to the v0.36.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Machine And Cluster Labels Completion

The UI of search inputs was reworked. Now Omni suggests autocompletion for all existing machine and cluster labels.
It also displays the labels as colored boxes in the input to better match with what's shown in the list.


### Machine Set Scaling Parallelism

It is now possible to adjust worker machine sets scaling and update strategies in the UI.


### `omnictl` Version Warnings

`omnictl` now warns that it has the different version from the backend.


### Contributors

* Artem Chernyshev
* Dmitriy Matrenichev

### Changes
<details><summary>13 commits</summary>
<p>

* [`6501134`](https://github.com/siderolabs/omni/commit/65011345032d35a92cafe4037d54687e0ac1e285) feat: implement labels completion for clusters and machines
* [`f0b9240`](https://github.com/siderolabs/omni/commit/f0b9240c7173ab333b8806f5136435aec4058e69) fix: add the label when clicking outside of the input
* [`859f04a`](https://github.com/siderolabs/omni/commit/859f04aecdfb042572d57f5961991c225310ba3c) feat: warn about using `omnictl` version different from the backend
* [`5397c70`](https://github.com/siderolabs/omni/commit/5397c7010768ac390f1882957b41910b10c0ebe9) chore: bump siderolink to 0.3.7
* [`15186b6`](https://github.com/siderolabs/omni/commit/15186b6ffe9c6760d83002e11cbd6b41d99b36ac) fix: machine class edit page
* [`a330167`](https://github.com/siderolabs/omni/commit/a330167c0513afa7715c7f28864414bd4f21bb38) fix: use proper help string for `omnictl download --talos-version` flag
* [`c1d38e6`](https://github.com/siderolabs/omni/commit/c1d38e613d15e942fe277d7e62aab75b5dd84a6e) fix: properly do rolling update on control plane nodes
* [`a0d02ea`](https://github.com/siderolabs/omni/commit/a0d02ea20b2ebcc6556e80afe04cb22392afe561) fix: do not block machine config updates if loadbalancer is down
* [`105fd8b`](https://github.com/siderolabs/omni/commit/105fd8b496d8b7bb7f8105eff65c1ce2c004e574) fix: do not try to audit machine which no longer has `MachineStatus`
* [`81f749f`](https://github.com/siderolabs/omni/commit/81f749f91a5380f9213548443aae69111a43e7c2) fix: do not fail schematic reconcile if initial talos version is empty
* [`7bd922a`](https://github.com/siderolabs/omni/commit/7bd922a6a87423c4a087408b338a002dd3f6b554) feat: implement the UI for adjusting machine sets update strategies
* [`0058c04`](https://github.com/siderolabs/omni/commit/0058c043d6e0e3a10884f87cddc31b46f1392dbd) fix: get all attribute values from SAML ACS when adding user labels
* [`7aabbb0`](https://github.com/siderolabs/omni/commit/7aabbb089152debe12acbf0b5ca52c6f4f349d29) fix: make search work on `NodeExtensions` page
</p>
</details>

### Changes from siderolabs/siderolink
<details><summary>2 commits</summary>
<p>

* [`be00ff5`](https://github.com/siderolabs/siderolink/commit/be00ff59bac50e0da4cd0747f8e5f30c7b029ded) chore: redo event filtering as a sequence of iterators
* [`a936b60`](https://github.com/siderolabs/siderolink/commit/a936b60645267d2e7320083b402df5ad19de76f5) chore: handle peer events in batches
</p>
</details>

### Dependency Changes

* **github.com/siderolabs/siderolink**  v0.3.5 -> v0.3.7
* **golang.org/x/crypto**               v0.22.0 -> v0.23.0
* **golang.org/x/net**                  v0.24.0 -> v0.25.0
* **golang.org/x/sync**                 v0.6.0 -> v0.7.0
* **golang.org/x/text**                 v0.14.0 -> v0.15.0
* **golang.org/x/tools**                v0.19.0 -> v0.20.0
* **google.golang.org/grpc**            v1.62.2 -> v1.63.2
* **google.golang.org/protobuf**        v1.33.0 -> v1.34.1

Previous release can be found at [v0.35.0](https://github.com/siderolabs/omni/releases/tag/v0.35.0)

## [Omni 0.35.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.35.0-beta.0) (2024-05-08)

Welcome to the v0.35.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Auth With Username/Password

Omni now shows the error about unverified Auth0 emails.


### Machine Extensions

It is now possible to see currently installed system extensions for each machine in the UI and change them there.
It is also possible to configure machines' system extensions during cluster creation and scaling.

Also Talos upgrades from 1.5.x -> 1.6.x+ will make Omni automatically pick up kernel modules which are no longer
included in Talos >= 1.6.x.


### Machine Join Configs

Partial config for joining Talos nodes running in maintenance mode can now be downloaded from the Omni UI.


### Machine Status

Talos machine status is now composed both from Talos events (push model).
And from Talos node `MachineStatus` resource (pull model).
This way even if the node gets disconnected from Omni for a long time, Omni won't lose any important events.


### Settings UI

Admin settings UI (backups and users) is now unified under the single page with tabs.


### Contributors

* Artem Chernyshev
* Utku Ozdemir
* Andrey Smirnov
* Simon-Boyer

### Changes
<details><summary>22 commits</summary>
<p>

* [`16108a9`](https://github.com/siderolabs/omni/commit/16108a9f22519577da838d60ccd238586335383f) feat: allow setting some url params for api endpoint
* [`041a436`](https://github.com/siderolabs/omni/commit/041a4364c132a59b479c342f03a0d89109eb9f51) feat: unify admin settings under `Settings` page
* [`987f8cd`](https://github.com/siderolabs/omni/commit/987f8cdbd450ec5a24ff1e686e67c5e94eeaf0f9) feat: improve auth flow when user email is not verified
* [`5b8c130`](https://github.com/siderolabs/omni/commit/5b8c13082ca43b7e8134008c5ffbc5fe0504450e) feat: imlpement the UI for configuring extensions during cluster create
* [`f6cd840`](https://github.com/siderolabs/omni/commit/f6cd840e0adf55b74edcba0b95a0e6f467e45229) feat: implement the page that shows list of extensions per node
* [`89fa1ad`](https://github.com/siderolabs/omni/commit/89fa1adccef10e8f4375130c61f6756bd4edfdbc) fix: make `MachineSetNodeController` handle machinesets without clusters
* [`fa3c9ff`](https://github.com/siderolabs/omni/commit/fa3c9ffeabd3d6ac502fdbe230adbf541555d84f) feat: automatically pick up extensions when upgrading Talos
* [`f40c552`](https://github.com/siderolabs/omni/commit/f40c55293d82de6665184161c30bd0e626d16974) chore: use new Auth0 app for CI
* [`23d5532`](https://github.com/siderolabs/omni/commit/23d55329eaa19f5b48506c26ffe48c55f83a0c7f) fix: invert the order of recent clusters
* [`baec123`](https://github.com/siderolabs/omni/commit/baec123131b6495f856ba24c3620750192f4adf8) fix: do not allow adding ISO, PXE nodes running different Talos version
* [`264fb35`](https://github.com/siderolabs/omni/commit/264fb352ae9c08112a81858592a642fa3d96a4d3) chore: bump `go-kubernetes` module
* [`2c42f5c`](https://github.com/siderolabs/omni/commit/2c42f5c059e4220da54ad4c3eb9e1d03dd687731) feat: add button to overview page to download partial machine config
* [`95197e2`](https://github.com/siderolabs/omni/commit/95197e2b077e79324256696b4449154d4533c392) feat: improve reliability of machine status snapshots
* [`ac4fcd8`](https://github.com/siderolabs/omni/commit/ac4fcd84008a0ab93595a364e9fdaddaf84e8a77) fix: drop outdated `SchematicConfigurationController` finalizer
* [`7953a49`](https://github.com/siderolabs/omni/commit/7953a49678f22bb118004a860daec887a6512410) fix: ignore unknown machine version on the cluster create page
* [`fbe196e`](https://github.com/siderolabs/omni/commit/fbe196e6e96bb9932c69c37f59de7f6ce9aa946f) test: use Talos nodes with partial config in integration tests
* [`4b50d7c`](https://github.com/siderolabs/omni/commit/4b50d7cdc9624c4fef0ab3d22f37b9ad1d75002c) test: fix flaky test by longer k8s node checks and retries
* [`a32cb8a`](https://github.com/siderolabs/omni/commit/a32cb8a1f837201d48800978e804d57420dfbccf) fix: start watch before delete in `omnictl delete`
* [`40033da`](https://github.com/siderolabs/omni/commit/40033da9982ab52f895590befd802b8e2c71f557) fix: remove MachineSetNodes after links removal
* [`29667ef`](https://github.com/siderolabs/omni/commit/29667ef428c620c1aa43d45137e8f0e91c211aad) fix: make cluster machine install disk selector pick correct disk
* [`18e41f8`](https://github.com/siderolabs/omni/commit/18e41f87ef2fc8fa3ee1e94b041b57c6e65fcfa2) fix: issue with etcd watch cancel
* [`7f58ea4`](https://github.com/siderolabs/omni/commit/7f58ea471370e231753c05595285edd8bee6df96) fix: allow adding machines to Omni at higher speed
</p>
</details>

### Changes from siderolabs/go-kubernetes
<details><summary>1 commit</summary>
<p>

* [`ddd4c69`](https://github.com/siderolabs/go-kubernetes/commit/ddd4c69a16f173e080f24aeabb6b472f42d140b6) feat: add support for Kubernetes 1.30
</p>
</details>

### Dependency Changes

* **github.com/aws/smithy-go**             v1.20.2 **_new_**
* **github.com/cosi-project/runtime**      v0.4.1 -> v0.4.2
* **github.com/cosi-project/state-etcd**   v0.2.8 -> v0.2.9
* **github.com/rs/xid**                    v1.5.0 **_new_**
* **github.com/siderolabs/go-kubernetes**  v0.2.8 -> v0.2.9
* **go.etcd.io/etcd/client/pkg/v3**        v3.5.12 -> v3.5.13
* **go.etcd.io/etcd/client/v3**            v3.5.12 -> v3.5.13
* **go.etcd.io/etcd/server/v3**            v3.5.12 -> v3.5.13

Previous release can be found at [v0.34.0](https://github.com/siderolabs/omni/releases/tag/v0.34.0)

## [Omni 0.34.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.34.0-beta.0) (2024-04-22)

Welcome to the v0.34.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Contributors

* Andrey Smirnov
* Andrey Smirnov
* Artem Chernyshev
* Utku Ozdemir
* Noel Georgi
* Andrew Rynhard
* Andrey Smirnov
* Artem Chernyshev
* Mattias Cockburn
* Dmitriy Matrenichev

### Changes
<details><summary>15 commits</summary>
<p>

* [`d79e863`](https://github.com/siderolabs/omni/commit/d79e8637a34abf7b5d509cd2f829f7388b42600b) test: get rid of upgrade test flakiness, fix cli tests
* [`6fff261`](https://github.com/siderolabs/omni/commit/6fff261a18258d4f692496ad14607f5cbcc8e37f) fix: implement the correct upgrade flow from 1.6.x to 1.7.x for SBC
* [`586d2d7`](https://github.com/siderolabs/omni/commit/586d2d7d7fa36e2b7786ef9986c27b590a9b3edc) feat: generate overlay info by extracting board kernel args
* [`4134d2c`](https://github.com/siderolabs/omni/commit/4134d2cffb8636f79a27a4c16225c0b6a5af9510) chore: use sops for secrets
* [`f2b975b`](https://github.com/siderolabs/omni/commit/f2b975bfcd15d5aff107f0ca36e1c88ef1c29e8e) feat: read overlays from the machine, preserve them during updates
* [`340d078`](https://github.com/siderolabs/omni/commit/340d078571897c2174c93151a357b204a708327e) fix: use correct labels struct in the download installation media cmd
* [`0d337c2`](https://github.com/siderolabs/omni/commit/0d337c2c8a032a4f4a93a1fc1590c9649c57710a) test: fix the flakiness in the resourcelogger test
* [`23dcf32`](https://github.com/siderolabs/omni/commit/23dcf32c1e252ee58387e066125bc64d38947b55) feat: implement kubernetes node audit controller
* [`e037975`](https://github.com/siderolabs/omni/commit/e0379754fd5c6b93d54a4ec23abbbea728b71fc3) chore: rekres & fix linting errors
* [`8aa6a6a`](https://github.com/siderolabs/omni/commit/8aa6a6af152fb02475d2704c63511eec771dd35d) fix: properly select schematics for machine set and machine levels
* [`09a7b12`](https://github.com/siderolabs/omni/commit/09a7b129ecbec15edde7ddd1808c3a020c013ee2) fix: skip empty config patches in `ClusterMachineConfigPatches`
* [`aa4d764`](https://github.com/siderolabs/omni/commit/aa4d76489e296858bfd2e2df538bc99d8b77e4a1) fix: always delete removed nodes from discovery service
* [`7486bb8`](https://github.com/siderolabs/omni/commit/7486bb8d20d42b6c2fddda9e641bca55601b1dd9) feat: support generating installation media with overlays for Talos 1.7+
* [`e580f14`](https://github.com/siderolabs/omni/commit/e580f14e8ec1808e32d7e8052d5f2e8a85a79cd2) test: fix assertion in maintenance config patch test
* [`bb0618f`](https://github.com/siderolabs/omni/commit/bb0618fd9eb25f7e71788a739509b564c21115cc) release(v0.33.0-beta.0): prepare release
</p>
</details>

### Changes from siderolabs/discovery-api
<details><summary>1 commit</summary>
<p>

* [`e1dc7bb`](https://github.com/siderolabs/discovery-api/commit/e1dc7bbd44f52e799fe65a6bd43a40973d611a3c) chore: rekres, update dependencies
</p>
</details>

### Changes from siderolabs/discovery-client
<details><summary>13 commits</summary>
<p>

* [`ca662d2`](https://github.com/siderolabs/discovery-client/commit/ca662d218418eb50eb22d84560c290bef4369702) feat: export default GRPC dial options for the client
* [`7a767fa`](https://github.com/siderolabs/discovery-client/commit/7a767fa89005209f5f39b2f5891ca7b169f52d89) chore: bump Go, deps and rekres
* [`f4095a1`](https://github.com/siderolabs/discovery-client/commit/f4095a109d3947d1a1f470446ef40e1b386aeaf1) chore: bump discovery API to v0.1.4
* [`fbb1cea`](https://github.com/siderolabs/discovery-client/commit/fbb1cea89609242e20f6cb35b4bfec12ade4144e) fix: keepalive interval calculation
* [`ff8f4be`](https://github.com/siderolabs/discovery-client/commit/ff8f4be618f077f91ce1f9b8240c050719623582) fix: enable gRPC keepalives
* [`9ba5f03`](https://github.com/siderolabs/discovery-client/commit/9ba5f033a47d41448153962c5fe22db2d9a8a00c) chore: app optional ControlPlane data
* [`269a832`](https://github.com/siderolabs/discovery-client/commit/269a832ce9e35d4edeeddba2a23cf5682a2ca425) chore: rekres, update discovery api
* [`a5c19c6`](https://github.com/siderolabs/discovery-client/commit/a5c19c65f4833a104ac68f35a3c0f8f37be8fe87) feat: provide public IP discovered from the server
* [`230f317`](https://github.com/siderolabs/discovery-client/commit/230f317a8e6e9542b82efcbac9f5cd7b9cff34b6) fix: reconnect the client on update failure
* [`ac5ab32`](https://github.com/siderolabs/discovery-client/commit/ac5ab32d1350332e837eea76f02a2225ce17c626) feat: support deleting an affiliate
* [`27a5bee`](https://github.com/siderolabs/discovery-client/commit/27a5beeccc45c82222fee5a70a2318b21cf87ac6) chore: rekres
* [`a9a5e9b`](https://github.com/siderolabs/discovery-client/commit/a9a5e9bfddaa670e0fb4f57510167d377cf09b07) feat: initial client code
* [`98eb999`](https://github.com/siderolabs/discovery-client/commit/98eb9999c0c76d2f93378108b7e22de6bcae6e81) chore: initial commit
</p>
</details>

### Changes from siderolabs/go-procfs
<details><summary>12 commits</summary>
<p>

* [`9f72b22`](https://github.com/siderolabs/go-procfs/commit/9f72b22602b5ea3af5949dbdaa4b48a7e65687bd) feat: support removing kernel args
* [`4b4a6ff`](https://github.com/siderolabs/go-procfs/commit/4b4a6ff4fad6aab3be895ef4c48c1c1e71817063) chore: rekres
* [`a062a4c`](https://github.com/siderolabs/go-procfs/commit/a062a4ca078a6b3b3f119edf86e5f80620e67a55) chore: rekres, rename
* [`8cbc42d`](https://github.com/siderolabs/go-procfs/commit/8cbc42d3dc246a693d9b307c5358f6f7f3cb60bc) feat: provide an option to overwrite some args in AppendAll
* [`24d06a9`](https://github.com/siderolabs/go-procfs/commit/24d06a955782ed7d468f5117e986ec632f316310) refactor: remove talos kernel default args
* [`a82654e`](https://github.com/siderolabs/go-procfs/commit/a82654edcec13531a3f6baf1d9c2933b074326cf) feat: implement SetAll method
* [`16ce2ef`](https://github.com/siderolabs/go-procfs/commit/16ce2ef52acd0f351c93365e5c9263af442bec12) fix: update cmdline.Set() to drop the value being overwritten
* [`5a9a4a7`](https://github.com/siderolabs/go-procfs/commit/5a9a4a75d559eab694afcdad2496d268473db432) feat: update kernel args for new KSPP requirements
* [`57c7311`](https://github.com/siderolabs/go-procfs/commit/57c7311fdd4524bc17f528486bf9b417536153c3) refactor: change directory layout
* [`a077c96`](https://github.com/siderolabs/go-procfs/commit/a077c96480d04ad432ce909295cfd969d8c4da7d) fix: fix go module name
* [`698666f`](https://github.com/siderolabs/go-procfs/commit/698666fd4540a0460b5141425d47df084f9a6e20) chore: move package to new repo
* [`dabb425`](https://github.com/siderolabs/go-procfs/commit/dabb42542312758dd0edc22ece49d8daa5476bbd) Initial commit
</p>
</details>

### Changes from siderolabs/image-factory
<details><summary>14 commits</summary>
<p>

* [`db55c07`](https://github.com/siderolabs/image-factory/commit/db55c07209bc4f1a1d9c4afe2f04ab2956b6fc92) release(v0.3.1): prepare release
* [`762cf2b`](https://github.com/siderolabs/image-factory/commit/762cf2b40c609b460ffe8c82be49c2aa75b781df) fix: generation of SecureBoot ISO
* [`ae1f0a3`](https://github.com/siderolabs/image-factory/commit/ae1f0a3c1b6e68bd6ef5a8ea852cb7c67a49c02c) fix: sort extensions in the UI schematic generator
* [`c2de13f`](https://github.com/siderolabs/image-factory/commit/c2de13f682b1a2add2983436698d12561a7f5bf9) release(v0.3.0): prepare release
* [`7062392`](https://github.com/siderolabs/image-factory/commit/70623924c4a872b6cf7cdf08221350263f93c123) chore: update Talos dependency to 1.7.0-beta.0
* [`78f8944`](https://github.com/siderolabs/image-factory/commit/78f8944cbb8e673e0726250308b72eaf562d6290) feat: add cert issuer regexp option
* [`c0981e8`](https://github.com/siderolabs/image-factory/commit/c0981e849d2146313dd179b9174b7686f5c27846) feat: add support for -insecure-schematic-service-repository flag
* [`5d779bb`](https://github.com/siderolabs/image-factory/commit/5d779bb38adcc2a9dcd526683d8ea77eb94b0388) chore: bump dependencies
* [`93eb7de`](https://github.com/siderolabs/image-factory/commit/93eb7de1f6432ac31d34f5cccbf9ff40587e65bc) feat: support overlay
* [`df3d211`](https://github.com/siderolabs/image-factory/commit/df3d2119e49a4c6e09c8a4261e1bd679ab408a23) release(v0.2.3): prepare release
* [`4ccf0e5`](https://github.com/siderolabs/image-factory/commit/4ccf0e5d7ed44e39d97ab45040cca6665618f4fa) fix: ignore missing DTB and other SBC artifacts
* [`c7dba02`](https://github.com/siderolabs/image-factory/commit/c7dba02d17b068e576de7c155d5a5e58fa156a76) chore: run tailwindcss before creating image
* [`81f2cb4`](https://github.com/siderolabs/image-factory/commit/81f2cb437f71e4cb2d92db71a6f2a2b7becb8b56) chore: bump dependencies, rekres
* [`07095cd`](https://github.com/siderolabs/image-factory/commit/07095cd4966ab8943d93490bd5a9bc5085bec2f8) chore: re-enable govulncheck
</p>
</details>

### Dependency Changes

* **github.com/aws/aws-sdk-go-v2**               v1.24.1 -> v1.26.1
* **github.com/aws/aws-sdk-go-v2/config**        v1.26.4 -> v1.27.10
* **github.com/aws/aws-sdk-go-v2/credentials**   v1.16.15 -> v1.17.10
* **github.com/google/go-containerregistry**     v0.18.0 -> v0.19.1
* **github.com/prometheus/client_golang**        v1.18.0 -> v1.19.0
* **github.com/siderolabs/discovery-api**        v0.1.3 -> v0.1.4
* **github.com/siderolabs/discovery-client**     v0.1.9 **_new_**
* **github.com/siderolabs/go-procfs**            v0.1.2 **_new_**
* **github.com/siderolabs/image-factory**        v0.2.2 -> v0.3.1
* **github.com/siderolabs/talos/pkg/machinery**  v1.7.0-beta.0 -> 3dd1f4e88c22
* **golang.org/x/crypto**                        v0.21.0 -> v0.22.0
* **golang.org/x/net**                           v0.23.0 -> v0.24.0
* **google.golang.org/grpc**                     v1.62.1 -> v1.62.2
* **k8s.io/api**                                 v0.29.2 -> v0.30.0-rc.1
* **k8s.io/apimachinery**                        v0.29.2 -> v0.30.0-rc.1
* **k8s.io/client-go**                           v0.29.2 -> v0.30.0-rc.1
* **k8s.io/klog/v2**                             v2.120.0 -> v2.120.1

Previous release can be found at [v0.33.0](https://github.com/siderolabs/omni/releases/tag/v0.33.0)

## [Omni 0.33.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.33.0-beta.0) (2024-04-12)

Welcome to the v0.33.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Extensions Support

It is now possible to change the list of installed extensions for the machines which are allocated into a cluster.
It can be done using cluster templates.
The extensions list can be defined for all machines of a cluster, machine set or for a particular machine.
Extensions update is done the same way as Talos upgrades.


### Machine Allocation Changes

From now on Omni doesn't allow adding machines to a cluster which has lower major or minor version of Talos.
Which means that adding a machine to a cluster which will lead to downgrade of Talos version is no longer possible.
It is done to avoid all kinds of weird issues which Talos downgrades might lead to.

### Contributors

* Artem Chernyshev
* Dmitriy Matrenichev
* Utku Ozdemir
* Andrey Smirnov
* Spencer Smith
* Justin Garrison
* Sherif Fanous

### Changes
<details><summary>15 commits</summary>
<p>

* [`592f916`](https://github.com/siderolabs/omni/commit/592f916346c9987c2b613a34196c3ad78dc44cae) feat: don't allow downgrades of the machines when adding to a cluster
* [`2e015a9`](https://github.com/siderolabs/omni/commit/2e015a994abe1e7d8237353028f3b9d7f5ae85ef) chore: support Auth0 client playing nicely with other OAuth2/OIDC providers
* [`de4c096`](https://github.com/siderolabs/omni/commit/de4c096a9b99c110565ce02d6cde16fc61f8c711) fix: ignore not existing cluster in `MachineSet` teardown flow
* [`d3e3eef`](https://github.com/siderolabs/omni/commit/d3e3eef0fabdd3685e32cd58293f4f9485c03cd4) chore: support WG over GRPC in Omni
* [`1cc5fb9`](https://github.com/siderolabs/omni/commit/1cc5fb91563752f3c58336eba3db6f66dbd0b92a) refactor: disable K8s stats for clusters with > 50 nodes
* [`1b64824`](https://github.com/siderolabs/omni/commit/1b648244051fe07a1275e41cf4b2c59bf76eba41) fix: add missing `region` input on the backups storage config page
* [`f70239c`](https://github.com/siderolabs/omni/commit/f70239c6397d41fb9968ced430a707a63ca82ff1) fix: ignore `modules.dep` virtual extension on schematic id calculation
* [`1196863`](https://github.com/siderolabs/omni/commit/11968634c0942a8e0c170848fb2d855d446d7db5) feat: forbid `*.acceptedCAs` fields in config patches
* [`4c179fa`](https://github.com/siderolabs/omni/commit/4c179fa0fe0a8f6b01495e0bbcc0c8cf177edb44) chore: bump Go to 1.22.2 and Talos machinery to `v1.7.0-beta.0`
* [`b171daa`](https://github.com/siderolabs/omni/commit/b171daad3fc9e9a17392e986c0d8bcd64fe8a61a) fix: properly render download installation media page in Safari
* [`7fb5d2b`](https://github.com/siderolabs/omni/commit/7fb5d2b20a9372e1a0906b9384696daf93a45c51) chore: add barebones compose file
* [`9d35dfe`](https://github.com/siderolabs/omni/commit/9d35dfeb712956c4b1bdbecaaa6beebd14ba1ff6) chore: bump net library to v0.23.0
* [`5dc2eaa`](https://github.com/siderolabs/omni/commit/5dc2eaa1024f0ea09a1a5571289ba2cbebd6f633) fix: prevent link and clustermachine deletion from getting stuck
* [`ae85293`](https://github.com/siderolabs/omni/commit/ae85293e1411d6844c1c48255915dba4095cb425) docs: add screenshot and install link
* [`2107c01`](https://github.com/siderolabs/omni/commit/2107c0195bead299f9f2a7f4c809802d92ce8c95) feat: support setting extensions list in the cluster template
</p>
</details>

### Changes from siderolabs/crypto
<details><summary>2 commits</summary>
<p>

* [`c240482`](https://github.com/siderolabs/crypto/commit/c2404820ab1c1346c76b5b0f9b7632ca9d51e547) feat: provide dynamic client CA matching
* [`2f4f911`](https://github.com/siderolabs/crypto/commit/2f4f911da321ade3cedacc3b6abfef5f119f7508) feat: add PEMEncodedCertificate wrapper
</p>
</details>

### Changes from siderolabs/siderolink
<details><summary>5 commits</summary>
<p>

* [`5422b1c`](https://github.com/siderolabs/siderolink/commit/5422b1c3d2e0ccc0bf5801e25130336c1fff0813) chore: quick fixes
* [`9300968`](https://github.com/siderolabs/siderolink/commit/930096812155cb460d7c99db47de39bea1418021) feat: move actual logic into the `agent` package
* [`8866351`](https://github.com/siderolabs/siderolink/commit/8866351abf8dc6120da3d984684855c94e43adf9) chore: implement WireGuard over GRPC
* [`7909156`](https://github.com/siderolabs/siderolink/commit/79091567e14526293eb19988fc2015a98c7b1898) chore: bump deps
* [`eb221dd`](https://github.com/siderolabs/siderolink/commit/eb221ddf88db7df35465db9bf1733b23580a6159) chore: bump deps
</p>
</details>

### Dependency Changes

* **github.com/cenkalti/backoff/v4**             v4.2.1 -> v4.3.0
* **github.com/cosi-project/runtime**            v0.4.0-alpha.9 -> v0.4.1
* **github.com/siderolabs/crypto**               v0.4.2 -> v0.4.4
* **github.com/siderolabs/siderolink**           v0.3.4 -> v0.3.5
* **github.com/siderolabs/talos/pkg/machinery**  v1.7.0-alpha.1 -> v1.7.0-beta.0
* **golang.org/x/crypto**                        v0.19.0 -> v0.21.0
* **golang.org/x/net**                           v0.21.0 -> v0.23.0
* **golang.org/x/tools**                         v0.16.1 -> v0.19.0
* **golang.zx2c4.com/wireguard**                 12269c276173 **_new_**

Previous release can be found at [v0.32.0](https://github.com/siderolabs/omni/releases/tag/v0.32.0)

## [Omni 0.20.0](https://github.com/siderolabs/omni/releases/tag/v0.20.0) (2023-10-17)

Welcome to the v0.20.0 release of Omni!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Kubeconfig Changes

Omni now generates Kubernetes configs without accessing Talos API.


###

Omni can now define SAML user roles depending on the SAML labels it gets from the SAML assertion.
Role is assigned only once on user creation.


### Contributors

* Andrey Smirnov
* Artem Chernyshev
* Utku Ozdemir

### Changes
<details><summary>19 commits</summary>
<p>

* [`992614d4`](https://github.com/siderolabs/omni/commit/992614d408f185692ddde2021682fdef68ebd5ba) chore: stop kubernetes status watchers for the offline cluster
* [`79868c27`](https://github.com/siderolabs/omni/commit/79868c279355bec3b36c194f97381d55e3b4a007) chore: optimize controller operations for disconnected machines
* [`8c2c39d3`](https://github.com/siderolabs/omni/commit/8c2c39d386cb8074db3c196b69b07fc80d30fdbe) fix: do not run loadbalancer for the unreachable clusters
* [`caf3d955`](https://github.com/siderolabs/omni/commit/caf3d955fe8e79e1a52ff463c3c6e0fa5e8420d8) test: set unique names for config patches
* [`f31373bd`](https://github.com/siderolabs/omni/commit/f31373bd317dcb1be407d3616669746f2ba79fee) feat: implement kubeconfig generation on Omni side
* [`681ffa3b`](https://github.com/siderolabs/omni/commit/681ffa3b1d8446f77618399353f8563d139d9f44) feat: allow defining SAML label mapping rules to Omni roles
* [`9d3f3b9e`](https://github.com/siderolabs/omni/commit/9d3f3b9e3ad0a56f666b50e2930610c0fa91f8eb) fix: rewrite the link counter handling
* [`9becbc78`](https://github.com/siderolabs/omni/commit/9becbc78f8ffc7065e2c8dcaa0386fa00a147c17) refactor: use COSI runtime with new controller runtime DB
* [`22235517`](https://github.com/siderolabs/omni/commit/22235517c04351136fc1a34977f394e4298cb25e) fix: gracefully handle links removal in the siderolink manager
* [`26ae4163`](https://github.com/siderolabs/omni/commit/26ae416378acfee1ae66561017cd809da6960f3c) refactor: lower the level of log storage logs
* [`01743ecd`](https://github.com/siderolabs/omni/commit/01743ecdde23583e9bc177e250122e6237996975) fix: rework the talos client and configuration generation
* [`f837129a`](https://github.com/siderolabs/omni/commit/f837129ae469d319b78e575f0e72e16f97f8e2fe) chore: bump Talos machinery to the latest main
* [`4a79387e`](https://github.com/siderolabs/omni/commit/4a79387e95f07e2f33df682904af00eb38bf1f5b) fix: update to Go 1.21.3
* [`3df360b8`](https://github.com/siderolabs/omni/commit/3df360b81196e939eb571e6ee262eb5bee715bd7) chore: log received interruption signals in Omni
* [`0a72c596`](https://github.com/siderolabs/omni/commit/0a72c5962a446f4ca1ea9399ff87aa750587bd07) chore: update state-etcd to v0.2.4
* [`4f2978d2`](https://github.com/siderolabs/omni/commit/4f2978d2cf051a2a83eca885193792a335d94fcc) test: override grpc call log level in authorization tests
* [`e21e39a8`](https://github.com/siderolabs/omni/commit/e21e39a83e80ef3226e73f85d0f36fcfd7e0b7b9) test: avoid excessive public key registration in integration tests
* [`78c5fbdf`](https://github.com/siderolabs/omni/commit/78c5fbdf290ffe8feb2d42a7a510089f327eaf60) ci: remove gh actions workflow
* [`e9f07068`](https://github.com/siderolabs/omni/commit/e9f07068ac0c08a7500677ec22d94afbb437e151) test: fix the assertion on cluster destroyed
</p>
</details>

### Dependency Changes

* **github.com/cosi-project/runtime**            v0.3.11 -> v0.3.13
* **github.com/cosi-project/state-etcd**         v0.2.3 -> v0.2.4
* **github.com/hashicorp/golang-lru/v2**         v2.0.7 **_new_**
* **github.com/siderolabs/talos/pkg/machinery**  c14a5d4f79a3 -> 7bb205ebe2ef
* **golang.org/x/crypto**                        v0.13.0 -> v0.14.0
* **golang.org/x/net**                           v0.15.0 -> v0.17.0
* **google.golang.org/grpc**                     v1.58.2 -> v1.58.3

Previous release can be found at [v0.19.0](https://github.com/siderolabs/omni/releases/tag/v0.19.0)

## [Omni 0.16.0](https://github.com/siderolabs/omni/releases/tag/v0.16.0) (2023-08-18)

Welcome to the v0.16.0 release of Omni!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Better Etcd Disaster Recovery

Omni now allows replacing control plane machines even if etcd is unhealthy.
And stil properly handles safety checks, not allowing to break etcd quorum,
allowing removing only unhealthy machines.

It also allows canceling machine destroy sequence if the machine destroyed
is not being torn down by the machine set controller.


### Machine Locking

Cluster templates now also support machine locking:

```yaml
kind: Machine
name: 430d882a-51a8-48b3-ab00-d4b5b0b5b0b0
locked: true
```


### Limit Workload Access

Workload proxy now takes into account the access to the cluster when allowing users to open the service endpoints.


### Contributors

* Utku Ozdemir
* Artem Chernyshev
* Dmitriy Matrenichev

### Changes
<details><summary>18 commits</summary>
<p>

* [`3d1c19a1`](https://github.com/siderolabs/omni/commit/3d1c19a11110e8b3e8a04543797b0cf32cd79a88) feat: allow replacing control plane machines if it doesn't break etcd
* [`12561b5b`](https://github.com/siderolabs/omni/commit/12561b5b82d4f150dac334e8b891b7f3dc7aeb54) fix: don't allow changing disk for the nodes that have Talos installed
* [`8e17f742`](https://github.com/siderolabs/omni/commit/8e17f742c17f0e47b79b1e2da03725a7205721f8) feat: allow canceling deletion of a machine set node
* [`1d8722aa`](https://github.com/siderolabs/omni/commit/1d8722aa353258c4e157e122ec9f11ddd8d1476b) chore: use 1.5.0 Talos in tests and enable disk encryption feature
* [`3318a443`](https://github.com/siderolabs/omni/commit/3318a443c4bd8c36914b1d0a768c59ff6651680b) feat: show `invalid-state` label if the machine is reachable but apid is not
* [`05f69c0d`](https://github.com/siderolabs/omni/commit/05f69c0d2a40837a466b4b2f607f7df5601207fa) feat: enable workload proxying by default
* [`dded4d81`](https://github.com/siderolabs/omni/commit/dded4d814633f42b2677ca392636f6103d042b55) fix: check for roles and ACLs on exposed service access
* [`d718f134`](https://github.com/siderolabs/omni/commit/d718f13432e92666b875b9134cf38c12dbbf01e5) chore: run auth tests in main integration test pipeline
* [`79516583`](https://github.com/siderolabs/omni/commit/79516583e7995b3832f0759dbdc855e230f42abe) chore: remove `toInputWeak` and add mutex.Empty
* [`6b2e09b7`](https://github.com/siderolabs/omni/commit/6b2e09b7e2229c25bd77863a7aa9aed367b20845) chore: bump Go to 1.21
* [`a5f4a9a4`](https://github.com/siderolabs/omni/commit/a5f4a9a493a7e359ec552f61dabb898ad7dd66e6) chore: cleanup `ConfigPatch` resources along with their owners
* [`a48efd7a`](https://github.com/siderolabs/omni/commit/a48efd7a9793c47a23b1b57fa23f90e7c72c7825) feat: add support for machine locking in cluster templates
* [`964eb23d`](https://github.com/siderolabs/omni/commit/964eb23dc862cabb9b3029515ff1a947ee6978f2) feat: block `os:admin` access to Talos API from workload clusters
* [`65bb6403`](https://github.com/siderolabs/omni/commit/65bb6403a529bf24aedf1175230fb98b4f63ab0d) refactor: simplify cleanup of exposed services
* [`ede70550`](https://github.com/siderolabs/omni/commit/ede70550561562b930196701f989a1821da5ebba) fix: destroy exposedservices when cluster is destroyed
* [`ddfd7657`](https://github.com/siderolabs/omni/commit/ddfd7657a3d871ab6da4b42adfb2b940e3781b36) fix: fix workload svc proxy feature visibility on frontend
* [`ead58143`](https://github.com/siderolabs/omni/commit/ead581434227dad691e0e8ac9a7f0926afb7d2b9) chore: update vault in docker-compose
* [`653824ca`](https://github.com/siderolabs/omni/commit/653824ca093ca053c129bd15780ae2e27e91ced0) chore: set default Talos version to v1.4.7
</p>
</details>

### Dependency Changes

* **github.com/emicklei/dot**                    v1.5.0 -> v1.6.0
* **github.com/siderolabs/talos/pkg/machinery**  80238a05a6f8 -> v1.5.0-beta.1
* **go.uber.org/zap**                            v1.24.0 -> v1.25.0
* **golang.org/x/net**                           v0.12.0 -> v0.14.0
* **golang.org/x/text**                          v0.11.0 -> v0.12.0
* **golang.org/x/tools**                         v0.11.0 -> v0.12.0

Previous release can be found at [v0.15.0](https://github.com/siderolabs/omni/releases/tag/v0.15.0)

## [Omni 0.11.0-alpha.0](https://github.com/siderolabs/omni/releases/tag/v0.11.0-alpha.0) (2023-06-08)

Welcome to the v0.11.0-alpha.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Support Full ACL Syntax

ACL now supports configuring additive perimissions to the base role, which includes:

- accessing the clusters - read-only, write access, separate Talos API access
- read-only access to machines
- write access to machines


### SAML support

Omni now supports SAML authentication.
SAML authentication is enabled by the following cmd line flags:

```
--auth-saml-enabled
--auth-saml-url <idp-url>
--auth-saml-metadata <idp-metadata>
--auth-saml-label-rules '{"Role": "role"}'
```

Omni metadata endpoint is `/saml/metadata`.

The users are automatically created on the first SAML login.
The first created user has Admin permissions, other have no permissions.
Permissions can be managed by ACLs or `Admin` can change user roles.


### Replace User Scopes with Roles

User management is now simplified. Instead of having scopes like `cluster:read`, `cluster:write`, etc,
the user is assigned one of 4 roles: `None`, `Reader`, `Operator`, `Admin`.

- `None` - gives no permissions.
- `Reader` - gives readonly permissions.
- `Operator` - allows managing clusters, machines, getting talosconfig, but doesn't allow editing users.
- `Amdin` - all permissions.

Fine grained access can still be managed by ACLs.


### Contributors

* Utku Ozdemir
* Artem Chernyshev
* Andrey Smirnov

### Changes
<details><summary>10 commits</summary>
<p>

* [`c7c93a1e`](https://github.com/siderolabs/omni/commit/c7c93a1e87f9d58ed8626aeed03bddb5b1d27a0d) fix: let the empty endpoints be recorded if there are no endpoints
* [`c28907e4`](https://github.com/siderolabs/omni/commit/c28907e4cafe2ed478ace42e5dfeeb0beaf52b95) feat: copy SAML attributes to `Identity` as labels
* [`a2f17a21`](https://github.com/siderolabs/omni/commit/a2f17a21f7321cc9dac4ea740afa8a14fa9d3e77) feat: implement full ACL syntax
* [`c0fa5d46`](https://github.com/siderolabs/omni/commit/c0fa5d46f705e73d904b78a5ce4e98dc0b787d0e) feat: add support for SAML authentication
* [`ad783798`](https://github.com/siderolabs/omni/commit/ad783798a081b4ff4ce667748db05688f91e0006) fix: replace `exponential-backoff` library with own implementation
* [`c8d7183a`](https://github.com/siderolabs/omni/commit/c8d7183a37e20c8fbc8c28b0d3cff684ae367199) feat: replace scopes with simplified roles
* [`08a048a9`](https://github.com/siderolabs/omni/commit/08a048a925fde227329b9b6696a03307ebc92256) feat: update default Talos to 1.4.5, Kubernetes to 1.27.2
* [`5278321b`](https://github.com/siderolabs/omni/commit/5278321bb38a1af39b4157c0a49157cc5d16fe73) fix: respect service account key env on omnictl download
* [`caac445d`](https://github.com/siderolabs/omni/commit/caac445d65289afb4276de0788c877640c74b4de) fix: don't show `OngoingTasks` until the UI is authorized
* [`dfca66df`](https://github.com/siderolabs/omni/commit/dfca66dfa297f5c35b5c7ea44e81dab20e74d8c2) fix: include node name in the cluster node search
</p>
</details>

### Changes from siderolabs/go-api-signature
<details><summary>1 commit</summary>
<p>

* [`a034e9f`](https://github.com/siderolabs/go-api-signature/commit/a034e9ff315ba4a56115acc7ad0fb99d0dc77800) feat: replace scopes with roles
</p>
</details>

### Dependency Changes

* **github.com/crewjam/saml**                    v0.4.13 **_new_**
* **github.com/siderolabs/go-api-signature**     v0.2.4 -> a034e9ff315b
* **github.com/siderolabs/talos/pkg/machinery**  v1.4.4 -> v1.4.5

Previous release can be found at [v0.10.0](https://github.com/siderolabs/omni/releases/tag/v0.10.0)

## [Omni 0.1.0-beta.2](https://github.com/siderolabs/omni/releases/tag/v0.1.0-beta.2) (2022-12-20)

Welcome to the v0.1.0-beta.2 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Contributors

* Andrey Smirnov
* Artem Chernyshev

### Changes
<details><summary>5 commits</summary>
<p>

* [`59df55f`](https://github.com/siderolabs/omni/commit/59df55f7b82c1e26564c77772eaa9755a2947b9e) fix: bring K8s info back to life on the node overview page
* [`2f54f91`](https://github.com/siderolabs/omni/commit/2f54f9136ecce5009dbca552c1ab01cfeb602679) chore: run etcd elections ("lock") to prevent concurrent Omni runs
* [`8beb051`](https://github.com/siderolabs/omni/commit/8beb05147a2746630c96fae2f62465dd3c95dd64) chore: update COSI to v0.3.0-alpha.2
* [`f14e358`](https://github.com/siderolabs/omni/commit/f14e3582ed8f63ba188d7b7e0b33fed0f27c5b8a) fix: better errors in `talosctl` via Omni
* [`f12a216`](https://github.com/siderolabs/omni/commit/f12a21673593ace90c51bdf087e2c7d084bb9c5f) fix: properly reset flush timeout in the Talos logs viewer
</p>
</details>

### Dependency Changes

* **github.com/cosi-project/runtime**  v0.3.0-alpha.1 -> v0.3.0-alpha.2

Previous release can be found at [v0.1.0-beta.1](https://github.com/siderolabs/omni/releases/tag/v0.1.0-beta.1)

## [Omni 0.1.0-beta.1](https://github.com/siderolabs/omni/releases/tag/v0.1.0-beta.1) (2022-12-16)

Welcome to the v0.1.0-beta.1 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Contributors

* Andrey Smirnov
* Andrey Smirnov
* Utku Ozdemir
* Alexey Palazhchenko
* Artem Chernyshev
* Dmitriy Matrenichev
* Andrew Rynhard
* Artem Chernyshev
* Noel Georgi
* Serge Logvinov

### Changes
<details><summary>20 commits</summary>
<p>

* [`9a7a9a0`](https://github.com/siderolabs/omni/commit/9a7a9a02f4853ecb9d99031c9e606eab1cb4f7ea) feat: add RedactedClusterMachineConfig resource
* [`c83cfe2`](https://github.com/siderolabs/omni/commit/c83cfe2f84c88bcff62a2c664b805bccbc996e56) feat: rework the cluster list view and cluster overview pages
* [`f65ce14`](https://github.com/siderolabs/omni/commit/f65ce14cea1d9169e39bf5845c6f8aff70a299f1) fix: ignore keys if the auth is disabled
* [`e9c3831`](https://github.com/siderolabs/omni/commit/e9c383161583c33ae32f229dbcbb8e546db507c2) fix: create config patch if it does not exist
* [`437d271`](https://github.com/siderolabs/omni/commit/437d2718c6d4bdcef2aac4d5b2c98cf2ce17e4e8) fix: support Kubernetes proxy OIDC flow when auth is disabled
* [`a47c211`](https://github.com/siderolabs/omni/commit/a47c211222881d474bb5774ff4969d4144652096) fix: read cluster reference from MachineStatus spec
* [`8091f16`](https://github.com/siderolabs/omni/commit/8091f16161c7779b2406062e37bdbd7f4ea7e68e) fix: set owner on MachineStatus migration
* [`e986e20`](https://github.com/siderolabs/omni/commit/e986e20d6f8899c3ff99e316c9111de56dd8b42c) fix: wrong yaml module version (should be v3)
* [`dbb3d48`](https://github.com/siderolabs/omni/commit/dbb3d48d0ff3305a9c0184b469d0100ada76db6a) fix: rollback etcd auto compaction retention
* [`047b89f`](https://github.com/siderolabs/omni/commit/047b89fd81eeffb60680a289cf1c5ba335afed40) refactor: move machine status labels into spec
* [`f990aea`](https://github.com/siderolabs/omni/commit/f990aea20db95502aa0013889168897d34161a98) feat: do not allow setting config patch fields which are owned by Omni
* [`7d9258f`](https://github.com/siderolabs/omni/commit/7d9258ff3b48582111487e114b7fffe098d38464) fix: fix incorrect yaml multiline string decoding in ClusterMachineSpec
* [`0b5b095`](https://github.com/siderolabs/omni/commit/0b5b0959c80beccc5eb5d062cf5158f8577edc0d) fix: prevent etcd audit from removing valid members
* [`82fe21b`](https://github.com/siderolabs/omni/commit/82fe21be7169a29f70b8c3425716082696dc770e) fix: label generated patches with `system-patch` label
* [`4c2ce26`](https://github.com/siderolabs/omni/commit/4c2ce26a28b57911b0489de6d7ba99ab8ebdbb77) fix: enhance watch to accept a single `Ref` value
* [`ef78843`](https://github.com/siderolabs/omni/commit/ef788432b6a3f3c2a40875fef65650e177a1adcd) feat: implement `Machine` level config patch editor
* [`8144d44`](https://github.com/siderolabs/omni/commit/8144d44f9f0974a94cdab257c5803367581d4db3) fix: encode image download URL when signing & slugify file names
* [`75ea9e6`](https://github.com/siderolabs/omni/commit/75ea9e6b60134981f3776f3b250678113f19dc63) refactor: rewrite generic ClusterMachineStatusController
* [`5dba725`](https://github.com/siderolabs/omni/commit/5dba725ed040cb0cde35ec9295c6e99db4cb9d6a) feat: add ability to download admin talosconfig in debug mode
* [`5baa939`](https://github.com/siderolabs/omni/commit/5baa939b8db3d3069960ff896ac77349ba63f172) refactor: `kubernetes.Runtime` to cache clients and configs
</p>
</details>

### Changes from siderolabs/crypto
<details><summary>28 commits</summary>
<p>

* [`c03ff58`](https://github.com/siderolabs/crypto/commit/c03ff58af5051acb9b56e08377200324a3ea1d5e) feat: add a way to represent redacted x509 private keys
* [`c3225ee`](https://github.com/siderolabs/crypto/commit/c3225eee603a8d1218c67e1bfe33ddde7953ed74) feat: allow CSR template subject field to be overridden
* [`8570669`](https://github.com/siderolabs/crypto/commit/85706698dac8cddd0e9f41006bed059347d2ea26) chore: rename to siderolabs/crypto
* [`e9df1b8`](https://github.com/siderolabs/crypto/commit/e9df1b8ca74c6efdc7f72191e5d2613830162fd5) feat: add support for generating keys from RSA-SHA256 CAs
* [`510b0d2`](https://github.com/siderolabs/crypto/commit/510b0d2753a89170d0c0f60e052a66484997a5b2) chore: add json tags
* [`6fa2d93`](https://github.com/siderolabs/crypto/commit/6fa2d93d0382299d5471e0de8e831c923398aaa8) fix: deepcopy nil fields as `nil`
* [`9a63cba`](https://github.com/siderolabs/crypto/commit/9a63cba8dabd278f3080fa8c160613efc48c43f8) fix: add back support for generating ECDSA keys with P-256 and SHA512
* [`893bc66`](https://github.com/siderolabs/crypto/commit/893bc66e4716a4cb7d1d5e66b5660ffc01f22823) fix: use SHA256 for ECDSA-P256
* [`deec8d4`](https://github.com/siderolabs/crypto/commit/deec8d47700e10e3ea813bdce01377bd93c83367) chore: implement DeepCopy methods for PEMEncoded* types
* [`d3cb772`](https://github.com/siderolabs/crypto/commit/d3cb77220384b3a3119a6f3ddb1340bbc811f1d1) feat: make possible to change KeyUsage
* [`6bc5bb5`](https://github.com/siderolabs/crypto/commit/6bc5bb50c52767296a1b1cab6580e3fcf1358f34) chore: remove unused argument
* [`cd18ef6`](https://github.com/siderolabs/crypto/commit/cd18ef62eb9f65d8b6730a2eb73e47e629949e1b) feat: add support for several organizations
* [`97c888b`](https://github.com/siderolabs/crypto/commit/97c888b3924dd5ac70b8d30dd66b4370b5ab1edc) chore: add options to CSR
* [`7776057`](https://github.com/siderolabs/crypto/commit/7776057f5086157873f62f6a21ec23fa9fd86e05) chore: fix typos
* [`80df078`](https://github.com/siderolabs/crypto/commit/80df078327030af7e822668405bb4853c512bd7c) chore: remove named result parameters
* [`15bdd28`](https://github.com/siderolabs/crypto/commit/15bdd282b74ac406ab243853c1b50338a1bc29d0) chore: minor updates
* [`4f80b97`](https://github.com/siderolabs/crypto/commit/4f80b976b640d773fb025d981bf85bcc8190815b) fix: verify CSR signature before issuing a certificate
* [`39584f1`](https://github.com/siderolabs/crypto/commit/39584f1b6e54e9966db1f16369092b2215707134) feat: support for key/certificate types RSA, Ed25519, ECDSA
* [`cf75519`](https://github.com/siderolabs/crypto/commit/cf75519cab82bd1b128ae9b45107c6bb422bd96a) fix: function NewKeyPair should create certificate with proper subject
* [`751c95a`](https://github.com/siderolabs/crypto/commit/751c95aa9434832a74deb6884cff7c5fd785db0b) feat: add 'PEMEncodedKey' which allows to transport keys in YAML
* [`562c3b6`](https://github.com/siderolabs/crypto/commit/562c3b66f89866746c0ba47927c55f41afed0f7f) feat: add support for public RSA key in RSAKey
* [`bda0e9c`](https://github.com/siderolabs/crypto/commit/bda0e9c24e80c658333822e2002e0bc671ac53a3) feat: enable more conversions between encoded and raw versions
* [`e0dd56a`](https://github.com/siderolabs/crypto/commit/e0dd56ac47456f85c0b247999afa93fb87ebc78b) feat: add NotBefore option for x509 cert creation
* [`12a4897`](https://github.com/siderolabs/crypto/commit/12a489768a6bb2c13e16e54617139c980f99a658) feat: add support for SPKI fingerprint generation and matching
* [`d0c3eef`](https://github.com/siderolabs/crypto/commit/d0c3eef149ec9b713e7eca8c35a6214bd0a64bc4) fix: implement NewKeyPair
* [`196679e`](https://github.com/siderolabs/crypto/commit/196679e9ec77cb709db54879ddeddd4eaafaea01) feat: move `pkg/grpc/tls` from `github.com/talos-systems/talos` as `./tls`
* [`1ff6242`](https://github.com/siderolabs/crypto/commit/1ff6242c91bb298ceeb4acd65685cba952fe4178) chore: initial version as imported from talos-systems/talos
* [`835063e`](https://github.com/siderolabs/crypto/commit/835063e055b28a525038b826a6d80cbe76402414) chore: initial commit
</p>
</details>

### Changes from siderolabs/gen
<details><summary>1 commit</summary>
<p>

* [`8e89b1e`](https://github.com/siderolabs/gen/commit/8e89b1ede9f35ff4c18a41ee44a69259181c892b) feat: add GetOrCreate and GetOrCall methods
</p>
</details>

### Dependency Changes

* **github.com/cosi-project/runtime**            v0.2.0 -> v0.3.0-alpha.1
* **github.com/grpc-ecosystem/grpc-gateway/v2**  v2.13.0 -> v2.14.0
* **github.com/siderolabs/crypto**               c03ff58af505 **_new_**
* **github.com/siderolabs/gen**                  v0.4.1 -> v0.4.2
* **github.com/siderolabs/talos/pkg/machinery**  v1.3.0-beta.0 -> 873bd3807c0f
* **go.uber.org/zap**                            v1.23.0 -> v1.24.0
* **golang.org/x/net**                           v0.2.0 -> v0.4.0
* **golang.org/x/text**                          v0.4.0 -> v0.5.0

Previous release can be found at [v0.1.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.1.0-beta.0)

## [Omni 0.1.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.1.0-beta.0) (2022-12-02)

Welcome to the v0.1.0-beta.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Contributors

* Artem Chernyshev
* Andrey Smirnov
* Artem Chernyshev
* Dmitriy Matrenichev
* Utku Ozdemir
* Philipp Sauter
* evgeniybryzh
* Noel Georgi
* Andrew Rynhard
* Tim Jones
* Andrew Rynhard
* Gerard de Leeuw
* Steve Francis
* Volodymyr Mazurets

### Changes
<details><summary>405 commits</summary>
<p>

* [`e096c88`](https://github.com/siderolabs/omni/commit/e096c887604399028a559e33da13653c1f54965d) chore: add resource operation metrics
* [`741e820`](https://github.com/siderolabs/omni/commit/741e8202c5aecfe171082c38e2c55e0184e9c80c) feat: implement config patch creation UI
* [`5def267`](https://github.com/siderolabs/omni/commit/5def26706fa21df7748801cbdab5c6e81543174f) fix: attempt to clean up docker container better
* [`876ff5e`](https://github.com/siderolabs/omni/commit/876ff5ee44d4193c52e4daeec776ad50b69664f9) feat: update COSI and state-etcd to 0.2.0
* [`3df410d`](https://github.com/siderolabs/omni/commit/3df410d964fc66b2d4ad8c7db0459108d16adde0) test: refactor and update config patch integration tests
* [`5eea9e5`](https://github.com/siderolabs/omni/commit/5eea9e50b47a6df324f2fd5564aa9010b56e16e0) feat: add TLS support to siderolink API
* [`36394ea`](https://github.com/siderolabs/omni/commit/36394ea242f9af4d9c17f90ec143b0356fa9e671) refactor: simplify the resource leak fix
* [`e5b962b`](https://github.com/siderolabs/omni/commit/e5b962b66f158fd31b74dc6b97f524c168b4fad1) chore: update dev environment
* [`39bf206`](https://github.com/siderolabs/omni/commit/39bf206eec29262b1c15ed557f7f24e029c61206) fix: save user picture and fullname in the local storage
* [`f1611c1`](https://github.com/siderolabs/omni/commit/f1611c10d26b937b5bae69a1b9eda67d2bc5e137) feat: add machine level config patch support
* [`f2e6cf5`](https://github.com/siderolabs/omni/commit/f2e6cf5cddb47aaa290e7db1a037f2155fcd60d2) fix: remove several resource/goroutine leaks
* [`fc37af3`](https://github.com/siderolabs/omni/commit/fc37af36d87e01c3e9f349f206711f154740e0b4) feat: allow destroying config patches in the UI
* [`3154d59`](https://github.com/siderolabs/omni/commit/3154d591e7c65713c6940d953df45d8242ae9359) fix: respect SIDEROLINK_DEV_JOIN_TOKEN only in debug mode
* [`38f5380`](https://github.com/siderolabs/omni/commit/38f53802ab3dda70fedc0a81de9d6dd43e6204f1) feat: avoid deleting all resources on omnictl delete
* [`28666bc`](https://github.com/siderolabs/omni/commit/28666bcb4acaf6e4f053e99d8d45d5dae320c89c) chore: add support for local development using compose
* [`cad73ce`](https://github.com/siderolabs/omni/commit/cad73cefc6b187a26e3833089e89ca1cb6fbf843) chore: increase TestEtcdAudit timeout and fix incorrect `Assert()` calls.
* [`7199b75`](https://github.com/siderolabs/omni/commit/7199b75c2108568d8bee82c42fcc00edb4a22e1c) chore: during `config merge` create config if there was none
* [`dab54d1`](https://github.com/siderolabs/omni/commit/dab54d14fcd8c0fadc6bb2a49d79e90379234403) chore: increase `TestTalosBackendRoles` reliability
* [`997cd78`](https://github.com/siderolabs/omni/commit/997cd7823bd126302ed4772658c0791768d67638) feat: add reconfiguring phase to machinesetstatus
* [`81fb2b9`](https://github.com/siderolabs/omni/commit/81fb2b94e61f7e7aaf41075fe17a2bbfea005d9f) fix: fix button order and vue config
* [`252fb29`](https://github.com/siderolabs/omni/commit/252fb29d64dac660da08459d9c5acc44e457b034) refactor: simplify backend.Server.Run method
* [`f335c2f`](https://github.com/siderolabs/omni/commit/f335c2f5311a81ca23699c473b68bf6918430aab) refactor: split watch to `Watch` and `WatchFunc`, add unit tests
* [`35a7919`](https://github.com/siderolabs/omni/commit/35a79193b965d42fba0a649bef0efe82abbd2fd5) feat: track machine config apply status
* [`1c54710`](https://github.com/siderolabs/omni/commit/1c54710c6f5ebe2740af27cebfb9c5532b22cc26) fix: use rolling update strategy on control planes
* [`17ccdc2`](https://github.com/siderolabs/omni/commit/17ccdc2f78693b5d1276b843c027e8057faa2ff7) refactor: various logging fixes
* [`3c9ca9c`](https://github.com/siderolabs/omni/commit/3c9ca9cd83298c5281c7ced50720b341c10a02f0) fix: update node overview Kubernetes node watch to make it compatible
* [`e8c2063`](https://github.com/siderolabs/omni/commit/e8c20631501308952bbc596e994a71b7677034b3) fix: enable edit config patches button on the cluster overview page
* [`6e80521`](https://github.com/siderolabs/omni/commit/6e8052169dd672e6fce5668982b704331eac4645) fix: reset the item list after the watch gets reconnected
* [`620d197`](https://github.com/siderolabs/omni/commit/620d1977a70bbc2cca8b331db825fc7bdb8fcda3) chore: remove AddContext method from runtime.Runtime interface
* [`8972ade`](https://github.com/siderolabs/omni/commit/8972ade40dea2bf3bf41bcb865a817d90b37657d) chore: update default version of Talos to v1.2.7
* [`6a2dde8`](https://github.com/siderolabs/omni/commit/6a2dde863d306986027904167f262d4307a7420d) fix: update the config patch rollout strategy
* [`fb3f6a3`](https://github.com/siderolabs/omni/commit/fb3f6a340c37d1958e36400edf7ca53e2cde48a7) fix: skip updating config status if applying config caused a reboot
* [`8776146`](https://github.com/siderolabs/omni/commit/877614606d0c7d0259c4e65e4911f331550dd7d7) fix: apply finalizer to the `Machine` only when CMS is created
* [`134bb20`](https://github.com/siderolabs/omni/commit/134bb2053ce6250b9b4c647f3b2dbb8255cea2ce) test: fix config patch test with reboot
* [`d3b6b5a`](https://github.com/siderolabs/omni/commit/d3b6b5a75f9ea5304595851d6160e98ec4c9b8aa) feat: implement config patch viewer and editor
* [`149efe1`](https://github.com/siderolabs/omni/commit/149efe189a24c07e648289ee81d0b95ed1c972b7) chore: bump runtime and state-etcd modules
* [`c345b83`](https://github.com/siderolabs/omni/commit/c345b8348412aef59cbd43c35bf06ce3eac5ad3f) chore: output omnictl auth log to stderr
* [`39b2ba2`](https://github.com/siderolabs/omni/commit/39b2ba2a86972324161c6cff056abf10eb2fce5c) refactor: introduce ClusterEndpoint resource
* [`6998ff0`](https://github.com/siderolabs/omni/commit/6998ff0803063b22e113da0c72356ee254f13143) fix: treat created and updated events same
* [`289fe88`](https://github.com/siderolabs/omni/commit/289fe88aba94d6cfe4d7be7472b609232e45cbf6) feat: add omnictl apply
* [`2f1be3b`](https://github.com/siderolabs/omni/commit/2f1be3b4643e2a66a62da6a7f8f1f1da39ed6e17) chore: fix `TestGenerateJoinToken` test
* [`3829176`](https://github.com/siderolabs/omni/commit/382917630030415b1a218f14f2a1d6d3595834a0) fix: don't close config patch editor window if config validation fails
* [`c96f504`](https://github.com/siderolabs/omni/commit/c96f5041be7befb517998fc7bbccd135cb76908d) feat: add suspended mode
* [`b967bcf`](https://github.com/siderolabs/omni/commit/b967bcfd26b2fccfa6bbb08b8a15eb3796e2e872) feat: add last config apply error to clustermachineconfigstatus
* [`0395d9d`](https://github.com/siderolabs/omni/commit/0395d9dd7b985802be8f4cd2b8005b409faca3de) test: increase key generation timeout on storage signing test
* [`577eba4`](https://github.com/siderolabs/omni/commit/577eba4231142fe983f9a0f9b5a81280c377686e) fix: set SideroLink MTU to 1280
* [`0f32172`](https://github.com/siderolabs/omni/commit/0f32172922ed2f7b8b4b7433fb1f9ce104f3c5a8) fix: minor things in frontend
* [`9abcc7b`](https://github.com/siderolabs/omni/commit/9abcc7b444c49f6223e0ae4948bff13eedbb05b5) test: add config patching integration tests
* [`99531fb`](https://github.com/siderolabs/omni/commit/99531fbeee982e2ab87d9f0162a0080308b852ab) refactor: drop unneeded controller inputs
* [`5172354`](https://github.com/siderolabs/omni/commit/51723541621d91964e88e8a5add834159214dc5b) chore: add omnictl to the generated image
* [`738cf64`](https://github.com/siderolabs/omni/commit/738cf649f53ec29e88112a027ec72f3d6f0cfff8) fix: set cluster machine version in machine config status correctly
* [`1d0d220`](https://github.com/siderolabs/omni/commit/1d0d220f47f1cc9ca8b20bfef47004a875b7573c) fix: lower ttl of the issued keys on the FE side by 10 minutes
* [`2889524`](https://github.com/siderolabs/omni/commit/2889524f222e42d49061867b2b2f5b59a16af4ba) feat: dynamic title
* [`3d17bd7`](https://github.com/siderolabs/omni/commit/3d17bd7cfd4775292090ccb3fd3c2b575b26d449) chore: fix release CI run
* [`f2c752f`](https://github.com/siderolabs/omni/commit/f2c752fed627006912018ae3e5f2ff0f2bed60b8) fix: properly proxy watch requests through dev-server
* [`9a74897`](https://github.com/siderolabs/omni/commit/9a74897d0ce60a51086f5af98c4c4eb71f2b0009) release(v0.1.0-alpha.1): prepare release
* [`8b284f3`](https://github.com/siderolabs/omni/commit/8b284f3aa26cf8a34452f33807dcc04045e7a098) feat: implement Kubernetes API OIDC proxy and OIDC server
* [`adad8d0`](https://github.com/siderolabs/omni/commit/adad8d0fe2f3356e97de613104196233a3b98ff5) refactor: rework LoadBalancerConfig/LoadBalancerStatus resources
* [`08e2cb4`](https://github.com/siderolabs/omni/commit/08e2cb4fd40ec918bf458edd6a5d8e6c86fe5c97) feat: support editing config patches on cluster and machine set levels
* [`e2197c8`](https://github.com/siderolabs/omni/commit/e2197c83e994afb435671f5af5cdefa843bbddb5) test: e2e testing improvements
* [`ec9051f`](https://github.com/siderolabs/omni/commit/ec9051f6dfdf1f5acaf3fa6766dc1195b6f6dcdd) fix: config patching
* [`e2a1d6c`](https://github.com/siderolabs/omni/commit/e2a1d6c78809eaa4168ca5ede433824797a6aa4e) fix: send logs in JSON format by default
* [`954dd70`](https://github.com/siderolabs/omni/commit/954dd70b935b7c373ba5830fd7ad6e965f6b0da8) chore: replace talos-systems depedencies with siderolabs
* [`acf94db`](https://github.com/siderolabs/omni/commit/acf94db8ac80fb6f15cc87ff276b7edca0cb8661) chore: add payload logger
* [`838c716`](https://github.com/siderolabs/omni/commit/838c7168c64f2296a9e01d3ef6ab4feb9f16aeb9) fix: allow time skew on validating the public keys
* [`dd481d6`](https://github.com/siderolabs/omni/commit/dd481d6cb3620790f6e7a9c8e305defb507cbe5f) fix: refactor runGRPCProxy in router tests to catch listener errors
* [`e68d010`](https://github.com/siderolabs/omni/commit/e68d010685d4f0a5d25fee671744119cecf6c27b) chore: small fixes
* [`ad86875`](https://github.com/siderolabs/omni/commit/ad86875ec146e05d7d7f461bf7c8094a8c143df5) feat: minor adjustments on the cluster create page
* [`e61f194`](https://github.com/siderolabs/omni/commit/e61f1943e965287c79fbaef05760bb0b0deee988) chore: implement debug handlers with controller dependency graphs
* [`cbbf901`](https://github.com/siderolabs/omni/commit/cbbf901e601d31c777ad2ada0f0036c57020ba96) refactor: use generic TransformController more
* [`33f9f2c`](https://github.com/siderolabs/omni/commit/33f9f2ce3ec0999198f311ae4bae9b58e57153c9) chore: remove reflect from runtime package
* [`6586963`](https://github.com/siderolabs/omni/commit/65869636aa33013b5feafb06e727b9d2a4cf1c19) feat: add scopes to users, rework authz & add integration tests
* [`bb355f5`](https://github.com/siderolabs/omni/commit/bb355f5c659d8c66b825de409d9446767005a2bb) fix: reload the page to init the UI Authenticator on signature fails
* [`c90cd48`](https://github.com/siderolabs/omni/commit/c90cd48eefa7f29328a456aa5ca474eece17c6fe) chore: log auth context
* [`d278780`](https://github.com/siderolabs/omni/commit/d2787801a4904fe895996e5319f301a1d7ca76df) fix: update Clusters page UI
* [`5e77607`](https://github.com/siderolabs/omni/commit/5e776072285e535e93c0458774dcad810b9b857a) tests: abort on first failure
* [`4c55980`](https://github.com/siderolabs/omni/commit/4c5598083ff6d8763c8763d8e46a3d7b659784ff) chore: get full method name from the service
* [`2194f43`](https://github.com/siderolabs/omni/commit/2194f4391607e6e73bce1917d2744e78fdd2cebc) feat: redesign cluster list view
* [`40b3f23`](https://github.com/siderolabs/omni/commit/40b3f23071096987e8a7c6f30a2622c317c190cb) chore: enable gRPC request duration histogram
* [`0235bb9`](https://github.com/siderolabs/omni/commit/0235bb91a71510cf4d349eedd3625b119c7e4e11) refactor: make sure Talos/Kubernetes versions are defined once
* [`dd6154a`](https://github.com/siderolabs/omni/commit/dd6154a45d5dcd14870e0aa3f97aa1d4e53bdcfb) chore: add public key pruning
* [`68908ba`](https://github.com/siderolabs/omni/commit/68908ba330ecd1e285681e24db4b9037eb2e8202) fix: bring back UpgradeInfo API
* [`f1bc692`](https://github.com/siderolabs/omni/commit/f1bc692c9125f7683fe5f234b03eb3521ba7e773) refactor: drop dependency on Talos Go module
* [`0e3ef43`](https://github.com/siderolabs/omni/commit/0e3ef43cfed68e53879e6c22b46e7d0568ddc05f) feat: implement talosctl access via Omni
* [`2b0014f`](https://github.com/siderolabs/omni/commit/2b0014fea15da359217f89ef723965dcc9faa739) fix: provide a way to switch the user on the authenticate page
* [`e295d7e`](https://github.com/siderolabs/omni/commit/e295d7e2854ac0226e7efda32864f6a687a88470) chore: refactor all controller tests to use assertResource function
* [`8251dfb`](https://github.com/siderolabs/omni/commit/8251dfb9e44341e9df9471f387cc76c91359cf84) refactor: extract PGP client key handling
* [`02da9ee`](https://github.com/siderolabs/omni/commit/02da9ee66f15462e6f4d7da18515651a5fde11aa) refactor: use extracted go-api-signature library
* [`4bc3db4`](https://github.com/siderolabs/omni/commit/4bc3db4dcbc14e0e51c7a3b5257686b671cc2823) fix: drop not working upgrade k8s functional
* [`17ca75e`](https://github.com/siderolabs/omni/commit/17ca75ef864b7a59f9c6f829de19cc9630a670c0) feat: add 404 page
* [`8dcde2a`](https://github.com/siderolabs/omni/commit/8dcde2af3ca49d9be16cc705c0b403826f2eee5d) feat: implement logout flow in the frontend
* [`ba766b9`](https://github.com/siderolabs/omni/commit/ba766b9922302b9d1f279b74caf94e6ca727f86f) fix: make `omnictl` correctly re-auth on invalid key
* [`fd16f87`](https://github.com/siderolabs/omni/commit/fd16f8743d3843e8ec6735a7c2e96532694b876e) fix: don't set timeout on watch gRPC requests
* [`8dc3cc6`](https://github.com/siderolabs/omni/commit/8dc3cc682e5419c3824c6e740a32085c386b8817) fix: don't use `omni` in external names
* [`2513661`](https://github.com/siderolabs/omni/commit/2513661578574255ca3f736d3dfa1f307f5d43b6) fix: reset `Error` field of the `MachineSetStatus`
* [`b611e99`](https://github.com/siderolabs/omni/commit/b611e99e14a7e2ebc64c55ed5c95a47e17d6ac32) fix: properly handle `Forbidden` errors on the authentication page
* [`8525502`](https://github.com/siderolabs/omni/commit/8525502265b10dc3cc056d301785f6f60e4f7e22) fix: stop runners properly and clean up StatusMachineSnapshot
* [`ab0190d`](https://github.com/siderolabs/omni/commit/ab0190d9a41b830daf60173b998acdbcbbdd3754) feat: implement scopes and enforce authorization
* [`9198d96`](https://github.com/siderolabs/omni/commit/9198d96ea9d57bb5949c59350aec42b2ce13ebac) feat: sign gRPC requests on the frontend to enable Authentication flow
* [`bdd8f21`](https://github.com/siderolabs/omni/commit/bdd8f216a9eca7ec657fa0dc554e663743f058d1) chore: remove reset button and fix padding
* [`362db57`](https://github.com/siderolabs/omni/commit/362db570349b4a2659f746ce18a436d684481ecb) fix: gRPC verifier should verify against original JSON payload
* [`30186b8`](https://github.com/siderolabs/omni/commit/30186b8cfe2eea6eaade8bacf31114886d3da3ea) fix: omnictl ignoring omniconfig argument
* [`e8ab0ba`](https://github.com/siderolabs/omni/commit/e8ab0ba45648b8f521500b46fe032797da6a111f) fix: do not attempt to execute failed integration test again
* [`9fda25e`](https://github.com/siderolabs/omni/commit/9fda25ef45f0060cc6c3ec812f5fa1c7b1015801) chore: add more info on errors to different controllers
* [`ccda526`](https://github.com/siderolabs/omni/commit/ccda5260c4645b5929724574a9f856eeaa4c232f) chore: bump grpc version
* [`b1ac125`](https://github.com/siderolabs/omni/commit/b1ac1255da5ca4b5d9c409e27c51e4298275e73c) chore: emit log when we got machine status event.
* [`005d257`](https://github.com/siderolabs/omni/commit/005d257c25c745b61e5a25c39167d511710562c7) chore: set admin role specifically for Reboot request.
* [`27f0e30`](https://github.com/siderolabs/omni/commit/27f0e309cec76a454e5bb24c2df1e62d9e4718e0) chore: update deps
* [`77f0219`](https://github.com/siderolabs/omni/commit/77f02198c1e7fb215548f3a0e2be30a0e19aaf6d) test: more unit-tests for auth components
* [`0bf6ddf`](https://github.com/siderolabs/omni/commit/0bf6ddfa46e0ea6ad255ede00a600c390344e221) fix: pass through HTTP request if auth is disabled
* [`4f3a67b`](https://github.com/siderolabs/omni/commit/4f3a67b08e03a1bad65c2acb8d65f0281fdd2f9e) fix: unit-tests for auth package and fixes
* [`e3390cb`](https://github.com/siderolabs/omni/commit/e3390cbbac1d0e78b72512c6ebb64a8f53dcde17) chore: rename arges-theila to omni
* [`14d2614`](https://github.com/siderolabs/omni/commit/14d2614538ec696d468a0850bd4ee7bc6884c3b1) chore: allow slashes in secretPath
* [`e423edc`](https://github.com/siderolabs/omni/commit/e423edc072714e7f693249b60079f5f700cc0a65) fix: add unit-tests for auth message and fix issues
* [`b5cfa1a`](https://github.com/siderolabs/omni/commit/b5cfa1a84e93b6bbf5533c599917f293fc5cdf66) feat: add vault client
* [`b47791c`](https://github.com/siderolabs/omni/commit/b47791ce303cbb9a8aab279685d17f92a480c7f4) feat: sign grpc requests on cli with pgp key & verify it on server
* [`d6ef4d9`](https://github.com/siderolabs/omni/commit/d6ef4d9c36758cb0091e2c528b848952f312941a) feat: split account ID and name
* [`e412e1a`](https://github.com/siderolabs/omni/commit/e412e1a69edad0d19d7e46fa3aa076dcb8e6d4b6) chore: workaround the bind problem
* [`e23cc59`](https://github.com/siderolabs/omni/commit/e23cc59bb8cb8f9df81738d4c58aed08d80fa9c4) chore: bump minimum Talos version to v1.2.4
* [`0638a29`](https://github.com/siderolabs/omni/commit/0638a29d78c092641573aa2b8d2e594a7ff6aab4) feat: stop using websockets
* [`8f3c19d`](https://github.com/siderolabs/omni/commit/8f3c19d0f0ecfbe5beabc7dc508dcafa720e83e2) feat: update install media to be identifiable
* [`70d1e35`](https://github.com/siderolabs/omni/commit/70d1e354466618bb07c13445a16ca639be12009e) feat: implement resource encryption
* [`7653638`](https://github.com/siderolabs/omni/commit/76536386499889994b65f66a8a40f18b5535c5ba) fix: fix NPE in integration tests
* [`e39849f`](https://github.com/siderolabs/omni/commit/e39849f4047f028251123781bd8be350ebbfd65d) chore: update Makefile and Dockerfile with kres
* [`4709473`](https://github.com/siderolabs/omni/commit/4709473ec20fbf92a3240fb3376a322f1321103a) fix: return an error if external etcd client fails to be built
* [`5366661`](https://github.com/siderolabs/omni/commit/536666140556ba9b997a2b5d4441ea4b5f42d1c5) refactor: use generic transform controller
* [`a2a5f16`](https://github.com/siderolabs/omni/commit/a2a5f167f21df6375767d018981651d60bb2f768) feat: limit access to Talos API via Omni to `os:reader`
* [`e254201`](https://github.com/siderolabs/omni/commit/e2542013938991faa8f1c521fc524b8fcf31ea34) feat: merge internal/external states into one
* [`3258ca4`](https://github.com/siderolabs/omni/commit/3258ca487c818a34924f138640f44a2e51d307fb) feat: add `ControlPlaneStatus` controller
* [`1c0f286`](https://github.com/siderolabs/omni/commit/1c0f286a28f5134333130708d031dbfa11051a42) refactor: use `MachineStatus` Talos resource
* [`0a6b19f`](https://github.com/siderolabs/omni/commit/0a6b19fb916ea301a8f5f6ccd9bbdaa7cb4c39e0) chore: drop support for Talos resource API
* [`ee5f6d5`](https://github.com/siderolabs/omni/commit/ee5f6d58a2b22a87930d3c8bb9963f71c92f3908) feat: add auth resource types & implement CLI auth
* [`36736e1`](https://github.com/siderolabs/omni/commit/36736e14e5c837d38568a473834d14073b88a153) fix: use correct protobuf URL for cosi resource spec
* [`b98c56d`](https://github.com/siderolabs/omni/commit/b98c56dafe33beef7792bd861ac4e637fe13c494) feat: bump minimum version for Talos to v1.2.3
* [`b93bc9c`](https://github.com/siderolabs/omni/commit/b93bc9cd913b017c66502d96d99c52e4d971e231) chore: move containers and optional package to the separate module
* [`e1af4d8`](https://github.com/siderolabs/omni/commit/e1af4d8a0bee31721d8946ef452afe04da6b494d) chore: update COSI to v0.2.0-alpha.1
* [`788dd37`](https://github.com/siderolabs/omni/commit/788dd37c0be32745547ee8268aa0f004041dc96f) feat: implement and enable by default etcd backend
* [`1b83038`](https://github.com/siderolabs/omni/commit/1b83038b77cab87ffc2d4d73a91582785ed446ef) release(v0.1.0-alpha.0): prepare release
* [`8a9c4f1`](https://github.com/siderolabs/omni/commit/8a9c4f17ed6ee0d8e4a51b466d60a8278cd50f9c) feat: implement CLI configuration file (omniconfig)
* [`b0c92d5`](https://github.com/siderolabs/omni/commit/b0c92d56da00529c106f042399c1163375046785) feat: implement etcd audit controller
* [`0e993a0`](https://github.com/siderolabs/omni/commit/0e993a0977c711fb8767e3de2ad828fd5b9e688f) feat: properly support scaling down the cluster
* [`264cdc9`](https://github.com/siderolabs/omni/commit/264cdc9e015fd87724c7a07128d1136153732540) refactor: prepare for etcd backend integration
* [`b519d17`](https://github.com/siderolabs/omni/commit/b519d17971bb1c919286813b4c2465c2f5803a03) feat: show version in the UI
* [`a2fb539`](https://github.com/siderolabs/omni/commit/a2fb5397f9efb22a1354c5675180ca49537bee55) feat: keep track of loadbalancer health in the controller
* [`4789c62`](https://github.com/siderolabs/omni/commit/4789c62af0d1694d8d0a492cd6fb7d436e213fe5) feat: implement a new controller that can gather cluster machine data
* [`bd3712e`](https://github.com/siderolabs/omni/commit/bd3712e13491ede4610ab1452ae85bde6d92b2db) fix: populate machine label field in the patches created by the UI
* [`ba70b4a`](https://github.com/siderolabs/omni/commit/ba70b4a48623939d31775935bd0338c0d60ab65b) fix: rename to Omni, fix workers scale up, hide join token
* [`47b45c1`](https://github.com/siderolabs/omni/commit/47b45c129160821576d808d9a46a9ec5d14c6469) fix: correct filenames for Digital Ocean images
* [`9d217cf`](https://github.com/siderolabs/omni/commit/9d217cf16d432c5194110ae16a566b44b02a567e) feat: introduce new resources, deprecate `ClusterMachineTemplate`
* [`aee153b`](https://github.com/siderolabs/omni/commit/aee153bedb2f7856913a54b282603b07bf20059b) fix: address style issue in the Pods paginator
* [`752dd44`](https://github.com/siderolabs/omni/commit/752dd44ac42c95c644cad5640f6b2c5536a29676) chore: update Talos machinery to 1.2.0 and use client config struct
* [`88d7079`](https://github.com/siderolabs/omni/commit/88d7079a6656605a1a8dfed56d392414583a283e) fix: regenerate sources from proto files that were rolled back.
* [`84062c5`](https://github.com/siderolabs/omni/commit/84062c53417197417ff636a667289342089f390c) chore: update Talos to the latest master
* [`5a139e4`](https://github.com/siderolabs/omni/commit/5a139e473abcdf7fd25ad7c61dad8cbdc964a453) fix: properly route theila internal requests in the gRPC proxy
* [`4be4fb6`](https://github.com/siderolabs/omni/commit/4be4fb6a4e0bca29b32e1b732c227c9e7a0b1f43) feat: add support for 'talosconfig' generation
* [`9235b8b`](https://github.com/siderolabs/omni/commit/9235b8b522d4bc0712012425b68ff89e455886b9) fix: properly layer gRPC proxies
* [`9a516cc`](https://github.com/siderolabs/omni/commit/9a516ccb5c892ed8fe41f7cf69aaa5bb1d3fa471) fix: wait for selector of 'View All' to render in e2e tests.
* [`3cf3aa7`](https://github.com/siderolabs/omni/commit/3cf3aa730e7833c0c1abe42a6afb87a85f14b58c) fix: some unhandled errors in the e2e tests.
* [`c32c7d5`](https://github.com/siderolabs/omni/commit/c32c7d55c92007aa1aa10feab3c7a7de2b2afc42) fix: ignore updating cluster machines statuses without machine statuses
* [`4cfa307`](https://github.com/siderolabs/omni/commit/4cfa307b85b410b44e482b259d14670b55e4a237) chore: run rekres, fix lint errors and bump Go to 1.19
* [`eb2d449`](https://github.com/siderolabs/omni/commit/eb2d4499f1a3da7bc1552a6b099c28bed6fd0e4d) fix: skip the machines in `tearingDown` phase in the controller
* [`9ebc769`](https://github.com/siderolabs/omni/commit/9ebc769b89a2bab37fd081e555f84e3e4c99187e) fix: allow all services to be proxied by gRPC router
* [`ea2b01d`](https://github.com/siderolabs/omni/commit/ea2b01d0a0e054b259d710317fe368882534cf4c) fix: properly handle non empty resource id in the K8s resource watch
* [`3bb7da3`](https://github.com/siderolabs/omni/commit/3bb7da3a0fa6b746f6a7b9aa668e055bdf825e6a) feat: show a Cluster column in the Machine section
* [`8beb70b`](https://github.com/siderolabs/omni/commit/8beb70b7f045a218f9cb753e1402a07542b0bf1c) fix: ignore tearing down clusters in the `Cluster` migrations
* [`319d4e7`](https://github.com/siderolabs/omni/commit/319d4e7947cb78135f5a14c02afe5814c56a312c) fix: properly handle `null` memory modules list
* [`6c2120b`](https://github.com/siderolabs/omni/commit/6c2120b5ae2bd947f473d002dfe165646032e811) chore: introduce migrations manager for COSI DB state
* [`ec52139`](https://github.com/siderolabs/omni/commit/ec521397946cc15929472feb7c45435fb48df848) fix: filter out invalid memory modules info coming from Talos nodes
* [`8e87031`](https://github.com/siderolabs/omni/commit/8e870313a3a31d052eecf81acb522433ff98ae79) fix: bump loadbalancer timeout settings
* [`bc0ed26`](https://github.com/siderolabs/omni/commit/bc0ed2672064a6bf148cd9799b35a2790f5aa7f6) feat: introduce websocket, HTTP requests monitoring
* [`857401f`](https://github.com/siderolabs/omni/commit/857401f54e3922a9ab85d7dc703a5afb70c6ee45) feat: add HTTP logging (static, gateway), and websocket logging
* [`eb612a3`](https://github.com/siderolabs/omni/commit/eb612a38e9c71913ebecc9f345e17844d60800b8) fix: do hard stop of events sink gRPC server after 5 seconds
* [`3162513`](https://github.com/siderolabs/omni/commit/31625135e2b971d6b9f92eb4096c010113030a80) fix: populate nodes filter dropdown properly and rewrite filter function
* [`5713a51`](https://github.com/siderolabs/omni/commit/5713a516391a5190fac9b7044a9f71952ce15479) fix: make `TSelectList` search filter the items in the dropdown
* [`f2519ff`](https://github.com/siderolabs/omni/commit/f2519ff51b88766a907f1d7717ef74031157fd56) feat: don't allow using nodes with not enough mem for the cluster
* [`9e474d6`](https://github.com/siderolabs/omni/commit/9e474d69c76a898fc5b6fcd9fdc8e87f25b7dc53) feat: show disconnected warning in the machines list
* [`fa52b48`](https://github.com/siderolabs/omni/commit/fa52b48f54362c7305681ca79a7d98237531f2b4) feat: redesign Installation Media selection menu
* [`01e301a`](https://github.com/siderolabs/omni/commit/01e301a875699cf6fcc887cb31cd7939338f58e9) fix: query node list using `talosctl get members` instead of K8s nodes
* [`e694df5`](https://github.com/siderolabs/omni/commit/e694df59c50fbee356a48c94ade95e924ea46bb2) fix: display all available Talos versions on cluster create page
* [`7a87525`](https://github.com/siderolabs/omni/commit/7a87525ed1b928a8f8e3e6a39feb4c19009ec264) fix: use `v-model` instead of callbacks in the inputs
* [`d681f5f`](https://github.com/siderolabs/omni/commit/d681f5f58788612f144fa1f8d90ec6c996badb0e) feat: support scaling up the clusters
* [`e992b95`](https://github.com/siderolabs/omni/commit/e992b9574d7b8f76497f46e25764618ec274af1a) feat: show notification on image download progress
* [`8ea6d9f`](https://github.com/siderolabs/omni/commit/8ea6d9f1724b271919e538ed55ff6582858470f9) fix: probably fix 'context canceled' on image download
* [`692612b`](https://github.com/siderolabs/omni/commit/692612b7e628588fa7608cff683c5af406f24ca7) fix: improve the Talos image generation process
* [`a69c140`](https://github.com/siderolabs/omni/commit/a69c140e26f4298fcaafb1f96c389269992fc069) feat: introduce Prometheus metrics
* [`e90ca78`](https://github.com/siderolabs/omni/commit/e90ca7875c501391f860f5df9f2a4e4f8e2f2d7a) fix: make grpc api listen only on siderolink interface
* [`99fc28c`](https://github.com/siderolabs/omni/commit/99fc28c36c62a8d8c654c05f9b9c64ff37cedba8) fix: display correct cluster/machine status on ui
* [`eaf7655`](https://github.com/siderolabs/omni/commit/eaf7655395401cd88e6bd47f4f8aa958abee30f1) fix: add a pause before integration tests
* [`19ff1c9`](https://github.com/siderolabs/omni/commit/19ff1c909bedf63fe6cf2f5cc0e44f34046ca568) chore: rename download button
* [`e1c4e1b`](https://github.com/siderolabs/omni/commit/e1c4e1b171eab08585a3315ca5838c88a4d2eb24) feat: add download options for all talos images
* [`24e7863`](https://github.com/siderolabs/omni/commit/24e786369bfc0bb4966712296395db91751e657b) fix: delete cached clients from gRPC proxy when the cluster is destroyed
* [`58c89ef`](https://github.com/siderolabs/omni/commit/58c89ef3fe621ef6909c5d38a0d47cc861667f45) feat: implement `argesctl delete` command
* [`3c99b49`](https://github.com/siderolabs/omni/commit/3c99b49a9b680b091d92455a0d3bc325f8f68ca6) test: add a test which removes allocated machine
* [`75dd28f`](https://github.com/siderolabs/omni/commit/75dd28f56d7ce9a92b96822a867fbfe2655cd0fa) chore: fill in resource definitions for table headers
* [`028f168`](https://github.com/siderolabs/omni/commit/028f16886c41b7aa7eafb65308cc4adf4d624037) feat: End-to-end tests with playwright
* [`6be6b36`](https://github.com/siderolabs/omni/commit/6be6b3605583ce8e8068746624ca86ed6adc37af) chore: bump goimports from 0.1.10 to 0.1.11 and node from 18.5.0 to 18.6.0
* [`af4da08`](https://github.com/siderolabs/omni/commit/af4da086d4b709f504eda7909a36a8f0cf84e480) test: implement kernel log streaming test
* [`1eacfee`](https://github.com/siderolabs/omni/commit/1eacfee2c1084040ae2201eba957409218a92c66) feat: implement argesctl machine-logs output in 'zap-like' and 'dmesg' form.
* [`96ab7ab`](https://github.com/siderolabs/omni/commit/96ab7ab8317898dd45d129d5cecd2aaf1d379fba) chore: ignore memory modules with zero size
* [`fd0575f`](https://github.com/siderolabs/omni/commit/fd0575ff4050702c9d07e34c7d9d5596b4ad7311) chore: retrieve k8s versions from github registry
* [`8651527`](https://github.com/siderolabs/omni/commit/86515275a77741bacc790d2006f3671a5cfb27c6) feat: redo errgroup to return error on first nil error
* [`944222d`](https://github.com/siderolabs/omni/commit/944222d06607079b5d982afe4b19fc1dda7f1ec2) fix: show ClusterMachineStatus.Stage in 'Clusters' view
* [`f3f6b6e`](https://github.com/siderolabs/omni/commit/f3f6b6eecd3ffc13b69845dff50d2e8ab31bc0d2) chore: refactor run method and no longer ignore log receiver listener errors
* [`b316377`](https://github.com/siderolabs/omni/commit/b316377b277f87a184b969b3bbf20ebe6047a0a8) chore: rename 'Dmesg' to 'Console'
* [`19ee857`](https://github.com/siderolabs/omni/commit/19ee8578a6f1c1bf742699d1b5720dc4c2674c82) test: add a way to recover deleted machines
* [`e5b5bdc`](https://github.com/siderolabs/omni/commit/e5b5bdc39fa6f3812b15771366f942ddcbe7f328) fix: update SideroLink library for EEXIST fixes
* [`363de69`](https://github.com/siderolabs/omni/commit/363de69a50b5c1e9d07fa42152cca935844d118b) fix: spec collector equality
* [`841f3b2`](https://github.com/siderolabs/omni/commit/841f3b22aacc6d2875062ef324d900c5f2091f9d) feat: add ability to supply machine config patches on the machines
* [`907ca93`](https://github.com/siderolabs/omni/commit/907ca93247267d80125866c2b60225ceca3ada27) test: fix link destroy test
* [`4c9f99d`](https://github.com/siderolabs/omni/commit/4c9f99d32874cdaff1eb651bf6d74ef39167c273) fix: remove machine status if the machine is in tearing down phase
* [`d9747e5`](https://github.com/siderolabs/omni/commit/d9747e552e52156a9baeae962a9478231e26c566) fix: make cluster machine status test more reliable
* [`3bfff3b`](https://github.com/siderolabs/omni/commit/3bfff3bb0eea9d18956dee21aff7f3de900c6b82) fix: do not set up full theila runtime during clients tests
* [`4bf33bc`](https://github.com/siderolabs/omni/commit/4bf33bc9d37404a733c5039784c80e92800fb3dc) fix: immediately fail the request if the cluster is down
* [`124a5c2`](https://github.com/siderolabs/omni/commit/124a5c2947978e6bc86d1b19c9eacbcf7f870b53) fix: ensure the created date on resources is set
* [`14161bf`](https://github.com/siderolabs/omni/commit/14161bf3dad4484868359d186d99d9198b6eed95) feat: add scale up integration test and minor log fixes
* [`7af06fd`](https://github.com/siderolabs/omni/commit/7af06fd75959eb9e807680ac8a6ba4f0a7f59255) feat: make integration tests a subtests of one global test
* [`f7c1464`](https://github.com/siderolabs/omni/commit/f7c1464a1002f63daab29b36d19ea16de0cd5794) feat: implement log receiver for logs from Talos
* [`5b800ea`](https://github.com/siderolabs/omni/commit/5b800ea970215fb4e100ed7b3b73d7e218fd6d86) fix: accumulate bytes received/send in the link resource
* [`b3b1e9b`](https://github.com/siderolabs/omni/commit/b3b1e9bbfbf62632dc0d8c2239a72793883101ce) feat: machine removal
* [`fb01bc4`](https://github.com/siderolabs/omni/commit/fb01bc4b26c5b37f15bac923450e1f58fb7a3d89) fix: use Talos 1.2.0
* [`3a50efe`](https://github.com/siderolabs/omni/commit/3a50efe363c4724f369a02f672848ad7c284847c) feat: filter machines that can be added to cluster
* [`ba62db5`](https://github.com/siderolabs/omni/commit/ba62db521b47049e92557bf8cfc5f737e496bf57) fix: properly parse `siderolink-api-advertised-url` if there's no port
* [`96f835a`](https://github.com/siderolabs/omni/commit/96f835a91136f62d9dbdf5c1d1c46c729d57e51e) fix: properly display node selectors in FireFox
* [`12c20a4`](https://github.com/siderolabs/omni/commit/12c20a42c9dfdea5f88e0e7942fbdb42ea543b95) fix: populate disks when machines are connected during cluster create
* [`0dc97f8`](https://github.com/siderolabs/omni/commit/0dc97f8696a7c571d5318daf794700342e06f639) fix: adjust overview page to look closer to the mockups
* [`2b77af8`](https://github.com/siderolabs/omni/commit/2b77af8d39e555970487c3265dfbd63412e90d2f) feat: add the chart showing the count of clusters
* [`a1dff65`](https://github.com/siderolabs/omni/commit/a1dff6589d64207e6e7331d0407e7857f9c4079d) feat: implement ISO download with embedded kernel args
* [`37c03d8`](https://github.com/siderolabs/omni/commit/37c03d8cb04b02e79f42e70eeea1e4368445604d) test: pull kubeconfig and interact with Kubernetes API
* [`75bfb08`](https://github.com/siderolabs/omni/commit/75bfb08f0738fc9f67259caf12902db67860370f) fix: ignore the error on splitting host/port
* [`3be5a32`](https://github.com/siderolabs/omni/commit/3be5a3254168cddec8f1629789c2ae50d9eaa08e) feat: make the whole cluster list item clickable, add dropdown menu item
* [`2c9dc99`](https://github.com/siderolabs/omni/commit/2c9dc99000266b3d4c139f27dea4f6283709251e) fix: adjust the look of the Overview page a bit
* [`aa4a926`](https://github.com/siderolabs/omni/commit/aa4a926cbb85bf63312493b937440a174aed5070) feat: add the button for downloading cluster Kubeconfig on overview page
* [`4532de6`](https://github.com/siderolabs/omni/commit/4532de6f3d514a534c38a63731c43075698f5c01) feat: support basic auth in `argesctl` command
* [`b66bb3c`](https://github.com/siderolabs/omni/commit/b66bb3cbcc85d7be4348ecd9a6d5d62f72a90e11) feat: add summary information Overview page
* [`3bdbce4`](https://github.com/siderolabs/omni/commit/3bdbce41a3ed89a42556d837bc0c5cfe417e22e6) test: more cluster creation tests, two clusters, cleanup
* [`3b00bd5`](https://github.com/siderolabs/omni/commit/3b00bd5bf417c5c9cb42471d27811c1849a40c78) fix: improve cluster deletion and node reset flow
* [`2d83d16`](https://github.com/siderolabs/omni/commit/2d83d1694ec73da818004f91ede76a0bca30fe79) test: create a cluster and verify cluster machine statuses
* [`f471cfd`](https://github.com/siderolabs/omni/commit/f471cfdcf7c9e70f37436e173c3a58c1965e8bb2) fix: copy all labels from the `ClusterMachine` to `ClusterMachineStatus`
* [`ec32f86`](https://github.com/siderolabs/omni/commit/ec32f8632db104efd6fedc5421179175274d6339) test: add integration tests up to the cluster creation
* [`a8d3ee5`](https://github.com/siderolabs/omni/commit/a8d3ee5b14a57ad1d9d88512a95032bbda61e734) feat: add kubeconfig command to argesctl and fix kubeconfig
* [`10b9a3b`](https://github.com/siderolabs/omni/commit/10b9a3ba676a636e488805ed04a0c908c3d2cf53) test: implement API integration test
* [`3e6b891`](https://github.com/siderolabs/omni/commit/3e6b8913f916dc5e8ac3ef49e14648defa6e1bf6) feat: aggregate cluster machine statuses in cluster status controller
* [`f6cbc58`](https://github.com/siderolabs/omni/commit/f6cbc58a91124833f0cbae4ecd0c0416acbe8bfa) chore: ignore empty processor info
* [`c5fc71b`](https://github.com/siderolabs/omni/commit/c5fc71b86a5492d548ae9098c5c74de240ebd800) fix: clean up Kubernetes client and configs when a cluster is destroyed
* [`e8478fe`](https://github.com/siderolabs/omni/commit/e8478fe5280d5e8a32bb423ec96edacadabc7e43) fix: properly use tracker to cleanup `ClusterMachineConfig` resources
* [`044fcad`](https://github.com/siderolabs/omni/commit/044fcadb66de61742ab871d10f3fcf0f453f6e27) fix: make `MachineStatusController` connect to configured nodes
* [`2867099`](https://github.com/siderolabs/omni/commit/2867099a52d651c3b0f9d3abbae266f2792cafe7) feat: add api endpoint to fetch kubeconfig
* [`5f32667`](https://github.com/siderolabs/omni/commit/5f3266747012b590dd7a7d0ebc23ee0e80abb2ab) test: support registry mirrors for development purposes
* [`5114695`](https://github.com/siderolabs/omni/commit/5114695cfeb0b6c792002ff5f0f31c1944c269ab) refactor: consistent flag naming
* [`9ffb19e`](https://github.com/siderolabs/omni/commit/9ffb19e77968c6e411903a2c59fd9a18063b46d4) chore: use latest node
* [`5512321`](https://github.com/siderolabs/omni/commit/5512321f05b6b657a28abc25470664f6eb6e3d0a) refactor: set better defaults for cli args
* [`ff88242`](https://github.com/siderolabs/omni/commit/ff882427f56e42039b79900380b61b86d3290269) chore: mark 'siderolink-wireguard-endpoint' flags as required
* [`4a9d9ad`](https://github.com/siderolabs/omni/commit/4a9d9adef1e521d3c0293b6dc414f572bd8a93d4) feat: add the ClusterMachineStatus resource
* [`e4e8b62`](https://github.com/siderolabs/omni/commit/e4e8b6264cb48edd014f97129f52aefaa129fd63) refactor: unify all Arges API under a single HTTP server
* [`5af9049`](https://github.com/siderolabs/omni/commit/5af9049bdc2e09bf410e1b0646e4e08a4366f33b) chore: rename sidebar item
* [`a4fc47f`](https://github.com/siderolabs/omni/commit/a4fc47f97d79259532b91a8d391e84b59554ed8e) chore: fix build warning
* [`547b83c`](https://github.com/siderolabs/omni/commit/547b83c4a2a543d5b6ce4dca6cf6f5de87c33dcb) chore: bump siderolink version
* [`11c31f3`](https://github.com/siderolabs/omni/commit/11c31f39d834e3352b086c1aec665065fd74e944) refactor: drop one of the layered gRPC servers
* [`0adbbb7`](https://github.com/siderolabs/omni/commit/0adbbb7edfeacedd98a7e84c2f45ac458750a281) feat: introduce a way to copy kernel arguments from the UI
* [`ce5422a`](https://github.com/siderolabs/omni/commit/ce5422a27771a94cc25be70ec756711d140b2758) fix: import new COSI library to fix YAML marshaling
* [`d6cec09`](https://github.com/siderolabs/omni/commit/d6cec099cb6f4c3118e4263b9517176858bb9cfb) feat: implement Arges API client, and minimal `argesctl`
* [`65c8d68`](https://github.com/siderolabs/omni/commit/65c8d683187d82dc730752294c1bc03657f5df78) feat: implement cluster creation view
* [`8365b00`](https://github.com/siderolabs/omni/commit/8365b00df90ac55f99e0f82e1fa6d4367ebd6a3f) feat: re-enable old Theila UI
* [`63e703c`](https://github.com/siderolabs/omni/commit/63e703c4e1dfb4bf645fbc9cd28ba2a722e04dc2) fix: update Talos to the latest master
* [`d33e27b`](https://github.com/siderolabs/omni/commit/d33e27b49113729c5538fce688832152ff96a7ea) feat: implement clusters list view
* [`cb9e23c`](https://github.com/siderolabs/omni/commit/cb9e23ca6f420ac7b71acf6b19e9012265f3c69b) feat: protect Theila state from external API access
* [`952c235`](https://github.com/siderolabs/omni/commit/952c2359b32fdd077d85e312707f8b9c9e01ea0c) fix: properly allocated ports in the loadbalancer
* [`a58c479`](https://github.com/siderolabs/omni/commit/a58c479e9e31f70e806a1f3482b9b984c5c0ca68) chore: report siderolink events kernel arg
* [`8a56fe3`](https://github.com/siderolabs/omni/commit/8a56fe34ce1966fe28f9e432c696fdd779dfb638) refactor: move Theila resources to public `pkg/`
* [`1251699`](https://github.com/siderolabs/omni/commit/12516996eda859db6677403ad1f72a3994ea180b) fix: reset the `MachineEventsSnapshot` after the node is reset
* [`9a2e6af`](https://github.com/siderolabs/omni/commit/9a2e6af3113b795f57c4e3a86c1348b120fa3bbd) feat: implement bootstrap controller
* [`7107e27`](https://github.com/siderolabs/omni/commit/7107e27ee6b9ba644fc803e4463cbfcf26cf97de) feat: implement apply and reset config controller
* [`1579eb0`](https://github.com/siderolabs/omni/commit/1579eb09eb58f2cb679205e9e204369f3a362e07) feat: implement machine events handler and `ClusterStatus`
* [`7214f4a`](https://github.com/siderolabs/omni/commit/7214f4a514a921d6b9df7515116613996416f383) feat: implement cluster load balancer controller
* [`9c4fafa`](https://github.com/siderolabs/omni/commit/9c4fafaf6b8dc9b7ff08fe28704ca6a2e7efc097) feat: add a controller that manages load balancers for talos clusters
* [`7e3d80c`](https://github.com/siderolabs/omni/commit/7e3d80ce956d621ed79e4db094808831e18db85b) feat: add a resources that specify configurations for load balancers
* [`dc0d356`](https://github.com/siderolabs/omni/commit/dc0d356a181b4c37670d2ed4e8d7af370dccef60) feat: support Theila runtime watch with label selectors
* [`6a568a7`](https://github.com/siderolabs/omni/commit/6a568a72922e34e91f5448d3c1caa2f0b3a02e96) feat: implement `ClusterMachineConfig` resource and it's controller
* [`3db0f1c`](https://github.com/siderolabs/omni/commit/3db0f1c9d4e2d6f962b6f3216a4f9c7e2575dd21) feat: implement `TalosConfig` controller
* [`b7ae8e1`](https://github.com/siderolabs/omni/commit/b7ae8e113dc68acd87c4cfe5e3c8349d32bc392d) feat: introduce `Cluster` controller that adds finalizers on Clusters
* [`8d7ea02`](https://github.com/siderolabs/omni/commit/8d7ea0293e8f57388fd483dc82e79e6b4c76a53f) chore: use label selectors in `TalosConfig`, set labels on the resources
* [`cff9cb1`](https://github.com/siderolabs/omni/commit/cff9cb19ba8718fdad509b5e91cb8221c6c1ff00) fix: separate advertised endpoint from the actual wireguard endpoint
* [`5be6cc3`](https://github.com/siderolabs/omni/commit/5be6cc391adf8bcb58b8d47f09dad5aa75d1ad98) feat: implement cluster creation UI
* [`a1633eb`](https://github.com/siderolabs/omni/commit/a1633eb18772b9e99d687dfddd12fc09fd1ea5c4) chore: add typed wrappers around State, Reader and Writer
* [`5515f3d`](https://github.com/siderolabs/omni/commit/5515f3d004f54455a1eb1f4977bbb9d663fd1bca) feat: add `ClusterSecrets` resource and controller and tests
* [`7226f6c`](https://github.com/siderolabs/omni/commit/7226f6cdc60eeb4d6040d1aa0711fed378c50b33) feat: add `Cluster`, `ClusterMachine` and `TalosConfig` resources
* [`ec44930`](https://github.com/siderolabs/omni/commit/ec44930672ca8954c6ba68975c1799a087ec0c43) feat: enable vtprotobuf optimized marshaling
* [`15be219`](https://github.com/siderolabs/omni/commit/15be2198872fb637f7ba2e1ff550e4466179f2b1) feat: generate TS constants from go `//tsgen:` comments
* [`caa4c4d`](https://github.com/siderolabs/omni/commit/caa4c4d285dcd1176a70d87f28ee303cd0483ca8) fix: resource equality for proto specs
* [`beeca88`](https://github.com/siderolabs/omni/commit/beeca886213332f313f7f3a477d7e7c508e6d058) refactor: clarify code that creates or gets links for nodes
* [`340c63a`](https://github.com/siderolabs/omni/commit/340c63ad4ba918d4b11ab1f57fdbd3b5e5d8b3dc) feat: implement `Machines` page
* [`f7bc0c6`](https://github.com/siderolabs/omni/commit/f7bc0c69c69fe515cfa729bc062c730756a53019) feat: accept nodes if they provide the correct join token
* [`bdf789a`](https://github.com/siderolabs/omni/commit/bdf789a35da5491a4fcbd2af35a1c6efd22ab1fc) feat: immediately reconnect SideroLink peers after Arges restart
* [`6b74fa8`](https://github.com/siderolabs/omni/commit/6b74fa82ca5757d6f3809853c1ac3e7754efb06d) feat: implement MachineStatusController
* [`f5db0e0`](https://github.com/siderolabs/omni/commit/f5db0e05a87d5c11b4a1029b14020b19ca67035d) feat: add more info to the siderolink connection spec
* [`d3e4a71`](https://github.com/siderolabs/omni/commit/d3e4a71af8fd79328e4edda6d9642b83902b2003) refactor: simplify the usage of gRPC resource CRUD API
* [`2430115`](https://github.com/siderolabs/omni/commit/2430115af1aaac4226b7d5821e1fe706a1088501) feat: implement MachineController and small fixes
* [`e31d22d`](https://github.com/siderolabs/omni/commit/e31d22d7639753df53c130461ae1f96b9126f3a5) feat: support running Theila without contexts
* [`a6b3646`](https://github.com/siderolabs/omni/commit/a6b364626bd808687d5ad95307766344b16dd042) refactor: small fixes
* [`33d2b59`](https://github.com/siderolabs/omni/commit/33d2b59c202f03785580209c885aa297c023fa60) refactor: clean up a bit SideroLink code, fix shutdown
* [`98ec883`](https://github.com/siderolabs/omni/commit/98ec8830308755c7073a5d4510483e97d8e1d02d) chore: rename main executable to avoid clashing with Theila project
* [`828721d`](https://github.com/siderolabs/omni/commit/828721d9aa5d912cce628256f75579309d1ad67d) feat: enable COSI persistence for resources
* [`f1f7883`](https://github.com/siderolabs/omni/commit/f1f788344254e18bcab00a25b56a86289bfb1638) feat: set up siderolink endpoints in Theila
* [`6439335`](https://github.com/siderolabs/omni/commit/64393353ca7cf430f82bfe73a004da319da28261) refactor: migrate to `typed.Resource` in Theila internal state
* [`6195274`](https://github.com/siderolabs/omni/commit/61952742a47ea89e89228f057d0d3de351766150) refactor: restructure folders in the project
* [`1abf72b`](https://github.com/siderolabs/omni/commit/1abf72b4b2e382fe0cf9302b42242152c255a3ee) chore: update Talos libs to the latest version
* [`16dffd9`](https://github.com/siderolabs/omni/commit/16dffd9058570477b3a648896a89e6445e5b0162) fix: display delta time for pod's age
* [`8b80726`](https://github.com/siderolabs/omni/commit/8b807262b23cfa830f3ff444d49f11b3a1654703) feat: update favicon to sidero logo
* [`2da7378`](https://github.com/siderolabs/omni/commit/2da737841c2ae0bf1f1f916dc6f45b1e3996d6e4) feat: show the extended hardware info
* [`d3c6004`](https://github.com/siderolabs/omni/commit/d3c6004f9767bf0cff9191dc130308c848ede077) chore: allow getting resources without version and group
* [`eb19087`](https://github.com/siderolabs/omni/commit/eb190875b30275195e52f1a95ed0bb3aae08424f) fix: remove t-header error notification
* [`5a28202`](https://github.com/siderolabs/omni/commit/5a28202c939ef9683d14fb3d873e0bacb35577db) feat: restyle t-alert component
* [`9f2b482`](https://github.com/siderolabs/omni/commit/9f2b48228bbfa39d33b07ae43e9fdb34192c3eed) fix: get rid of racy code in the kubeconfig request code
* [`c40824e`](https://github.com/siderolabs/omni/commit/c40824ecc5d10cb5289e133b8b1f51213aa12f7f) feat: add text Highlight feature
* [`9018c81`](https://github.com/siderolabs/omni/commit/9018c81bd0d7c58bb5c632c06f3c3904f6674e03) feat: use `~/.talos/config` as a primary source for clusters
* [`e10547b`](https://github.com/siderolabs/omni/commit/e10547b5761ad96ab8b5766fe5c3f06fcdf86477) refactor: remove old components and not used code parts
* [`f704684`](https://github.com/siderolabs/omni/commit/f7046846ea8e83a0e39647c4fcc49addf4c56061) fix: properly calculate servers capacity
* [`755a077`](https://github.com/siderolabs/omni/commit/755a0779014b0a4177e0fc5180db20720be5a814) fix: use proper units for memory and CPU charts on the node monitor page
* [`d0a083d`](https://github.com/siderolabs/omni/commit/d0a083d1c15c319e236dd258fabcc9a231f797a1) release(v0.2.0-alpha.0): prepare release
* [`53878ee`](https://github.com/siderolabs/omni/commit/53878eea09c18f2bc0dd55ca11a6743587748319) fix: properly update servers menu item when the context is changed
* [`b4cb9c7`](https://github.com/siderolabs/omni/commit/b4cb9c7989ec5299785b86acb3fa0ee648efd259) feat: restyle TMonitor page
* [`f0377e2`](https://github.com/siderolabs/omni/commit/f0377e2ad5da702af71f2706141f4d7c638c7a15) fix: invert chart value for cpu, storage and memory on the overview page
* [`6ea6ecf`](https://github.com/siderolabs/omni/commit/6ea6ecf12c4d8b5253b4dfc2e64f5b5d787d022a) fix: update capi-utils to fix talosconfig requests for CAPI clusters
* [`e3796d3`](https://github.com/siderolabs/omni/commit/e3796d3876d33248fd0998901273a14d29a487a3) chore: update capi-utils
* [`39186eb`](https://github.com/siderolabs/omni/commit/39186ebe50da531f35d21ac2488f8a58c1ef8e78) feat: implement overview page, cluster dropdown, ongoing tasks
* [`59f2b27`](https://github.com/siderolabs/omni/commit/59f2b27be4d7f5a591fdeae533d649494356250d) docs: update README.md
* [`2b7831f`](https://github.com/siderolabs/omni/commit/2b7831f2d22106ac8a82f890d73c2705841b0739) feat: add Kubernetes and Servers pages
* [`4451a5b`](https://github.com/siderolabs/omni/commit/4451a5bc9f5c6b058c6bcf1252b7c83a001cafbe) fix: properly set TaskStatus namespace in the initial call
* [`4545464`](https://github.com/siderolabs/omni/commit/454546425f2fd7e4418aa8a03465f3a062de804e) fix: add new fields to the TaskStatus spec, update Talos
* [`891cf3b`](https://github.com/siderolabs/omni/commit/891cf3b79c8430deeed8a168955afd6e97083baa) docs: describe client context types, usage
* [`309b515`](https://github.com/siderolabs/omni/commit/309b51545ead2ee144244591df2e5ead2849fb11) feat: update k8s upgrades tasks structure for the new UI representation
* [`5aa8ca2`](https://github.com/siderolabs/omni/commit/5aa8ca24bd3159879c46c8e8a134702b174e3362) feat: add NodesPage
* [`db434e0`](https://github.com/siderolabs/omni/commit/db434e07b9f23562bd746a0f78e3868b079006e2) feat: add TPagination component
* [`0b51727`](https://github.com/siderolabs/omni/commit/0b51727efed31f13f52fa20b360071e7e2a6d9eb) feat: add Pods, Dashboard, Upgrade views, etc
* [`c549b8b`](https://github.com/siderolabs/omni/commit/c549b8b9ee8a563f14b2e791f91a7b3cb0430aa7) feat: add Overview and Upgrade Kubernetes pages
* [`cec2e85`](https://github.com/siderolabs/omni/commit/cec2e854f4f3999109220902bccaee6c25d1f502) chore: define constants for all used resource types
* [`962bdaf`](https://github.com/siderolabs/omni/commit/962bdaf6406ab8e5febea0ad8d32da9c86fa39e7) feat: add TSideBar
* [`fa28ccb`](https://github.com/siderolabs/omni/commit/fa28ccb67f52c1dd9096b23388427d78be526275) feat: add TheHeader component
* [`f3418a5`](https://github.com/siderolabs/omni/commit/f3418a59e38e551bd0be7cc7ae66ef4645719aa7) feat: button;icons;config
* [`db30f50`](https://github.com/siderolabs/omni/commit/db30f503730bdbd8ed359d4070dea0214df67fcd) fix: add `frontend/node_modules` to gitignore
* [`a675b86`](https://github.com/siderolabs/omni/commit/a675b86f7d55cecd4ae1277cbf057a6bc264940c) fix: properly pass label selector to the metadata in ClusterListItem
* [`7911d6a`](https://github.com/siderolabs/omni/commit/7911d6a31abdb51e86586a025b705ddfeb1dd19e) chore: add ability to start local development server for the frontend
* [`076fee1`](https://github.com/siderolabs/omni/commit/076fee10c6583dc49e6530b02cab1f757da0e853) feat: use CAPI utils for CAPI requests
* [`5ed5ba2`](https://github.com/siderolabs/omni/commit/5ed5ba2a122585a97cf65c3ff081126752cd26fa) fix: more websocket client bugfixes
* [`6fe22ad`](https://github.com/siderolabs/omni/commit/6fe22ad370026380ba75b38e261870addc341e6f) fix: reset reconnect timeouts after the client is reconnected
* [`c4b144a`](https://github.com/siderolabs/omni/commit/c4b144af272a46dbdc8d1bb35784e09ba1b79987) fix: talosconfig/kubeconfig when using the default context
* [`b439a37`](https://github.com/siderolabs/omni/commit/b439a371c13a8d46d986a1dae3d6f4b7cba4a298) fix: properly handle Same-Origin header in websockets
* [`ffffed1`](https://github.com/siderolabs/omni/commit/ffffed100cec18209bae723b9919eb8613950649) fix: read node name from nodename resource instead of hostname
* [`2d6f984`](https://github.com/siderolabs/omni/commit/2d6f9844440a6d18b3093dea6228ac6a237dc86b) fix: use secure websockets if the page itself is using https
* [`799f2d2`](https://github.com/siderolabs/omni/commit/799f2d2d00762d5270dd4a3f4b4b312b32dbb7dd) feat: rework the node overview page
* [`0d0eaf4`](https://github.com/siderolabs/omni/commit/0d0eaf4b2721dfa1b04bce24e4a1e476579e3a74) fix: make charts height resize depending on the screen height
* [`7de0101`](https://github.com/siderolabs/omni/commit/7de0101bf0e613653caadd5733db0e29a6bb5bfb) fix: use polyfill to fix streaming APIs on Firefox
* [`0cff2b0`](https://github.com/siderolabs/omni/commit/0cff2b02b5d8b2c2c644067cf6bd3ed573cb784d) feat: small UI adjustments
* [`d70bd41`](https://github.com/siderolabs/omni/commit/d70bd41992e13fb3dacc1740532083a8f6ce9afa) feat: implement accept Sidero server functional
* [`f3a6e16`](https://github.com/siderolabs/omni/commit/f3a6e16a79e1bca9ea6c87eb0d3e0f2a6c65ff2e) feat: add top processes list to the Overview page
* [`3cf97e4`](https://github.com/siderolabs/omni/commit/3cf97e4b9e07f8383da8a6fb7a993b70c8f82503) refactor: use the same object for gRPC metadata context and messages
* [`243206f`](https://github.com/siderolabs/omni/commit/243206f95aa6ba944bd4361db6274e7072bae1fc) release(v0.1.0-alpha.2): prepare release
* [`e5b6f29`](https://github.com/siderolabs/omni/commit/e5b6f29fd298904e06284a67681cc0ce5135145f) feat: implement node Reset
* [`bcb7d23`](https://github.com/siderolabs/omni/commit/bcb7d237c31f42a35f5c3b53e7615ddae1ce0a8b) fix: node IP not being truncated
* [`e576d33`](https://github.com/siderolabs/omni/commit/e576d33ba40f629eed14668f2d9bf77d7fef62c2) feat: add upgrade UI for CAPI clusters
* [`10cdce7`](https://github.com/siderolabs/omni/commit/10cdce7fcc219af969a85a41d18fb904936faa0a) fix: server labels key/value order and chevron orientation
* [`4007177`](https://github.com/siderolabs/omni/commit/40071775d6de1eea697f67e55441c384c86e75d9) feat: implement Kubernetes upgrade UI components
* [`f4917ee`](https://github.com/siderolabs/omni/commit/f4917eecfb3173acf7518883c738118c8537d657) fix: accumulate chart updates into a single update
* [`414d76c`](https://github.com/siderolabs/omni/commit/414d76c1c926695e5d66787b34decae92e151b45) feat: implement upgrade controller
* [`36742ea`](https://github.com/siderolabs/omni/commit/36742ea5ab1e8a983b73f73443c1cf122a90d054) feat: introduce create, delete and update gRPC APIs
* [`2b3d314`](https://github.com/siderolabs/omni/commit/2b3d314a460b385d8c13bdd025fadb37b5508bdc) feat: install internal COSI runtime alongside with K8s and Talos
* [`ae7f784`](https://github.com/siderolabs/omni/commit/ae7f784d08621d18075b1763f026a7513d9d9dcb) refactor: move all generated TypeScript files under `frontend/src/api`
* [`61bad64`](https://github.com/siderolabs/omni/commit/61bad64540c28fb0520a39a6c64d64c3e9353361) release(v0.1.0-alpha.1): prepare release
* [`8e5e722`](https://github.com/siderolabs/omni/commit/8e5e7229470713d2fbd5ad0df027bd825f5481e3) feat: implement node reboot controls
* [`9765a88`](https://github.com/siderolabs/omni/commit/9765a88069f05c49f5a7d854675ee37e1c7a8273) feat: dmesg logs page
* [`ecbbd67`](https://github.com/siderolabs/omni/commit/ecbbd67936b1fb570d706fe3b93b81f6089b5124) feat: use updated timestamp to display event time on the graph
* [`7c56773`](https://github.com/siderolabs/omni/commit/7c56773448a496fe1ceeec3c47978975ce336b3a) refactor: use Metadata to pass context in all gRPC calls
* [`abb4733`](https://github.com/siderolabs/omni/commit/abb47330222217d7d8b5c36ff28902415bc755d8) feat: implement service logs viewer
* [`8e8e032`](https://github.com/siderolabs/omni/commit/8e8e032b20d082bfd71a26c2af2bbc821d9c2a7b) feat: add ability to pick sort order on the servers page
* [`1a1c728`](https://github.com/siderolabs/omni/commit/1a1c728ac929bb02db7f1bd0b991a747e63fe81a) fix: resolve the issue with idFn value generating undefined ids
* [`2e83fe2`](https://github.com/siderolabs/omni/commit/2e83fe23a7feb51b73bc7b53997636b641ae42b9) feat: allow filtering servers by picking from predefined categories
* [`48f776e`](https://github.com/siderolabs/omni/commit/48f776e10f6c79772481393d7397557419520046) fix: navigate home when changing the context
* [`a1ce0ca`](https://github.com/siderolabs/omni/commit/a1ce0ca8c8fabb2267c3dc6f6b1509f131e18ba8) fix: resolve services search issues
* [`5b768f8`](https://github.com/siderolabs/omni/commit/5b768f85277ee31131994ae0b253700a5d26978d) feat: make stacked lists searchable
* [`ec1bc5b`](https://github.com/siderolabs/omni/commit/ec1bc5b48943e473c756ebc7a8c943a34cdeaeac) feat: implement stats component and add stats to the servers page
* [`1a85999`](https://github.com/siderolabs/omni/commit/1a8599981f93fc5ce68e23b1b4cd7aabbb43c90c) feat: align Sidero servers list outlook with the wireframes
* [`524264c`](https://github.com/siderolabs/omni/commit/524264c515a9efdce9f06a3c2ebd59c2979f9b2a) fix: display error message and use proper layout for the spinner
* [`5263d16`](https://github.com/siderolabs/omni/commit/5263d16cfb936aad9ba461e0cc7b150ff9b806d5) feat: introduce node stats page
* [`8feb35e`](https://github.com/siderolabs/omni/commit/8feb35e95a6d588e1d9c605231308976be452a2e) feat: make root sidebar sections collapsible
* [`36ad656`](https://github.com/siderolabs/omni/commit/36ad656a3bbdc1e2915a87c0d09c31738ae3f3c4) feat: detect cluster capabilities
* [`a25d90d`](https://github.com/siderolabs/omni/commit/a25d90d58a85b3b73432858f134fa09cd1338d5c) feat: support switching context in the UI
* [`67903e2`](https://github.com/siderolabs/omni/commit/67903e23f49623ae9a9a6b297282c62aa8579aa8) refactor: separate Watch from StackedList
* [`76b9e1d`](https://github.com/siderolabs/omni/commit/76b9e1dc88cccf74cebb28470eae5e9249809d40) release(v0.1.0-alpha.0): prepare release
* [`7bde4c8`](https://github.com/siderolabs/omni/commit/7bde4c8c6e16c197578cbb4e037a05d50194958f) fix: cobra command was initialized but not actually used
* [`04624c9`](https://github.com/siderolabs/omni/commit/04624c95cec587ae0b0d8888d95d484ef8d98cfa) feat: support getting Talos and Kubernetes client configs for a cluster
* [`219b9c8`](https://github.com/siderolabs/omni/commit/219b9c8663fe03af65796b0b6299cff5e66b3efc) feat: implement notifications component
* [`f8b19a0`](https://github.com/siderolabs/omni/commit/f8b19a0585e6e19c0e7da4e4afad5bbd264e0029) feat: decouple watch list from the view
* [`2f8c96e`](https://github.com/siderolabs/omni/commit/2f8c96e44012e7bd0db9869eeb90ab48ff41e162) feat: implement appearance settings modal window
* [`de745d6`](https://github.com/siderolabs/omni/commit/de745d6b7170a9c509cc835a8b675a1c788e80f4) feat: implement Talos runtime backend
* [`af69a0d`](https://github.com/siderolabs/omni/commit/af69a0d58906a86974bc7dbec2c09ca9f78b152f) feat: support getting Kubernetes resource through gRPC gateway
* [`2c50010`](https://github.com/siderolabs/omni/commit/2c50010b0d9f7b168354fedd698600d94123c354) feat: implement breadcrumbs component, add support for table header
* [`3fc1e80`](https://github.com/siderolabs/omni/commit/3fc1e808875f6f502cd2657c4548dd886fbf465d) feat: implement nodes view
* [`961e93a`](https://github.com/siderolabs/omni/commit/961e93a4af430eaa9efcd1e2922af8072fe4cf85) feat: implement clusters view
* [`e8248ff`](https://github.com/siderolabs/omni/commit/e8248ffab89633cae8834631e39cf4dce5e4147a) feat: use plain zap instead of SugaredLogger everywhere
* [`81ba93d`](https://github.com/siderolabs/omni/commit/81ba93dffdc37efdde06557a1c63511a7d61b2f2) chore: generate websocket protocol messages using protobuf
* [`37a878d`](https://github.com/siderolabs/omni/commit/37a878dd396b650df8afaf6730f9afe52d35569c) feat: make JS websocket reconnect on connection loss
* [`23b3281`](https://github.com/siderolabs/omni/commit/23b3281f8880800a9084e1c8a74617fcf966c846) feat: use dynamic watcher to allow listing any kinds of resources
* [`16475f5`](https://github.com/siderolabs/omni/commit/16475f51cc9651736213b36c57381b24dcabdc62) feat: implement real time update server on top of web sockets
* [`76b39ae`](https://github.com/siderolabs/omni/commit/76b39ae563d9f09ecac3451389e3d260abdad48d) feat: create hello world Vue app using Kres
* [`baab493`](https://github.com/siderolabs/omni/commit/baab493f155cbd78c2e8af6ce45268c40ef6aeed) Initial commit
</p>
</details>

### Changes since v0.1.0-alpha.1
<details><summary>55 commits</summary>
<p>

* [`e096c88`](https://github.com/siderolabs/omni/commit/e096c887604399028a559e33da13653c1f54965d) chore: add resource operation metrics
* [`741e820`](https://github.com/siderolabs/omni/commit/741e8202c5aecfe171082c38e2c55e0184e9c80c) feat: implement config patch creation UI
* [`5def267`](https://github.com/siderolabs/omni/commit/5def26706fa21df7748801cbdab5c6e81543174f) fix: attempt to clean up docker container better
* [`876ff5e`](https://github.com/siderolabs/omni/commit/876ff5ee44d4193c52e4daeec776ad50b69664f9) feat: update COSI and state-etcd to 0.2.0
* [`3df410d`](https://github.com/siderolabs/omni/commit/3df410d964fc66b2d4ad8c7db0459108d16adde0) test: refactor and update config patch integration tests
* [`5eea9e5`](https://github.com/siderolabs/omni/commit/5eea9e50b47a6df324f2fd5564aa9010b56e16e0) feat: add TLS support to siderolink API
* [`36394ea`](https://github.com/siderolabs/omni/commit/36394ea242f9af4d9c17f90ec143b0356fa9e671) refactor: simplify the resource leak fix
* [`e5b962b`](https://github.com/siderolabs/omni/commit/e5b962b66f158fd31b74dc6b97f524c168b4fad1) chore: update dev environment
* [`39bf206`](https://github.com/siderolabs/omni/commit/39bf206eec29262b1c15ed557f7f24e029c61206) fix: save user picture and fullname in the local storage
* [`f1611c1`](https://github.com/siderolabs/omni/commit/f1611c10d26b937b5bae69a1b9eda67d2bc5e137) feat: add machine level config patch support
* [`f2e6cf5`](https://github.com/siderolabs/omni/commit/f2e6cf5cddb47aaa290e7db1a037f2155fcd60d2) fix: remove several resource/goroutine leaks
* [`fc37af3`](https://github.com/siderolabs/omni/commit/fc37af36d87e01c3e9f349f206711f154740e0b4) feat: allow destroying config patches in the UI
* [`3154d59`](https://github.com/siderolabs/omni/commit/3154d591e7c65713c6940d953df45d8242ae9359) fix: respect SIDEROLINK_DEV_JOIN_TOKEN only in debug mode
* [`38f5380`](https://github.com/siderolabs/omni/commit/38f53802ab3dda70fedc0a81de9d6dd43e6204f1) feat: avoid deleting all resources on omnictl delete
* [`28666bc`](https://github.com/siderolabs/omni/commit/28666bcb4acaf6e4f053e99d8d45d5dae320c89c) chore: add support for local development using compose
* [`cad73ce`](https://github.com/siderolabs/omni/commit/cad73cefc6b187a26e3833089e89ca1cb6fbf843) chore: increase TestEtcdAudit timeout and fix incorrect `Assert()` calls.
* [`7199b75`](https://github.com/siderolabs/omni/commit/7199b75c2108568d8bee82c42fcc00edb4a22e1c) chore: during `config merge` create config if there was none
* [`dab54d1`](https://github.com/siderolabs/omni/commit/dab54d14fcd8c0fadc6bb2a49d79e90379234403) chore: increase `TestTalosBackendRoles` reliability
* [`997cd78`](https://github.com/siderolabs/omni/commit/997cd7823bd126302ed4772658c0791768d67638) feat: add reconfiguring phase to machinesetstatus
* [`81fb2b9`](https://github.com/siderolabs/omni/commit/81fb2b94e61f7e7aaf41075fe17a2bbfea005d9f) fix: fix button order and vue config
* [`252fb29`](https://github.com/siderolabs/omni/commit/252fb29d64dac660da08459d9c5acc44e457b034) refactor: simplify backend.Server.Run method
* [`f335c2f`](https://github.com/siderolabs/omni/commit/f335c2f5311a81ca23699c473b68bf6918430aab) refactor: split watch to `Watch` and `WatchFunc`, add unit tests
* [`35a7919`](https://github.com/siderolabs/omni/commit/35a79193b965d42fba0a649bef0efe82abbd2fd5) feat: track machine config apply status
* [`1c54710`](https://github.com/siderolabs/omni/commit/1c54710c6f5ebe2740af27cebfb9c5532b22cc26) fix: use rolling update strategy on control planes
* [`17ccdc2`](https://github.com/siderolabs/omni/commit/17ccdc2f78693b5d1276b843c027e8057faa2ff7) refactor: various logging fixes
* [`3c9ca9c`](https://github.com/siderolabs/omni/commit/3c9ca9cd83298c5281c7ced50720b341c10a02f0) fix: update node overview Kubernetes node watch to make it compatible
* [`e8c2063`](https://github.com/siderolabs/omni/commit/e8c20631501308952bbc596e994a71b7677034b3) fix: enable edit config patches button on the cluster overview page
* [`6e80521`](https://github.com/siderolabs/omni/commit/6e8052169dd672e6fce5668982b704331eac4645) fix: reset the item list after the watch gets reconnected
* [`620d197`](https://github.com/siderolabs/omni/commit/620d1977a70bbc2cca8b331db825fc7bdb8fcda3) chore: remove AddContext method from runtime.Runtime interface
* [`8972ade`](https://github.com/siderolabs/omni/commit/8972ade40dea2bf3bf41bcb865a817d90b37657d) chore: update default version of Talos to v1.2.7
* [`6a2dde8`](https://github.com/siderolabs/omni/commit/6a2dde863d306986027904167f262d4307a7420d) fix: update the config patch rollout strategy
* [`fb3f6a3`](https://github.com/siderolabs/omni/commit/fb3f6a340c37d1958e36400edf7ca53e2cde48a7) fix: skip updating config status if applying config caused a reboot
* [`8776146`](https://github.com/siderolabs/omni/commit/877614606d0c7d0259c4e65e4911f331550dd7d7) fix: apply finalizer to the `Machine` only when CMS is created
* [`134bb20`](https://github.com/siderolabs/omni/commit/134bb2053ce6250b9b4c647f3b2dbb8255cea2ce) test: fix config patch test with reboot
* [`d3b6b5a`](https://github.com/siderolabs/omni/commit/d3b6b5a75f9ea5304595851d6160e98ec4c9b8aa) feat: implement config patch viewer and editor
* [`149efe1`](https://github.com/siderolabs/omni/commit/149efe189a24c07e648289ee81d0b95ed1c972b7) chore: bump runtime and state-etcd modules
* [`c345b83`](https://github.com/siderolabs/omni/commit/c345b8348412aef59cbd43c35bf06ce3eac5ad3f) chore: output omnictl auth log to stderr
* [`39b2ba2`](https://github.com/siderolabs/omni/commit/39b2ba2a86972324161c6cff056abf10eb2fce5c) refactor: introduce ClusterEndpoint resource
* [`6998ff0`](https://github.com/siderolabs/omni/commit/6998ff0803063b22e113da0c72356ee254f13143) fix: treat created and updated events same
* [`289fe88`](https://github.com/siderolabs/omni/commit/289fe88aba94d6cfe4d7be7472b609232e45cbf6) feat: add omnictl apply
* [`2f1be3b`](https://github.com/siderolabs/omni/commit/2f1be3b4643e2a66a62da6a7f8f1f1da39ed6e17) chore: fix `TestGenerateJoinToken` test
* [`3829176`](https://github.com/siderolabs/omni/commit/382917630030415b1a218f14f2a1d6d3595834a0) fix: don't close config patch editor window if config validation fails
* [`c96f504`](https://github.com/siderolabs/omni/commit/c96f5041be7befb517998fc7bbccd135cb76908d) feat: add suspended mode
* [`b967bcf`](https://github.com/siderolabs/omni/commit/b967bcfd26b2fccfa6bbb08b8a15eb3796e2e872) feat: add last config apply error to clustermachineconfigstatus
* [`0395d9d`](https://github.com/siderolabs/omni/commit/0395d9dd7b985802be8f4cd2b8005b409faca3de) test: increase key generation timeout on storage signing test
* [`577eba4`](https://github.com/siderolabs/omni/commit/577eba4231142fe983f9a0f9b5a81280c377686e) fix: set SideroLink MTU to 1280
* [`0f32172`](https://github.com/siderolabs/omni/commit/0f32172922ed2f7b8b4b7433fb1f9ce104f3c5a8) fix: minor things in frontend
* [`9abcc7b`](https://github.com/siderolabs/omni/commit/9abcc7b444c49f6223e0ae4948bff13eedbb05b5) test: add config patching integration tests
* [`99531fb`](https://github.com/siderolabs/omni/commit/99531fbeee982e2ab87d9f0162a0080308b852ab) refactor: drop unneeded controller inputs
* [`5172354`](https://github.com/siderolabs/omni/commit/51723541621d91964e88e8a5add834159214dc5b) chore: add omnictl to the generated image
* [`738cf64`](https://github.com/siderolabs/omni/commit/738cf649f53ec29e88112a027ec72f3d6f0cfff8) fix: set cluster machine version in machine config status correctly
* [`1d0d220`](https://github.com/siderolabs/omni/commit/1d0d220f47f1cc9ca8b20bfef47004a875b7573c) fix: lower ttl of the issued keys on the FE side by 10 minutes
* [`2889524`](https://github.com/siderolabs/omni/commit/2889524f222e42d49061867b2b2f5b59a16af4ba) feat: dynamic title
* [`3d17bd7`](https://github.com/siderolabs/omni/commit/3d17bd7cfd4775292090ccb3fd3c2b575b26d449) chore: fix release CI run
* [`f2c752f`](https://github.com/siderolabs/omni/commit/f2c752fed627006912018ae3e5f2ff0f2bed60b8) fix: properly proxy watch requests through dev-server
</p>
</details>

### Dependency Changes

This release has no dependency changes

## [Omni 0.1.0-alpha.1](https://github.com/siderolabs/omni/releases/tag/v0.1.0-alpha.1) (2022-11-10)

Welcome to the v0.1.0-alpha.1 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Contributors

* Artem Chernyshev
* Andrey Smirnov
* Artem Chernyshev
* Dmitriy Matrenichev
* Philipp Sauter
* Utku Ozdemir
* evgeniybryzh
* Noel Georgi
* Andrew Rynhard
* Tim Jones
* Andrew Rynhard
* Gerard de Leeuw
* Steve Francis
* Volodymyr Mazurets

### Changes
<details><summary>349 commits</summary>
<p>

* [`8b284f3`](https://github.com/siderolabs/omni/commit/8b284f3aa26cf8a34452f33807dcc04045e7a098) feat: implement Kubernetes API OIDC proxy and OIDC server
* [`adad8d0`](https://github.com/siderolabs/omni/commit/adad8d0fe2f3356e97de613104196233a3b98ff5) refactor: rework LoadBalancerConfig/LoadBalancerStatus resources
* [`08e2cb4`](https://github.com/siderolabs/omni/commit/08e2cb4fd40ec918bf458edd6a5d8e6c86fe5c97) feat: support editing config patches on cluster and machine set levels
* [`e2197c8`](https://github.com/siderolabs/omni/commit/e2197c83e994afb435671f5af5cdefa843bbddb5) test: e2e testing improvements
* [`ec9051f`](https://github.com/siderolabs/omni/commit/ec9051f6dfdf1f5acaf3fa6766dc1195b6f6dcdd) fix: config patching
* [`e2a1d6c`](https://github.com/siderolabs/omni/commit/e2a1d6c78809eaa4168ca5ede433824797a6aa4e) fix: send logs in JSON format by default
* [`954dd70`](https://github.com/siderolabs/omni/commit/954dd70b935b7c373ba5830fd7ad6e965f6b0da8) chore: replace talos-systems depedencies with siderolabs
* [`acf94db`](https://github.com/siderolabs/omni/commit/acf94db8ac80fb6f15cc87ff276b7edca0cb8661) chore: add payload logger
* [`838c716`](https://github.com/siderolabs/omni/commit/838c7168c64f2296a9e01d3ef6ab4feb9f16aeb9) fix: allow time skew on validating the public keys
* [`dd481d6`](https://github.com/siderolabs/omni/commit/dd481d6cb3620790f6e7a9c8e305defb507cbe5f) fix: refactor runGRPCProxy in router tests to catch listener errors
* [`e68d010`](https://github.com/siderolabs/omni/commit/e68d010685d4f0a5d25fee671744119cecf6c27b) chore: small fixes
* [`ad86875`](https://github.com/siderolabs/omni/commit/ad86875ec146e05d7d7f461bf7c8094a8c143df5) feat: minor adjustments on the cluster create page
* [`e61f194`](https://github.com/siderolabs/omni/commit/e61f1943e965287c79fbaef05760bb0b0deee988) chore: implement debug handlers with controller dependency graphs
* [`cbbf901`](https://github.com/siderolabs/omni/commit/cbbf901e601d31c777ad2ada0f0036c57020ba96) refactor: use generic TransformController more
* [`33f9f2c`](https://github.com/siderolabs/omni/commit/33f9f2ce3ec0999198f311ae4bae9b58e57153c9) chore: remove reflect from runtime package
* [`6586963`](https://github.com/siderolabs/omni/commit/65869636aa33013b5feafb06e727b9d2a4cf1c19) feat: add scopes to users, rework authz & add integration tests
* [`bb355f5`](https://github.com/siderolabs/omni/commit/bb355f5c659d8c66b825de409d9446767005a2bb) fix: reload the page to init the UI Authenticator on signature fails
* [`c90cd48`](https://github.com/siderolabs/omni/commit/c90cd48eefa7f29328a456aa5ca474eece17c6fe) chore: log auth context
* [`d278780`](https://github.com/siderolabs/omni/commit/d2787801a4904fe895996e5319f301a1d7ca76df) fix: update Clusters page UI
* [`5e77607`](https://github.com/siderolabs/omni/commit/5e776072285e535e93c0458774dcad810b9b857a) tests: abort on first failure
* [`4c55980`](https://github.com/siderolabs/omni/commit/4c5598083ff6d8763c8763d8e46a3d7b659784ff) chore: get full method name from the service
* [`2194f43`](https://github.com/siderolabs/omni/commit/2194f4391607e6e73bce1917d2744e78fdd2cebc) feat: redesign cluster list view
* [`40b3f23`](https://github.com/siderolabs/omni/commit/40b3f23071096987e8a7c6f30a2622c317c190cb) chore: enable gRPC request duration histogram
* [`0235bb9`](https://github.com/siderolabs/omni/commit/0235bb91a71510cf4d349eedd3625b119c7e4e11) refactor: make sure Talos/Kubernetes versions are defined once
* [`dd6154a`](https://github.com/siderolabs/omni/commit/dd6154a45d5dcd14870e0aa3f97aa1d4e53bdcfb) chore: add public key pruning
* [`68908ba`](https://github.com/siderolabs/omni/commit/68908ba330ecd1e285681e24db4b9037eb2e8202) fix: bring back UpgradeInfo API
* [`f1bc692`](https://github.com/siderolabs/omni/commit/f1bc692c9125f7683fe5f234b03eb3521ba7e773) refactor: drop dependency on Talos Go module
* [`0e3ef43`](https://github.com/siderolabs/omni/commit/0e3ef43cfed68e53879e6c22b46e7d0568ddc05f) feat: implement talosctl access via Omni
* [`2b0014f`](https://github.com/siderolabs/omni/commit/2b0014fea15da359217f89ef723965dcc9faa739) fix: provide a way to switch the user on the authenticate page
* [`e295d7e`](https://github.com/siderolabs/omni/commit/e295d7e2854ac0226e7efda32864f6a687a88470) chore: refactor all controller tests to use assertResource function
* [`8251dfb`](https://github.com/siderolabs/omni/commit/8251dfb9e44341e9df9471f387cc76c91359cf84) refactor: extract PGP client key handling
* [`02da9ee`](https://github.com/siderolabs/omni/commit/02da9ee66f15462e6f4d7da18515651a5fde11aa) refactor: use extracted go-api-signature library
* [`4bc3db4`](https://github.com/siderolabs/omni/commit/4bc3db4dcbc14e0e51c7a3b5257686b671cc2823) fix: drop not working upgrade k8s functional
* [`17ca75e`](https://github.com/siderolabs/omni/commit/17ca75ef864b7a59f9c6f829de19cc9630a670c0) feat: add 404 page
* [`8dcde2a`](https://github.com/siderolabs/omni/commit/8dcde2af3ca49d9be16cc705c0b403826f2eee5d) feat: implement logout flow in the frontend
* [`ba766b9`](https://github.com/siderolabs/omni/commit/ba766b9922302b9d1f279b74caf94e6ca727f86f) fix: make `omnictl` correctly re-auth on invalid key
* [`fd16f87`](https://github.com/siderolabs/omni/commit/fd16f8743d3843e8ec6735a7c2e96532694b876e) fix: don't set timeout on watch gRPC requests
* [`8dc3cc6`](https://github.com/siderolabs/omni/commit/8dc3cc682e5419c3824c6e740a32085c386b8817) fix: don't use `omni` in external names
* [`2513661`](https://github.com/siderolabs/omni/commit/2513661578574255ca3f736d3dfa1f307f5d43b6) fix: reset `Error` field of the `MachineSetStatus`
* [`b611e99`](https://github.com/siderolabs/omni/commit/b611e99e14a7e2ebc64c55ed5c95a47e17d6ac32) fix: properly handle `Forbidden` errors on the authentication page
* [`8525502`](https://github.com/siderolabs/omni/commit/8525502265b10dc3cc056d301785f6f60e4f7e22) fix: stop runners properly and clean up StatusMachineSnapshot
* [`ab0190d`](https://github.com/siderolabs/omni/commit/ab0190d9a41b830daf60173b998acdbcbbdd3754) feat: implement scopes and enforce authorization
* [`9198d96`](https://github.com/siderolabs/omni/commit/9198d96ea9d57bb5949c59350aec42b2ce13ebac) feat: sign gRPC requests on the frontend to enable Authentication flow
* [`bdd8f21`](https://github.com/siderolabs/omni/commit/bdd8f216a9eca7ec657fa0dc554e663743f058d1) chore: remove reset button and fix padding
* [`362db57`](https://github.com/siderolabs/omni/commit/362db570349b4a2659f746ce18a436d684481ecb) fix: gRPC verifier should verify against original JSON payload
* [`30186b8`](https://github.com/siderolabs/omni/commit/30186b8cfe2eea6eaade8bacf31114886d3da3ea) fix: omnictl ignoring omniconfig argument
* [`e8ab0ba`](https://github.com/siderolabs/omni/commit/e8ab0ba45648b8f521500b46fe032797da6a111f) fix: do not attempt to execute failed integration test again
* [`9fda25e`](https://github.com/siderolabs/omni/commit/9fda25ef45f0060cc6c3ec812f5fa1c7b1015801) chore: add more info on errors to different controllers
* [`ccda526`](https://github.com/siderolabs/omni/commit/ccda5260c4645b5929724574a9f856eeaa4c232f) chore: bump grpc version
* [`b1ac125`](https://github.com/siderolabs/omni/commit/b1ac1255da5ca4b5d9c409e27c51e4298275e73c) chore: emit log when we got machine status event.
* [`005d257`](https://github.com/siderolabs/omni/commit/005d257c25c745b61e5a25c39167d511710562c7) chore: set admin role specifically for Reboot request.
* [`27f0e30`](https://github.com/siderolabs/omni/commit/27f0e309cec76a454e5bb24c2df1e62d9e4718e0) chore: update deps
* [`77f0219`](https://github.com/siderolabs/omni/commit/77f02198c1e7fb215548f3a0e2be30a0e19aaf6d) test: more unit-tests for auth components
* [`0bf6ddf`](https://github.com/siderolabs/omni/commit/0bf6ddfa46e0ea6ad255ede00a600c390344e221) fix: pass through HTTP request if auth is disabled
* [`4f3a67b`](https://github.com/siderolabs/omni/commit/4f3a67b08e03a1bad65c2acb8d65f0281fdd2f9e) fix: unit-tests for auth package and fixes
* [`e3390cb`](https://github.com/siderolabs/omni/commit/e3390cbbac1d0e78b72512c6ebb64a8f53dcde17) chore: rename arges-theila to omni
* [`14d2614`](https://github.com/siderolabs/omni/commit/14d2614538ec696d468a0850bd4ee7bc6884c3b1) chore: allow slashes in secretPath
* [`e423edc`](https://github.com/siderolabs/omni/commit/e423edc072714e7f693249b60079f5f700cc0a65) fix: add unit-tests for auth message and fix issues
* [`b5cfa1a`](https://github.com/siderolabs/omni/commit/b5cfa1a84e93b6bbf5533c599917f293fc5cdf66) feat: add vault client
* [`b47791c`](https://github.com/siderolabs/omni/commit/b47791ce303cbb9a8aab279685d17f92a480c7f4) feat: sign grpc requests on cli with pgp key & verify it on server
* [`d6ef4d9`](https://github.com/siderolabs/omni/commit/d6ef4d9c36758cb0091e2c528b848952f312941a) feat: split account ID and name
* [`e412e1a`](https://github.com/siderolabs/omni/commit/e412e1a69edad0d19d7e46fa3aa076dcb8e6d4b6) chore: workaround the bind problem
* [`e23cc59`](https://github.com/siderolabs/omni/commit/e23cc59bb8cb8f9df81738d4c58aed08d80fa9c4) chore: bump minimum Talos version to v1.2.4
* [`0638a29`](https://github.com/siderolabs/omni/commit/0638a29d78c092641573aa2b8d2e594a7ff6aab4) feat: stop using websockets
* [`8f3c19d`](https://github.com/siderolabs/omni/commit/8f3c19d0f0ecfbe5beabc7dc508dcafa720e83e2) feat: update install media to be identifiable
* [`70d1e35`](https://github.com/siderolabs/omni/commit/70d1e354466618bb07c13445a16ca639be12009e) feat: implement resource encryption
* [`7653638`](https://github.com/siderolabs/omni/commit/76536386499889994b65f66a8a40f18b5535c5ba) fix: fix NPE in integration tests
* [`e39849f`](https://github.com/siderolabs/omni/commit/e39849f4047f028251123781bd8be350ebbfd65d) chore: update Makefile and Dockerfile with kres
* [`4709473`](https://github.com/siderolabs/omni/commit/4709473ec20fbf92a3240fb3376a322f1321103a) fix: return an error if external etcd client fails to be built
* [`5366661`](https://github.com/siderolabs/omni/commit/536666140556ba9b997a2b5d4441ea4b5f42d1c5) refactor: use generic transform controller
* [`a2a5f16`](https://github.com/siderolabs/omni/commit/a2a5f167f21df6375767d018981651d60bb2f768) feat: limit access to Talos API via Omni to `os:reader`
* [`e254201`](https://github.com/siderolabs/omni/commit/e2542013938991faa8f1c521fc524b8fcf31ea34) feat: merge internal/external states into one
* [`3258ca4`](https://github.com/siderolabs/omni/commit/3258ca487c818a34924f138640f44a2e51d307fb) feat: add `ControlPlaneStatus` controller
* [`1c0f286`](https://github.com/siderolabs/omni/commit/1c0f286a28f5134333130708d031dbfa11051a42) refactor: use `MachineStatus` Talos resource
* [`0a6b19f`](https://github.com/siderolabs/omni/commit/0a6b19fb916ea301a8f5f6ccd9bbdaa7cb4c39e0) chore: drop support for Talos resource API
* [`ee5f6d5`](https://github.com/siderolabs/omni/commit/ee5f6d58a2b22a87930d3c8bb9963f71c92f3908) feat: add auth resource types & implement CLI auth
* [`36736e1`](https://github.com/siderolabs/omni/commit/36736e14e5c837d38568a473834d14073b88a153) fix: use correct protobuf URL for cosi resource spec
* [`b98c56d`](https://github.com/siderolabs/omni/commit/b98c56dafe33beef7792bd861ac4e637fe13c494) feat: bump minimum version for Talos to v1.2.3
* [`b93bc9c`](https://github.com/siderolabs/omni/commit/b93bc9cd913b017c66502d96d99c52e4d971e231) chore: move containers and optional package to the separate module
* [`e1af4d8`](https://github.com/siderolabs/omni/commit/e1af4d8a0bee31721d8946ef452afe04da6b494d) chore: update COSI to v0.2.0-alpha.1
* [`788dd37`](https://github.com/siderolabs/omni/commit/788dd37c0be32745547ee8268aa0f004041dc96f) feat: implement and enable by default etcd backend
* [`1b83038`](https://github.com/siderolabs/omni/commit/1b83038b77cab87ffc2d4d73a91582785ed446ef) release(v0.1.0-alpha.0): prepare release
* [`8a9c4f1`](https://github.com/siderolabs/omni/commit/8a9c4f17ed6ee0d8e4a51b466d60a8278cd50f9c) feat: implement CLI configuration file (omniconfig)
* [`b0c92d5`](https://github.com/siderolabs/omni/commit/b0c92d56da00529c106f042399c1163375046785) feat: implement etcd audit controller
* [`0e993a0`](https://github.com/siderolabs/omni/commit/0e993a0977c711fb8767e3de2ad828fd5b9e688f) feat: properly support scaling down the cluster
* [`264cdc9`](https://github.com/siderolabs/omni/commit/264cdc9e015fd87724c7a07128d1136153732540) refactor: prepare for etcd backend integration
* [`b519d17`](https://github.com/siderolabs/omni/commit/b519d17971bb1c919286813b4c2465c2f5803a03) feat: show version in the UI
* [`a2fb539`](https://github.com/siderolabs/omni/commit/a2fb5397f9efb22a1354c5675180ca49537bee55) feat: keep track of loadbalancer health in the controller
* [`4789c62`](https://github.com/siderolabs/omni/commit/4789c62af0d1694d8d0a492cd6fb7d436e213fe5) feat: implement a new controller that can gather cluster machine data
* [`bd3712e`](https://github.com/siderolabs/omni/commit/bd3712e13491ede4610ab1452ae85bde6d92b2db) fix: populate machine label field in the patches created by the UI
* [`ba70b4a`](https://github.com/siderolabs/omni/commit/ba70b4a48623939d31775935bd0338c0d60ab65b) fix: rename to Omni, fix workers scale up, hide join token
* [`47b45c1`](https://github.com/siderolabs/omni/commit/47b45c129160821576d808d9a46a9ec5d14c6469) fix: correct filenames for Digital Ocean images
* [`9d217cf`](https://github.com/siderolabs/omni/commit/9d217cf16d432c5194110ae16a566b44b02a567e) feat: introduce new resources, deprecate `ClusterMachineTemplate`
* [`aee153b`](https://github.com/siderolabs/omni/commit/aee153bedb2f7856913a54b282603b07bf20059b) fix: address style issue in the Pods paginator
* [`752dd44`](https://github.com/siderolabs/omni/commit/752dd44ac42c95c644cad5640f6b2c5536a29676) chore: update Talos machinery to 1.2.0 and use client config struct
* [`88d7079`](https://github.com/siderolabs/omni/commit/88d7079a6656605a1a8dfed56d392414583a283e) fix: regenerate sources from proto files that were rolled back.
* [`84062c5`](https://github.com/siderolabs/omni/commit/84062c53417197417ff636a667289342089f390c) chore: update Talos to the latest master
* [`5a139e4`](https://github.com/siderolabs/omni/commit/5a139e473abcdf7fd25ad7c61dad8cbdc964a453) fix: properly route theila internal requests in the gRPC proxy
* [`4be4fb6`](https://github.com/siderolabs/omni/commit/4be4fb6a4e0bca29b32e1b732c227c9e7a0b1f43) feat: add support for 'talosconfig' generation
* [`9235b8b`](https://github.com/siderolabs/omni/commit/9235b8b522d4bc0712012425b68ff89e455886b9) fix: properly layer gRPC proxies
* [`9a516cc`](https://github.com/siderolabs/omni/commit/9a516ccb5c892ed8fe41f7cf69aaa5bb1d3fa471) fix: wait for selector of 'View All' to render in e2e tests.
* [`3cf3aa7`](https://github.com/siderolabs/omni/commit/3cf3aa730e7833c0c1abe42a6afb87a85f14b58c) fix: some unhandled errors in the e2e tests.
* [`c32c7d5`](https://github.com/siderolabs/omni/commit/c32c7d55c92007aa1aa10feab3c7a7de2b2afc42) fix: ignore updating cluster machines statuses without machine statuses
* [`4cfa307`](https://github.com/siderolabs/omni/commit/4cfa307b85b410b44e482b259d14670b55e4a237) chore: run rekres, fix lint errors and bump Go to 1.19
* [`eb2d449`](https://github.com/siderolabs/omni/commit/eb2d4499f1a3da7bc1552a6b099c28bed6fd0e4d) fix: skip the machines in `tearingDown` phase in the controller
* [`9ebc769`](https://github.com/siderolabs/omni/commit/9ebc769b89a2bab37fd081e555f84e3e4c99187e) fix: allow all services to be proxied by gRPC router
* [`ea2b01d`](https://github.com/siderolabs/omni/commit/ea2b01d0a0e054b259d710317fe368882534cf4c) fix: properly handle non empty resource id in the K8s resource watch
* [`3bb7da3`](https://github.com/siderolabs/omni/commit/3bb7da3a0fa6b746f6a7b9aa668e055bdf825e6a) feat: show a Cluster column in the Machine section
* [`8beb70b`](https://github.com/siderolabs/omni/commit/8beb70b7f045a218f9cb753e1402a07542b0bf1c) fix: ignore tearing down clusters in the `Cluster` migrations
* [`319d4e7`](https://github.com/siderolabs/omni/commit/319d4e7947cb78135f5a14c02afe5814c56a312c) fix: properly handle `null` memory modules list
* [`6c2120b`](https://github.com/siderolabs/omni/commit/6c2120b5ae2bd947f473d002dfe165646032e811) chore: introduce migrations manager for COSI DB state
* [`ec52139`](https://github.com/siderolabs/omni/commit/ec521397946cc15929472feb7c45435fb48df848) fix: filter out invalid memory modules info coming from Talos nodes
* [`8e87031`](https://github.com/siderolabs/omni/commit/8e870313a3a31d052eecf81acb522433ff98ae79) fix: bump loadbalancer timeout settings
* [`bc0ed26`](https://github.com/siderolabs/omni/commit/bc0ed2672064a6bf148cd9799b35a2790f5aa7f6) feat: introduce websocket, HTTP requests monitoring
* [`857401f`](https://github.com/siderolabs/omni/commit/857401f54e3922a9ab85d7dc703a5afb70c6ee45) feat: add HTTP logging (static, gateway), and websocket logging
* [`eb612a3`](https://github.com/siderolabs/omni/commit/eb612a38e9c71913ebecc9f345e17844d60800b8) fix: do hard stop of events sink gRPC server after 5 seconds
* [`3162513`](https://github.com/siderolabs/omni/commit/31625135e2b971d6b9f92eb4096c010113030a80) fix: populate nodes filter dropdown properly and rewrite filter function
* [`5713a51`](https://github.com/siderolabs/omni/commit/5713a516391a5190fac9b7044a9f71952ce15479) fix: make `TSelectList` search filter the items in the dropdown
* [`f2519ff`](https://github.com/siderolabs/omni/commit/f2519ff51b88766a907f1d7717ef74031157fd56) feat: don't allow using nodes with not enough mem for the cluster
* [`9e474d6`](https://github.com/siderolabs/omni/commit/9e474d69c76a898fc5b6fcd9fdc8e87f25b7dc53) feat: show disconnected warning in the machines list
* [`fa52b48`](https://github.com/siderolabs/omni/commit/fa52b48f54362c7305681ca79a7d98237531f2b4) feat: redesign Installation Media selection menu
* [`01e301a`](https://github.com/siderolabs/omni/commit/01e301a875699cf6fcc887cb31cd7939338f58e9) fix: query node list using `talosctl get members` instead of K8s nodes
* [`e694df5`](https://github.com/siderolabs/omni/commit/e694df59c50fbee356a48c94ade95e924ea46bb2) fix: display all available Talos versions on cluster create page
* [`7a87525`](https://github.com/siderolabs/omni/commit/7a87525ed1b928a8f8e3e6a39feb4c19009ec264) fix: use `v-model` instead of callbacks in the inputs
* [`d681f5f`](https://github.com/siderolabs/omni/commit/d681f5f58788612f144fa1f8d90ec6c996badb0e) feat: support scaling up the clusters
* [`e992b95`](https://github.com/siderolabs/omni/commit/e992b9574d7b8f76497f46e25764618ec274af1a) feat: show notification on image download progress
* [`8ea6d9f`](https://github.com/siderolabs/omni/commit/8ea6d9f1724b271919e538ed55ff6582858470f9) fix: probably fix 'context canceled' on image download
* [`692612b`](https://github.com/siderolabs/omni/commit/692612b7e628588fa7608cff683c5af406f24ca7) fix: improve the Talos image generation process
* [`a69c140`](https://github.com/siderolabs/omni/commit/a69c140e26f4298fcaafb1f96c389269992fc069) feat: introduce Prometheus metrics
* [`e90ca78`](https://github.com/siderolabs/omni/commit/e90ca7875c501391f860f5df9f2a4e4f8e2f2d7a) fix: make grpc api listen only on siderolink interface
* [`99fc28c`](https://github.com/siderolabs/omni/commit/99fc28c36c62a8d8c654c05f9b9c64ff37cedba8) fix: display correct cluster/machine status on ui
* [`eaf7655`](https://github.com/siderolabs/omni/commit/eaf7655395401cd88e6bd47f4f8aa958abee30f1) fix: add a pause before integration tests
* [`19ff1c9`](https://github.com/siderolabs/omni/commit/19ff1c909bedf63fe6cf2f5cc0e44f34046ca568) chore: rename download button
* [`e1c4e1b`](https://github.com/siderolabs/omni/commit/e1c4e1b171eab08585a3315ca5838c88a4d2eb24) feat: add download options for all talos images
* [`24e7863`](https://github.com/siderolabs/omni/commit/24e786369bfc0bb4966712296395db91751e657b) fix: delete cached clients from gRPC proxy when the cluster is destroyed
* [`58c89ef`](https://github.com/siderolabs/omni/commit/58c89ef3fe621ef6909c5d38a0d47cc861667f45) feat: implement `argesctl delete` command
* [`3c99b49`](https://github.com/siderolabs/omni/commit/3c99b49a9b680b091d92455a0d3bc325f8f68ca6) test: add a test which removes allocated machine
* [`75dd28f`](https://github.com/siderolabs/omni/commit/75dd28f56d7ce9a92b96822a867fbfe2655cd0fa) chore: fill in resource definitions for table headers
* [`028f168`](https://github.com/siderolabs/omni/commit/028f16886c41b7aa7eafb65308cc4adf4d624037) feat: End-to-end tests with playwright
* [`6be6b36`](https://github.com/siderolabs/omni/commit/6be6b3605583ce8e8068746624ca86ed6adc37af) chore: bump goimports from 0.1.10 to 0.1.11 and node from 18.5.0 to 18.6.0
* [`af4da08`](https://github.com/siderolabs/omni/commit/af4da086d4b709f504eda7909a36a8f0cf84e480) test: implement kernel log streaming test
* [`1eacfee`](https://github.com/siderolabs/omni/commit/1eacfee2c1084040ae2201eba957409218a92c66) feat: implement argesctl machine-logs output in 'zap-like' and 'dmesg' form.
* [`96ab7ab`](https://github.com/siderolabs/omni/commit/96ab7ab8317898dd45d129d5cecd2aaf1d379fba) chore: ignore memory modules with zero size
* [`fd0575f`](https://github.com/siderolabs/omni/commit/fd0575ff4050702c9d07e34c7d9d5596b4ad7311) chore: retrieve k8s versions from github registry
* [`8651527`](https://github.com/siderolabs/omni/commit/86515275a77741bacc790d2006f3671a5cfb27c6) feat: redo errgroup to return error on first nil error
* [`944222d`](https://github.com/siderolabs/omni/commit/944222d06607079b5d982afe4b19fc1dda7f1ec2) fix: show ClusterMachineStatus.Stage in 'Clusters' view
* [`f3f6b6e`](https://github.com/siderolabs/omni/commit/f3f6b6eecd3ffc13b69845dff50d2e8ab31bc0d2) chore: refactor run method and no longer ignore log receiver listener errors
* [`b316377`](https://github.com/siderolabs/omni/commit/b316377b277f87a184b969b3bbf20ebe6047a0a8) chore: rename 'Dmesg' to 'Console'
* [`19ee857`](https://github.com/siderolabs/omni/commit/19ee8578a6f1c1bf742699d1b5720dc4c2674c82) test: add a way to recover deleted machines
* [`e5b5bdc`](https://github.com/siderolabs/omni/commit/e5b5bdc39fa6f3812b15771366f942ddcbe7f328) fix: update SideroLink library for EEXIST fixes
* [`363de69`](https://github.com/siderolabs/omni/commit/363de69a50b5c1e9d07fa42152cca935844d118b) fix: spec collector equality
* [`841f3b2`](https://github.com/siderolabs/omni/commit/841f3b22aacc6d2875062ef324d900c5f2091f9d) feat: add ability to supply machine config patches on the machines
* [`907ca93`](https://github.com/siderolabs/omni/commit/907ca93247267d80125866c2b60225ceca3ada27) test: fix link destroy test
* [`4c9f99d`](https://github.com/siderolabs/omni/commit/4c9f99d32874cdaff1eb651bf6d74ef39167c273) fix: remove machine status if the machine is in tearing down phase
* [`d9747e5`](https://github.com/siderolabs/omni/commit/d9747e552e52156a9baeae962a9478231e26c566) fix: make cluster machine status test more reliable
* [`3bfff3b`](https://github.com/siderolabs/omni/commit/3bfff3bb0eea9d18956dee21aff7f3de900c6b82) fix: do not set up full theila runtime during clients tests
* [`4bf33bc`](https://github.com/siderolabs/omni/commit/4bf33bc9d37404a733c5039784c80e92800fb3dc) fix: immediately fail the request if the cluster is down
* [`124a5c2`](https://github.com/siderolabs/omni/commit/124a5c2947978e6bc86d1b19c9eacbcf7f870b53) fix: ensure the created date on resources is set
* [`14161bf`](https://github.com/siderolabs/omni/commit/14161bf3dad4484868359d186d99d9198b6eed95) feat: add scale up integration test and minor log fixes
* [`7af06fd`](https://github.com/siderolabs/omni/commit/7af06fd75959eb9e807680ac8a6ba4f0a7f59255) feat: make integration tests a subtests of one global test
* [`f7c1464`](https://github.com/siderolabs/omni/commit/f7c1464a1002f63daab29b36d19ea16de0cd5794) feat: implement log receiver for logs from Talos
* [`5b800ea`](https://github.com/siderolabs/omni/commit/5b800ea970215fb4e100ed7b3b73d7e218fd6d86) fix: accumulate bytes received/send in the link resource
* [`b3b1e9b`](https://github.com/siderolabs/omni/commit/b3b1e9bbfbf62632dc0d8c2239a72793883101ce) feat: machine removal
* [`fb01bc4`](https://github.com/siderolabs/omni/commit/fb01bc4b26c5b37f15bac923450e1f58fb7a3d89) fix: use Talos 1.2.0
* [`3a50efe`](https://github.com/siderolabs/omni/commit/3a50efe363c4724f369a02f672848ad7c284847c) feat: filter machines that can be added to cluster
* [`ba62db5`](https://github.com/siderolabs/omni/commit/ba62db521b47049e92557bf8cfc5f737e496bf57) fix: properly parse `siderolink-api-advertised-url` if there's no port
* [`96f835a`](https://github.com/siderolabs/omni/commit/96f835a91136f62d9dbdf5c1d1c46c729d57e51e) fix: properly display node selectors in FireFox
* [`12c20a4`](https://github.com/siderolabs/omni/commit/12c20a42c9dfdea5f88e0e7942fbdb42ea543b95) fix: populate disks when machines are connected during cluster create
* [`0dc97f8`](https://github.com/siderolabs/omni/commit/0dc97f8696a7c571d5318daf794700342e06f639) fix: adjust overview page to look closer to the mockups
* [`2b77af8`](https://github.com/siderolabs/omni/commit/2b77af8d39e555970487c3265dfbd63412e90d2f) feat: add the chart showing the count of clusters
* [`a1dff65`](https://github.com/siderolabs/omni/commit/a1dff6589d64207e6e7331d0407e7857f9c4079d) feat: implement ISO download with embedded kernel args
* [`37c03d8`](https://github.com/siderolabs/omni/commit/37c03d8cb04b02e79f42e70eeea1e4368445604d) test: pull kubeconfig and interact with Kubernetes API
* [`75bfb08`](https://github.com/siderolabs/omni/commit/75bfb08f0738fc9f67259caf12902db67860370f) fix: ignore the error on splitting host/port
* [`3be5a32`](https://github.com/siderolabs/omni/commit/3be5a3254168cddec8f1629789c2ae50d9eaa08e) feat: make the whole cluster list item clickable, add dropdown menu item
* [`2c9dc99`](https://github.com/siderolabs/omni/commit/2c9dc99000266b3d4c139f27dea4f6283709251e) fix: adjust the look of the Overview page a bit
* [`aa4a926`](https://github.com/siderolabs/omni/commit/aa4a926cbb85bf63312493b937440a174aed5070) feat: add the button for downloading cluster Kubeconfig on overview page
* [`4532de6`](https://github.com/siderolabs/omni/commit/4532de6f3d514a534c38a63731c43075698f5c01) feat: support basic auth in `argesctl` command
* [`b66bb3c`](https://github.com/siderolabs/omni/commit/b66bb3cbcc85d7be4348ecd9a6d5d62f72a90e11) feat: add summary information Overview page
* [`3bdbce4`](https://github.com/siderolabs/omni/commit/3bdbce41a3ed89a42556d837bc0c5cfe417e22e6) test: more cluster creation tests, two clusters, cleanup
* [`3b00bd5`](https://github.com/siderolabs/omni/commit/3b00bd5bf417c5c9cb42471d27811c1849a40c78) fix: improve cluster deletion and node reset flow
* [`2d83d16`](https://github.com/siderolabs/omni/commit/2d83d1694ec73da818004f91ede76a0bca30fe79) test: create a cluster and verify cluster machine statuses
* [`f471cfd`](https://github.com/siderolabs/omni/commit/f471cfdcf7c9e70f37436e173c3a58c1965e8bb2) fix: copy all labels from the `ClusterMachine` to `ClusterMachineStatus`
* [`ec32f86`](https://github.com/siderolabs/omni/commit/ec32f8632db104efd6fedc5421179175274d6339) test: add integration tests up to the cluster creation
* [`a8d3ee5`](https://github.com/siderolabs/omni/commit/a8d3ee5b14a57ad1d9d88512a95032bbda61e734) feat: add kubeconfig command to argesctl and fix kubeconfig
* [`10b9a3b`](https://github.com/siderolabs/omni/commit/10b9a3ba676a636e488805ed04a0c908c3d2cf53) test: implement API integration test
* [`3e6b891`](https://github.com/siderolabs/omni/commit/3e6b8913f916dc5e8ac3ef49e14648defa6e1bf6) feat: aggregate cluster machine statuses in cluster status controller
* [`f6cbc58`](https://github.com/siderolabs/omni/commit/f6cbc58a91124833f0cbae4ecd0c0416acbe8bfa) chore: ignore empty processor info
* [`c5fc71b`](https://github.com/siderolabs/omni/commit/c5fc71b86a5492d548ae9098c5c74de240ebd800) fix: clean up Kubernetes client and configs when a cluster is destroyed
* [`e8478fe`](https://github.com/siderolabs/omni/commit/e8478fe5280d5e8a32bb423ec96edacadabc7e43) fix: properly use tracker to cleanup `ClusterMachineConfig` resources
* [`044fcad`](https://github.com/siderolabs/omni/commit/044fcadb66de61742ab871d10f3fcf0f453f6e27) fix: make `MachineStatusController` connect to configured nodes
* [`2867099`](https://github.com/siderolabs/omni/commit/2867099a52d651c3b0f9d3abbae266f2792cafe7) feat: add api endpoint to fetch kubeconfig
* [`5f32667`](https://github.com/siderolabs/omni/commit/5f3266747012b590dd7a7d0ebc23ee0e80abb2ab) test: support registry mirrors for development purposes
* [`5114695`](https://github.com/siderolabs/omni/commit/5114695cfeb0b6c792002ff5f0f31c1944c269ab) refactor: consistent flag naming
* [`9ffb19e`](https://github.com/siderolabs/omni/commit/9ffb19e77968c6e411903a2c59fd9a18063b46d4) chore: use latest node
* [`5512321`](https://github.com/siderolabs/omni/commit/5512321f05b6b657a28abc25470664f6eb6e3d0a) refactor: set better defaults for cli args
* [`ff88242`](https://github.com/siderolabs/omni/commit/ff882427f56e42039b79900380b61b86d3290269) chore: mark 'siderolink-wireguard-endpoint' flags as required
* [`4a9d9ad`](https://github.com/siderolabs/omni/commit/4a9d9adef1e521d3c0293b6dc414f572bd8a93d4) feat: add the ClusterMachineStatus resource
* [`e4e8b62`](https://github.com/siderolabs/omni/commit/e4e8b6264cb48edd014f97129f52aefaa129fd63) refactor: unify all Arges API under a single HTTP server
* [`5af9049`](https://github.com/siderolabs/omni/commit/5af9049bdc2e09bf410e1b0646e4e08a4366f33b) chore: rename sidebar item
* [`a4fc47f`](https://github.com/siderolabs/omni/commit/a4fc47f97d79259532b91a8d391e84b59554ed8e) chore: fix build warning
* [`547b83c`](https://github.com/siderolabs/omni/commit/547b83c4a2a543d5b6ce4dca6cf6f5de87c33dcb) chore: bump siderolink version
* [`11c31f3`](https://github.com/siderolabs/omni/commit/11c31f39d834e3352b086c1aec665065fd74e944) refactor: drop one of the layered gRPC servers
* [`0adbbb7`](https://github.com/siderolabs/omni/commit/0adbbb7edfeacedd98a7e84c2f45ac458750a281) feat: introduce a way to copy kernel arguments from the UI
* [`ce5422a`](https://github.com/siderolabs/omni/commit/ce5422a27771a94cc25be70ec756711d140b2758) fix: import new COSI library to fix YAML marshaling
* [`d6cec09`](https://github.com/siderolabs/omni/commit/d6cec099cb6f4c3118e4263b9517176858bb9cfb) feat: implement Arges API client, and minimal `argesctl`
* [`65c8d68`](https://github.com/siderolabs/omni/commit/65c8d683187d82dc730752294c1bc03657f5df78) feat: implement cluster creation view
* [`8365b00`](https://github.com/siderolabs/omni/commit/8365b00df90ac55f99e0f82e1fa6d4367ebd6a3f) feat: re-enable old Theila UI
* [`63e703c`](https://github.com/siderolabs/omni/commit/63e703c4e1dfb4bf645fbc9cd28ba2a722e04dc2) fix: update Talos to the latest master
* [`d33e27b`](https://github.com/siderolabs/omni/commit/d33e27b49113729c5538fce688832152ff96a7ea) feat: implement clusters list view
* [`cb9e23c`](https://github.com/siderolabs/omni/commit/cb9e23ca6f420ac7b71acf6b19e9012265f3c69b) feat: protect Theila state from external API access
* [`952c235`](https://github.com/siderolabs/omni/commit/952c2359b32fdd077d85e312707f8b9c9e01ea0c) fix: properly allocated ports in the loadbalancer
* [`a58c479`](https://github.com/siderolabs/omni/commit/a58c479e9e31f70e806a1f3482b9b984c5c0ca68) chore: report siderolink events kernel arg
* [`8a56fe3`](https://github.com/siderolabs/omni/commit/8a56fe34ce1966fe28f9e432c696fdd779dfb638) refactor: move Theila resources to public `pkg/`
* [`1251699`](https://github.com/siderolabs/omni/commit/12516996eda859db6677403ad1f72a3994ea180b) fix: reset the `MachineEventsSnapshot` after the node is reset
* [`9a2e6af`](https://github.com/siderolabs/omni/commit/9a2e6af3113b795f57c4e3a86c1348b120fa3bbd) feat: implement bootstrap controller
* [`7107e27`](https://github.com/siderolabs/omni/commit/7107e27ee6b9ba644fc803e4463cbfcf26cf97de) feat: implement apply and reset config controller
* [`1579eb0`](https://github.com/siderolabs/omni/commit/1579eb09eb58f2cb679205e9e204369f3a362e07) feat: implement machine events handler and `ClusterStatus`
* [`7214f4a`](https://github.com/siderolabs/omni/commit/7214f4a514a921d6b9df7515116613996416f383) feat: implement cluster load balancer controller
* [`9c4fafa`](https://github.com/siderolabs/omni/commit/9c4fafaf6b8dc9b7ff08fe28704ca6a2e7efc097) feat: add a controller that manages load balancers for talos clusters
* [`7e3d80c`](https://github.com/siderolabs/omni/commit/7e3d80ce956d621ed79e4db094808831e18db85b) feat: add a resources that specify configurations for load balancers
* [`dc0d356`](https://github.com/siderolabs/omni/commit/dc0d356a181b4c37670d2ed4e8d7af370dccef60) feat: support Theila runtime watch with label selectors
* [`6a568a7`](https://github.com/siderolabs/omni/commit/6a568a72922e34e91f5448d3c1caa2f0b3a02e96) feat: implement `ClusterMachineConfig` resource and it's controller
* [`3db0f1c`](https://github.com/siderolabs/omni/commit/3db0f1c9d4e2d6f962b6f3216a4f9c7e2575dd21) feat: implement `TalosConfig` controller
* [`b7ae8e1`](https://github.com/siderolabs/omni/commit/b7ae8e113dc68acd87c4cfe5e3c8349d32bc392d) feat: introduce `Cluster` controller that adds finalizers on Clusters
* [`8d7ea02`](https://github.com/siderolabs/omni/commit/8d7ea0293e8f57388fd483dc82e79e6b4c76a53f) chore: use label selectors in `TalosConfig`, set labels on the resources
* [`cff9cb1`](https://github.com/siderolabs/omni/commit/cff9cb19ba8718fdad509b5e91cb8221c6c1ff00) fix: separate advertised endpoint from the actual wireguard endpoint
* [`5be6cc3`](https://github.com/siderolabs/omni/commit/5be6cc391adf8bcb58b8d47f09dad5aa75d1ad98) feat: implement cluster creation UI
* [`a1633eb`](https://github.com/siderolabs/omni/commit/a1633eb18772b9e99d687dfddd12fc09fd1ea5c4) chore: add typed wrappers around State, Reader and Writer
* [`5515f3d`](https://github.com/siderolabs/omni/commit/5515f3d004f54455a1eb1f4977bbb9d663fd1bca) feat: add `ClusterSecrets` resource and controller and tests
* [`7226f6c`](https://github.com/siderolabs/omni/commit/7226f6cdc60eeb4d6040d1aa0711fed378c50b33) feat: add `Cluster`, `ClusterMachine` and `TalosConfig` resources
* [`ec44930`](https://github.com/siderolabs/omni/commit/ec44930672ca8954c6ba68975c1799a087ec0c43) feat: enable vtprotobuf optimized marshaling
* [`15be219`](https://github.com/siderolabs/omni/commit/15be2198872fb637f7ba2e1ff550e4466179f2b1) feat: generate TS constants from go `//tsgen:` comments
* [`caa4c4d`](https://github.com/siderolabs/omni/commit/caa4c4d285dcd1176a70d87f28ee303cd0483ca8) fix: resource equality for proto specs
* [`beeca88`](https://github.com/siderolabs/omni/commit/beeca886213332f313f7f3a477d7e7c508e6d058) refactor: clarify code that creates or gets links for nodes
* [`340c63a`](https://github.com/siderolabs/omni/commit/340c63ad4ba918d4b11ab1f57fdbd3b5e5d8b3dc) feat: implement `Machines` page
* [`f7bc0c6`](https://github.com/siderolabs/omni/commit/f7bc0c69c69fe515cfa729bc062c730756a53019) feat: accept nodes if they provide the correct join token
* [`bdf789a`](https://github.com/siderolabs/omni/commit/bdf789a35da5491a4fcbd2af35a1c6efd22ab1fc) feat: immediately reconnect SideroLink peers after Arges restart
* [`6b74fa8`](https://github.com/siderolabs/omni/commit/6b74fa82ca5757d6f3809853c1ac3e7754efb06d) feat: implement MachineStatusController
* [`f5db0e0`](https://github.com/siderolabs/omni/commit/f5db0e05a87d5c11b4a1029b14020b19ca67035d) feat: add more info to the siderolink connection spec
* [`d3e4a71`](https://github.com/siderolabs/omni/commit/d3e4a71af8fd79328e4edda6d9642b83902b2003) refactor: simplify the usage of gRPC resource CRUD API
* [`2430115`](https://github.com/siderolabs/omni/commit/2430115af1aaac4226b7d5821e1fe706a1088501) feat: implement MachineController and small fixes
* [`e31d22d`](https://github.com/siderolabs/omni/commit/e31d22d7639753df53c130461ae1f96b9126f3a5) feat: support running Theila without contexts
* [`a6b3646`](https://github.com/siderolabs/omni/commit/a6b364626bd808687d5ad95307766344b16dd042) refactor: small fixes
* [`33d2b59`](https://github.com/siderolabs/omni/commit/33d2b59c202f03785580209c885aa297c023fa60) refactor: clean up a bit SideroLink code, fix shutdown
* [`98ec883`](https://github.com/siderolabs/omni/commit/98ec8830308755c7073a5d4510483e97d8e1d02d) chore: rename main executable to avoid clashing with Theila project
* [`828721d`](https://github.com/siderolabs/omni/commit/828721d9aa5d912cce628256f75579309d1ad67d) feat: enable COSI persistence for resources
* [`f1f7883`](https://github.com/siderolabs/omni/commit/f1f788344254e18bcab00a25b56a86289bfb1638) feat: set up siderolink endpoints in Theila
* [`6439335`](https://github.com/siderolabs/omni/commit/64393353ca7cf430f82bfe73a004da319da28261) refactor: migrate to `typed.Resource` in Theila internal state
* [`6195274`](https://github.com/siderolabs/omni/commit/61952742a47ea89e89228f057d0d3de351766150) refactor: restructure folders in the project
* [`1abf72b`](https://github.com/siderolabs/omni/commit/1abf72b4b2e382fe0cf9302b42242152c255a3ee) chore: update Talos libs to the latest version
* [`16dffd9`](https://github.com/siderolabs/omni/commit/16dffd9058570477b3a648896a89e6445e5b0162) fix: display delta time for pod's age
* [`8b80726`](https://github.com/siderolabs/omni/commit/8b807262b23cfa830f3ff444d49f11b3a1654703) feat: update favicon to sidero logo
* [`2da7378`](https://github.com/siderolabs/omni/commit/2da737841c2ae0bf1f1f916dc6f45b1e3996d6e4) feat: show the extended hardware info
* [`d3c6004`](https://github.com/siderolabs/omni/commit/d3c6004f9767bf0cff9191dc130308c848ede077) chore: allow getting resources without version and group
* [`eb19087`](https://github.com/siderolabs/omni/commit/eb190875b30275195e52f1a95ed0bb3aae08424f) fix: remove t-header error notification
* [`5a28202`](https://github.com/siderolabs/omni/commit/5a28202c939ef9683d14fb3d873e0bacb35577db) feat: restyle t-alert component
* [`9f2b482`](https://github.com/siderolabs/omni/commit/9f2b48228bbfa39d33b07ae43e9fdb34192c3eed) fix: get rid of racy code in the kubeconfig request code
* [`c40824e`](https://github.com/siderolabs/omni/commit/c40824ecc5d10cb5289e133b8b1f51213aa12f7f) feat: add text Highlight feature
* [`9018c81`](https://github.com/siderolabs/omni/commit/9018c81bd0d7c58bb5c632c06f3c3904f6674e03) feat: use `~/.talos/config` as a primary source for clusters
* [`e10547b`](https://github.com/siderolabs/omni/commit/e10547b5761ad96ab8b5766fe5c3f06fcdf86477) refactor: remove old components and not used code parts
* [`f704684`](https://github.com/siderolabs/omni/commit/f7046846ea8e83a0e39647c4fcc49addf4c56061) fix: properly calculate servers capacity
* [`755a077`](https://github.com/siderolabs/omni/commit/755a0779014b0a4177e0fc5180db20720be5a814) fix: use proper units for memory and CPU charts on the node monitor page
* [`d0a083d`](https://github.com/siderolabs/omni/commit/d0a083d1c15c319e236dd258fabcc9a231f797a1) release(v0.2.0-alpha.0): prepare release
* [`53878ee`](https://github.com/siderolabs/omni/commit/53878eea09c18f2bc0dd55ca11a6743587748319) fix: properly update servers menu item when the context is changed
* [`b4cb9c7`](https://github.com/siderolabs/omni/commit/b4cb9c7989ec5299785b86acb3fa0ee648efd259) feat: restyle TMonitor page
* [`f0377e2`](https://github.com/siderolabs/omni/commit/f0377e2ad5da702af71f2706141f4d7c638c7a15) fix: invert chart value for cpu, storage and memory on the overview page
* [`6ea6ecf`](https://github.com/siderolabs/omni/commit/6ea6ecf12c4d8b5253b4dfc2e64f5b5d787d022a) fix: update capi-utils to fix talosconfig requests for CAPI clusters
* [`e3796d3`](https://github.com/siderolabs/omni/commit/e3796d3876d33248fd0998901273a14d29a487a3) chore: update capi-utils
* [`39186eb`](https://github.com/siderolabs/omni/commit/39186ebe50da531f35d21ac2488f8a58c1ef8e78) feat: implement overview page, cluster dropdown, ongoing tasks
* [`59f2b27`](https://github.com/siderolabs/omni/commit/59f2b27be4d7f5a591fdeae533d649494356250d) docs: update README.md
* [`2b7831f`](https://github.com/siderolabs/omni/commit/2b7831f2d22106ac8a82f890d73c2705841b0739) feat: add Kubernetes and Servers pages
* [`4451a5b`](https://github.com/siderolabs/omni/commit/4451a5bc9f5c6b058c6bcf1252b7c83a001cafbe) fix: properly set TaskStatus namespace in the initial call
* [`4545464`](https://github.com/siderolabs/omni/commit/454546425f2fd7e4418aa8a03465f3a062de804e) fix: add new fields to the TaskStatus spec, update Talos
* [`891cf3b`](https://github.com/siderolabs/omni/commit/891cf3b79c8430deeed8a168955afd6e97083baa) docs: describe client context types, usage
* [`309b515`](https://github.com/siderolabs/omni/commit/309b51545ead2ee144244591df2e5ead2849fb11) feat: update k8s upgrades tasks structure for the new UI representation
* [`5aa8ca2`](https://github.com/siderolabs/omni/commit/5aa8ca24bd3159879c46c8e8a134702b174e3362) feat: add NodesPage
* [`db434e0`](https://github.com/siderolabs/omni/commit/db434e07b9f23562bd746a0f78e3868b079006e2) feat: add TPagination component
* [`0b51727`](https://github.com/siderolabs/omni/commit/0b51727efed31f13f52fa20b360071e7e2a6d9eb) feat: add Pods, Dashboard, Upgrade views, etc
* [`c549b8b`](https://github.com/siderolabs/omni/commit/c549b8b9ee8a563f14b2e791f91a7b3cb0430aa7) feat: add Overview and Upgrade Kubernetes pages
* [`cec2e85`](https://github.com/siderolabs/omni/commit/cec2e854f4f3999109220902bccaee6c25d1f502) chore: define constants for all used resource types
* [`962bdaf`](https://github.com/siderolabs/omni/commit/962bdaf6406ab8e5febea0ad8d32da9c86fa39e7) feat: add TSideBar
* [`fa28ccb`](https://github.com/siderolabs/omni/commit/fa28ccb67f52c1dd9096b23388427d78be526275) feat: add TheHeader component
* [`f3418a5`](https://github.com/siderolabs/omni/commit/f3418a59e38e551bd0be7cc7ae66ef4645719aa7) feat: button;icons;config
* [`db30f50`](https://github.com/siderolabs/omni/commit/db30f503730bdbd8ed359d4070dea0214df67fcd) fix: add `frontend/node_modules` to gitignore
* [`a675b86`](https://github.com/siderolabs/omni/commit/a675b86f7d55cecd4ae1277cbf057a6bc264940c) fix: properly pass label selector to the metadata in ClusterListItem
* [`7911d6a`](https://github.com/siderolabs/omni/commit/7911d6a31abdb51e86586a025b705ddfeb1dd19e) chore: add ability to start local development server for the frontend
* [`076fee1`](https://github.com/siderolabs/omni/commit/076fee10c6583dc49e6530b02cab1f757da0e853) feat: use CAPI utils for CAPI requests
* [`5ed5ba2`](https://github.com/siderolabs/omni/commit/5ed5ba2a122585a97cf65c3ff081126752cd26fa) fix: more websocket client bugfixes
* [`6fe22ad`](https://github.com/siderolabs/omni/commit/6fe22ad370026380ba75b38e261870addc341e6f) fix: reset reconnect timeouts after the client is reconnected
* [`c4b144a`](https://github.com/siderolabs/omni/commit/c4b144af272a46dbdc8d1bb35784e09ba1b79987) fix: talosconfig/kubeconfig when using the default context
* [`b439a37`](https://github.com/siderolabs/omni/commit/b439a371c13a8d46d986a1dae3d6f4b7cba4a298) fix: properly handle Same-Origin header in websockets
* [`ffffed1`](https://github.com/siderolabs/omni/commit/ffffed100cec18209bae723b9919eb8613950649) fix: read node name from nodename resource instead of hostname
* [`2d6f984`](https://github.com/siderolabs/omni/commit/2d6f9844440a6d18b3093dea6228ac6a237dc86b) fix: use secure websockets if the page itself is using https
* [`799f2d2`](https://github.com/siderolabs/omni/commit/799f2d2d00762d5270dd4a3f4b4b312b32dbb7dd) feat: rework the node overview page
* [`0d0eaf4`](https://github.com/siderolabs/omni/commit/0d0eaf4b2721dfa1b04bce24e4a1e476579e3a74) fix: make charts height resize depending on the screen height
* [`7de0101`](https://github.com/siderolabs/omni/commit/7de0101bf0e613653caadd5733db0e29a6bb5bfb) fix: use polyfill to fix streaming APIs on Firefox
* [`0cff2b0`](https://github.com/siderolabs/omni/commit/0cff2b02b5d8b2c2c644067cf6bd3ed573cb784d) feat: small UI adjustments
* [`d70bd41`](https://github.com/siderolabs/omni/commit/d70bd41992e13fb3dacc1740532083a8f6ce9afa) feat: implement accept Sidero server functional
* [`f3a6e16`](https://github.com/siderolabs/omni/commit/f3a6e16a79e1bca9ea6c87eb0d3e0f2a6c65ff2e) feat: add top processes list to the Overview page
* [`3cf97e4`](https://github.com/siderolabs/omni/commit/3cf97e4b9e07f8383da8a6fb7a993b70c8f82503) refactor: use the same object for gRPC metadata context and messages
* [`243206f`](https://github.com/siderolabs/omni/commit/243206f95aa6ba944bd4361db6274e7072bae1fc) release(v0.1.0-alpha.2): prepare release
* [`e5b6f29`](https://github.com/siderolabs/omni/commit/e5b6f29fd298904e06284a67681cc0ce5135145f) feat: implement node Reset
* [`bcb7d23`](https://github.com/siderolabs/omni/commit/bcb7d237c31f42a35f5c3b53e7615ddae1ce0a8b) fix: node IP not being truncated
* [`e576d33`](https://github.com/siderolabs/omni/commit/e576d33ba40f629eed14668f2d9bf77d7fef62c2) feat: add upgrade UI for CAPI clusters
* [`10cdce7`](https://github.com/siderolabs/omni/commit/10cdce7fcc219af969a85a41d18fb904936faa0a) fix: server labels key/value order and chevron orientation
* [`4007177`](https://github.com/siderolabs/omni/commit/40071775d6de1eea697f67e55441c384c86e75d9) feat: implement Kubernetes upgrade UI components
* [`f4917ee`](https://github.com/siderolabs/omni/commit/f4917eecfb3173acf7518883c738118c8537d657) fix: accumulate chart updates into a single update
* [`414d76c`](https://github.com/siderolabs/omni/commit/414d76c1c926695e5d66787b34decae92e151b45) feat: implement upgrade controller
* [`36742ea`](https://github.com/siderolabs/omni/commit/36742ea5ab1e8a983b73f73443c1cf122a90d054) feat: introduce create, delete and update gRPC APIs
* [`2b3d314`](https://github.com/siderolabs/omni/commit/2b3d314a460b385d8c13bdd025fadb37b5508bdc) feat: install internal COSI runtime alongside with K8s and Talos
* [`ae7f784`](https://github.com/siderolabs/omni/commit/ae7f784d08621d18075b1763f026a7513d9d9dcb) refactor: move all generated TypeScript files under `frontend/src/api`
* [`61bad64`](https://github.com/siderolabs/omni/commit/61bad64540c28fb0520a39a6c64d64c3e9353361) release(v0.1.0-alpha.1): prepare release
* [`8e5e722`](https://github.com/siderolabs/omni/commit/8e5e7229470713d2fbd5ad0df027bd825f5481e3) feat: implement node reboot controls
* [`9765a88`](https://github.com/siderolabs/omni/commit/9765a88069f05c49f5a7d854675ee37e1c7a8273) feat: dmesg logs page
* [`ecbbd67`](https://github.com/siderolabs/omni/commit/ecbbd67936b1fb570d706fe3b93b81f6089b5124) feat: use updated timestamp to display event time on the graph
* [`7c56773`](https://github.com/siderolabs/omni/commit/7c56773448a496fe1ceeec3c47978975ce336b3a) refactor: use Metadata to pass context in all gRPC calls
* [`abb4733`](https://github.com/siderolabs/omni/commit/abb47330222217d7d8b5c36ff28902415bc755d8) feat: implement service logs viewer
* [`8e8e032`](https://github.com/siderolabs/omni/commit/8e8e032b20d082bfd71a26c2af2bbc821d9c2a7b) feat: add ability to pick sort order on the servers page
* [`1a1c728`](https://github.com/siderolabs/omni/commit/1a1c728ac929bb02db7f1bd0b991a747e63fe81a) fix: resolve the issue with idFn value generating undefined ids
* [`2e83fe2`](https://github.com/siderolabs/omni/commit/2e83fe23a7feb51b73bc7b53997636b641ae42b9) feat: allow filtering servers by picking from predefined categories
* [`48f776e`](https://github.com/siderolabs/omni/commit/48f776e10f6c79772481393d7397557419520046) fix: navigate home when changing the context
* [`a1ce0ca`](https://github.com/siderolabs/omni/commit/a1ce0ca8c8fabb2267c3dc6f6b1509f131e18ba8) fix: resolve services search issues
* [`5b768f8`](https://github.com/siderolabs/omni/commit/5b768f85277ee31131994ae0b253700a5d26978d) feat: make stacked lists searchable
* [`ec1bc5b`](https://github.com/siderolabs/omni/commit/ec1bc5b48943e473c756ebc7a8c943a34cdeaeac) feat: implement stats component and add stats to the servers page
* [`1a85999`](https://github.com/siderolabs/omni/commit/1a8599981f93fc5ce68e23b1b4cd7aabbb43c90c) feat: align Sidero servers list outlook with the wireframes
* [`524264c`](https://github.com/siderolabs/omni/commit/524264c515a9efdce9f06a3c2ebd59c2979f9b2a) fix: display error message and use proper layout for the spinner
* [`5263d16`](https://github.com/siderolabs/omni/commit/5263d16cfb936aad9ba461e0cc7b150ff9b806d5) feat: introduce node stats page
* [`8feb35e`](https://github.com/siderolabs/omni/commit/8feb35e95a6d588e1d9c605231308976be452a2e) feat: make root sidebar sections collapsible
* [`36ad656`](https://github.com/siderolabs/omni/commit/36ad656a3bbdc1e2915a87c0d09c31738ae3f3c4) feat: detect cluster capabilities
* [`a25d90d`](https://github.com/siderolabs/omni/commit/a25d90d58a85b3b73432858f134fa09cd1338d5c) feat: support switching context in the UI
* [`67903e2`](https://github.com/siderolabs/omni/commit/67903e23f49623ae9a9a6b297282c62aa8579aa8) refactor: separate Watch from StackedList
* [`76b9e1d`](https://github.com/siderolabs/omni/commit/76b9e1dc88cccf74cebb28470eae5e9249809d40) release(v0.1.0-alpha.0): prepare release
* [`7bde4c8`](https://github.com/siderolabs/omni/commit/7bde4c8c6e16c197578cbb4e037a05d50194958f) fix: cobra command was initialized but not actually used
* [`04624c9`](https://github.com/siderolabs/omni/commit/04624c95cec587ae0b0d8888d95d484ef8d98cfa) feat: support getting Talos and Kubernetes client configs for a cluster
* [`219b9c8`](https://github.com/siderolabs/omni/commit/219b9c8663fe03af65796b0b6299cff5e66b3efc) feat: implement notifications component
* [`f8b19a0`](https://github.com/siderolabs/omni/commit/f8b19a0585e6e19c0e7da4e4afad5bbd264e0029) feat: decouple watch list from the view
* [`2f8c96e`](https://github.com/siderolabs/omni/commit/2f8c96e44012e7bd0db9869eeb90ab48ff41e162) feat: implement appearance settings modal window
* [`de745d6`](https://github.com/siderolabs/omni/commit/de745d6b7170a9c509cc835a8b675a1c788e80f4) feat: implement Talos runtime backend
* [`af69a0d`](https://github.com/siderolabs/omni/commit/af69a0d58906a86974bc7dbec2c09ca9f78b152f) feat: support getting Kubernetes resource through gRPC gateway
* [`2c50010`](https://github.com/siderolabs/omni/commit/2c50010b0d9f7b168354fedd698600d94123c354) feat: implement breadcrumbs component, add support for table header
* [`3fc1e80`](https://github.com/siderolabs/omni/commit/3fc1e808875f6f502cd2657c4548dd886fbf465d) feat: implement nodes view
* [`961e93a`](https://github.com/siderolabs/omni/commit/961e93a4af430eaa9efcd1e2922af8072fe4cf85) feat: implement clusters view
* [`e8248ff`](https://github.com/siderolabs/omni/commit/e8248ffab89633cae8834631e39cf4dce5e4147a) feat: use plain zap instead of SugaredLogger everywhere
* [`81ba93d`](https://github.com/siderolabs/omni/commit/81ba93dffdc37efdde06557a1c63511a7d61b2f2) chore: generate websocket protocol messages using protobuf
* [`37a878d`](https://github.com/siderolabs/omni/commit/37a878dd396b650df8afaf6730f9afe52d35569c) feat: make JS websocket reconnect on connection loss
* [`23b3281`](https://github.com/siderolabs/omni/commit/23b3281f8880800a9084e1c8a74617fcf966c846) feat: use dynamic watcher to allow listing any kinds of resources
* [`16475f5`](https://github.com/siderolabs/omni/commit/16475f51cc9651736213b36c57381b24dcabdc62) feat: implement real time update server on top of web sockets
* [`76b39ae`](https://github.com/siderolabs/omni/commit/76b39ae563d9f09ecac3451389e3d260abdad48d) feat: create hello world Vue app using Kres
* [`baab493`](https://github.com/siderolabs/omni/commit/baab493f155cbd78c2e8af6ce45268c40ef6aeed) Initial commit
</p>
</details>

### Changes since v0.1.0-alpha.0
<details><summary>81 commits</summary>
<p>

* [`8b284f3`](https://github.com/siderolabs/omni/commit/8b284f3aa26cf8a34452f33807dcc04045e7a098) feat: implement Kubernetes API OIDC proxy and OIDC server
* [`adad8d0`](https://github.com/siderolabs/omni/commit/adad8d0fe2f3356e97de613104196233a3b98ff5) refactor: rework LoadBalancerConfig/LoadBalancerStatus resources
* [`08e2cb4`](https://github.com/siderolabs/omni/commit/08e2cb4fd40ec918bf458edd6a5d8e6c86fe5c97) feat: support editing config patches on cluster and machine set levels
* [`e2197c8`](https://github.com/siderolabs/omni/commit/e2197c83e994afb435671f5af5cdefa843bbddb5) test: e2e testing improvements
* [`ec9051f`](https://github.com/siderolabs/omni/commit/ec9051f6dfdf1f5acaf3fa6766dc1195b6f6dcdd) fix: config patching
* [`e2a1d6c`](https://github.com/siderolabs/omni/commit/e2a1d6c78809eaa4168ca5ede433824797a6aa4e) fix: send logs in JSON format by default
* [`954dd70`](https://github.com/siderolabs/omni/commit/954dd70b935b7c373ba5830fd7ad6e965f6b0da8) chore: replace talos-systems depedencies with siderolabs
* [`acf94db`](https://github.com/siderolabs/omni/commit/acf94db8ac80fb6f15cc87ff276b7edca0cb8661) chore: add payload logger
* [`838c716`](https://github.com/siderolabs/omni/commit/838c7168c64f2296a9e01d3ef6ab4feb9f16aeb9) fix: allow time skew on validating the public keys
* [`dd481d6`](https://github.com/siderolabs/omni/commit/dd481d6cb3620790f6e7a9c8e305defb507cbe5f) fix: refactor runGRPCProxy in router tests to catch listener errors
* [`e68d010`](https://github.com/siderolabs/omni/commit/e68d010685d4f0a5d25fee671744119cecf6c27b) chore: small fixes
* [`ad86875`](https://github.com/siderolabs/omni/commit/ad86875ec146e05d7d7f461bf7c8094a8c143df5) feat: minor adjustments on the cluster create page
* [`e61f194`](https://github.com/siderolabs/omni/commit/e61f1943e965287c79fbaef05760bb0b0deee988) chore: implement debug handlers with controller dependency graphs
* [`cbbf901`](https://github.com/siderolabs/omni/commit/cbbf901e601d31c777ad2ada0f0036c57020ba96) refactor: use generic TransformController more
* [`33f9f2c`](https://github.com/siderolabs/omni/commit/33f9f2ce3ec0999198f311ae4bae9b58e57153c9) chore: remove reflect from runtime package
* [`6586963`](https://github.com/siderolabs/omni/commit/65869636aa33013b5feafb06e727b9d2a4cf1c19) feat: add scopes to users, rework authz & add integration tests
* [`bb355f5`](https://github.com/siderolabs/omni/commit/bb355f5c659d8c66b825de409d9446767005a2bb) fix: reload the page to init the UI Authenticator on signature fails
* [`c90cd48`](https://github.com/siderolabs/omni/commit/c90cd48eefa7f29328a456aa5ca474eece17c6fe) chore: log auth context
* [`d278780`](https://github.com/siderolabs/omni/commit/d2787801a4904fe895996e5319f301a1d7ca76df) fix: update Clusters page UI
* [`5e77607`](https://github.com/siderolabs/omni/commit/5e776072285e535e93c0458774dcad810b9b857a) tests: abort on first failure
* [`4c55980`](https://github.com/siderolabs/omni/commit/4c5598083ff6d8763c8763d8e46a3d7b659784ff) chore: get full method name from the service
* [`2194f43`](https://github.com/siderolabs/omni/commit/2194f4391607e6e73bce1917d2744e78fdd2cebc) feat: redesign cluster list view
* [`40b3f23`](https://github.com/siderolabs/omni/commit/40b3f23071096987e8a7c6f30a2622c317c190cb) chore: enable gRPC request duration histogram
* [`0235bb9`](https://github.com/siderolabs/omni/commit/0235bb91a71510cf4d349eedd3625b119c7e4e11) refactor: make sure Talos/Kubernetes versions are defined once
* [`dd6154a`](https://github.com/siderolabs/omni/commit/dd6154a45d5dcd14870e0aa3f97aa1d4e53bdcfb) chore: add public key pruning
* [`68908ba`](https://github.com/siderolabs/omni/commit/68908ba330ecd1e285681e24db4b9037eb2e8202) fix: bring back UpgradeInfo API
* [`f1bc692`](https://github.com/siderolabs/omni/commit/f1bc692c9125f7683fe5f234b03eb3521ba7e773) refactor: drop dependency on Talos Go module
* [`0e3ef43`](https://github.com/siderolabs/omni/commit/0e3ef43cfed68e53879e6c22b46e7d0568ddc05f) feat: implement talosctl access via Omni
* [`2b0014f`](https://github.com/siderolabs/omni/commit/2b0014fea15da359217f89ef723965dcc9faa739) fix: provide a way to switch the user on the authenticate page
* [`e295d7e`](https://github.com/siderolabs/omni/commit/e295d7e2854ac0226e7efda32864f6a687a88470) chore: refactor all controller tests to use assertResource function
* [`8251dfb`](https://github.com/siderolabs/omni/commit/8251dfb9e44341e9df9471f387cc76c91359cf84) refactor: extract PGP client key handling
* [`02da9ee`](https://github.com/siderolabs/omni/commit/02da9ee66f15462e6f4d7da18515651a5fde11aa) refactor: use extracted go-api-signature library
* [`4bc3db4`](https://github.com/siderolabs/omni/commit/4bc3db4dcbc14e0e51c7a3b5257686b671cc2823) fix: drop not working upgrade k8s functional
* [`17ca75e`](https://github.com/siderolabs/omni/commit/17ca75ef864b7a59f9c6f829de19cc9630a670c0) feat: add 404 page
* [`8dcde2a`](https://github.com/siderolabs/omni/commit/8dcde2af3ca49d9be16cc705c0b403826f2eee5d) feat: implement logout flow in the frontend
* [`ba766b9`](https://github.com/siderolabs/omni/commit/ba766b9922302b9d1f279b74caf94e6ca727f86f) fix: make `omnictl` correctly re-auth on invalid key
* [`fd16f87`](https://github.com/siderolabs/omni/commit/fd16f8743d3843e8ec6735a7c2e96532694b876e) fix: don't set timeout on watch gRPC requests
* [`8dc3cc6`](https://github.com/siderolabs/omni/commit/8dc3cc682e5419c3824c6e740a32085c386b8817) fix: don't use `omni` in external names
* [`2513661`](https://github.com/siderolabs/omni/commit/2513661578574255ca3f736d3dfa1f307f5d43b6) fix: reset `Error` field of the `MachineSetStatus`
* [`b611e99`](https://github.com/siderolabs/omni/commit/b611e99e14a7e2ebc64c55ed5c95a47e17d6ac32) fix: properly handle `Forbidden` errors on the authentication page
* [`8525502`](https://github.com/siderolabs/omni/commit/8525502265b10dc3cc056d301785f6f60e4f7e22) fix: stop runners properly and clean up StatusMachineSnapshot
* [`ab0190d`](https://github.com/siderolabs/omni/commit/ab0190d9a41b830daf60173b998acdbcbbdd3754) feat: implement scopes and enforce authorization
* [`9198d96`](https://github.com/siderolabs/omni/commit/9198d96ea9d57bb5949c59350aec42b2ce13ebac) feat: sign gRPC requests on the frontend to enable Authentication flow
* [`bdd8f21`](https://github.com/siderolabs/omni/commit/bdd8f216a9eca7ec657fa0dc554e663743f058d1) chore: remove reset button and fix padding
* [`362db57`](https://github.com/siderolabs/omni/commit/362db570349b4a2659f746ce18a436d684481ecb) fix: gRPC verifier should verify against original JSON payload
* [`30186b8`](https://github.com/siderolabs/omni/commit/30186b8cfe2eea6eaade8bacf31114886d3da3ea) fix: omnictl ignoring omniconfig argument
* [`e8ab0ba`](https://github.com/siderolabs/omni/commit/e8ab0ba45648b8f521500b46fe032797da6a111f) fix: do not attempt to execute failed integration test again
* [`9fda25e`](https://github.com/siderolabs/omni/commit/9fda25ef45f0060cc6c3ec812f5fa1c7b1015801) chore: add more info on errors to different controllers
* [`ccda526`](https://github.com/siderolabs/omni/commit/ccda5260c4645b5929724574a9f856eeaa4c232f) chore: bump grpc version
* [`b1ac125`](https://github.com/siderolabs/omni/commit/b1ac1255da5ca4b5d9c409e27c51e4298275e73c) chore: emit log when we got machine status event.
* [`005d257`](https://github.com/siderolabs/omni/commit/005d257c25c745b61e5a25c39167d511710562c7) chore: set admin role specifically for Reboot request.
* [`27f0e30`](https://github.com/siderolabs/omni/commit/27f0e309cec76a454e5bb24c2df1e62d9e4718e0) chore: update deps
* [`77f0219`](https://github.com/siderolabs/omni/commit/77f02198c1e7fb215548f3a0e2be30a0e19aaf6d) test: more unit-tests for auth components
* [`0bf6ddf`](https://github.com/siderolabs/omni/commit/0bf6ddfa46e0ea6ad255ede00a600c390344e221) fix: pass through HTTP request if auth is disabled
* [`4f3a67b`](https://github.com/siderolabs/omni/commit/4f3a67b08e03a1bad65c2acb8d65f0281fdd2f9e) fix: unit-tests for auth package and fixes
* [`e3390cb`](https://github.com/siderolabs/omni/commit/e3390cbbac1d0e78b72512c6ebb64a8f53dcde17) chore: rename arges-theila to omni
* [`14d2614`](https://github.com/siderolabs/omni/commit/14d2614538ec696d468a0850bd4ee7bc6884c3b1) chore: allow slashes in secretPath
* [`e423edc`](https://github.com/siderolabs/omni/commit/e423edc072714e7f693249b60079f5f700cc0a65) fix: add unit-tests for auth message and fix issues
* [`b5cfa1a`](https://github.com/siderolabs/omni/commit/b5cfa1a84e93b6bbf5533c599917f293fc5cdf66) feat: add vault client
* [`b47791c`](https://github.com/siderolabs/omni/commit/b47791ce303cbb9a8aab279685d17f92a480c7f4) feat: sign grpc requests on cli with pgp key & verify it on server
* [`d6ef4d9`](https://github.com/siderolabs/omni/commit/d6ef4d9c36758cb0091e2c528b848952f312941a) feat: split account ID and name
* [`e412e1a`](https://github.com/siderolabs/omni/commit/e412e1a69edad0d19d7e46fa3aa076dcb8e6d4b6) chore: workaround the bind problem
* [`e23cc59`](https://github.com/siderolabs/omni/commit/e23cc59bb8cb8f9df81738d4c58aed08d80fa9c4) chore: bump minimum Talos version to v1.2.4
* [`0638a29`](https://github.com/siderolabs/omni/commit/0638a29d78c092641573aa2b8d2e594a7ff6aab4) feat: stop using websockets
* [`8f3c19d`](https://github.com/siderolabs/omni/commit/8f3c19d0f0ecfbe5beabc7dc508dcafa720e83e2) feat: update install media to be identifiable
* [`70d1e35`](https://github.com/siderolabs/omni/commit/70d1e354466618bb07c13445a16ca639be12009e) feat: implement resource encryption
* [`7653638`](https://github.com/siderolabs/omni/commit/76536386499889994b65f66a8a40f18b5535c5ba) fix: fix NPE in integration tests
* [`e39849f`](https://github.com/siderolabs/omni/commit/e39849f4047f028251123781bd8be350ebbfd65d) chore: update Makefile and Dockerfile with kres
* [`4709473`](https://github.com/siderolabs/omni/commit/4709473ec20fbf92a3240fb3376a322f1321103a) fix: return an error if external etcd client fails to be built
* [`5366661`](https://github.com/siderolabs/omni/commit/536666140556ba9b997a2b5d4441ea4b5f42d1c5) refactor: use generic transform controller
* [`a2a5f16`](https://github.com/siderolabs/omni/commit/a2a5f167f21df6375767d018981651d60bb2f768) feat: limit access to Talos API via Omni to `os:reader`
* [`e254201`](https://github.com/siderolabs/omni/commit/e2542013938991faa8f1c521fc524b8fcf31ea34) feat: merge internal/external states into one
* [`3258ca4`](https://github.com/siderolabs/omni/commit/3258ca487c818a34924f138640f44a2e51d307fb) feat: add `ControlPlaneStatus` controller
* [`1c0f286`](https://github.com/siderolabs/omni/commit/1c0f286a28f5134333130708d031dbfa11051a42) refactor: use `MachineStatus` Talos resource
* [`0a6b19f`](https://github.com/siderolabs/omni/commit/0a6b19fb916ea301a8f5f6ccd9bbdaa7cb4c39e0) chore: drop support for Talos resource API
* [`ee5f6d5`](https://github.com/siderolabs/omni/commit/ee5f6d58a2b22a87930d3c8bb9963f71c92f3908) feat: add auth resource types & implement CLI auth
* [`36736e1`](https://github.com/siderolabs/omni/commit/36736e14e5c837d38568a473834d14073b88a153) fix: use correct protobuf URL for cosi resource spec
* [`b98c56d`](https://github.com/siderolabs/omni/commit/b98c56dafe33beef7792bd861ac4e637fe13c494) feat: bump minimum version for Talos to v1.2.3
* [`b93bc9c`](https://github.com/siderolabs/omni/commit/b93bc9cd913b017c66502d96d99c52e4d971e231) chore: move containers and optional package to the separate module
* [`e1af4d8`](https://github.com/siderolabs/omni/commit/e1af4d8a0bee31721d8946ef452afe04da6b494d) chore: update COSI to v0.2.0-alpha.1
* [`788dd37`](https://github.com/siderolabs/omni/commit/788dd37c0be32745547ee8268aa0f004041dc96f) feat: implement and enable by default etcd backend
</p>
</details>

### Dependency Changes

This release has no dependency changes

## [Omni 0.1.0-alpha.0](https://github.com/siderolabs/arges-theila/releases/tag/v0.1.0-alpha.0) (2022-09-19)

Welcome to the v0.1.0-alpha.0 release of Omni!
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/arges-theila/issues.

### Contributors

* Artem Chernyshev
* Artem Chernyshev
* Andrey Smirnov
* Philipp Sauter
* evgeniybryzh
* Dmitriy Matrenichev
* Utku Ozdemir
* Noel Georgi
* Andrew Rynhard
* Andrew Rynhard
* Gerard de Leeuw
* Steve Francis
* Tim Jones
* Volodymyr Mazurets

### Changes
<details><summary>267 commits</summary>
<p>

* [`8a9c4f1`](https://github.com/siderolabs/arges-theila/commit/8a9c4f17ed6ee0d8e4a51b466d60a8278cd50f9c) feat: implement CLI configuration file (omniconfig)
* [`b0c92d5`](https://github.com/siderolabs/arges-theila/commit/b0c92d56da00529c106f042399c1163375046785) feat: implement etcd audit controller
* [`0e993a0`](https://github.com/siderolabs/arges-theila/commit/0e993a0977c711fb8767e3de2ad828fd5b9e688f) feat: properly support scaling down the cluster
* [`264cdc9`](https://github.com/siderolabs/arges-theila/commit/264cdc9e015fd87724c7a07128d1136153732540) refactor: prepare for etcd backend integration
* [`b519d17`](https://github.com/siderolabs/arges-theila/commit/b519d17971bb1c919286813b4c2465c2f5803a03) feat: show version in the UI
* [`a2fb539`](https://github.com/siderolabs/arges-theila/commit/a2fb5397f9efb22a1354c5675180ca49537bee55) feat: keep track of loadbalancer health in the controller
* [`4789c62`](https://github.com/siderolabs/arges-theila/commit/4789c62af0d1694d8d0a492cd6fb7d436e213fe5) feat: implement a new controller that can gather cluster machine data
* [`bd3712e`](https://github.com/siderolabs/arges-theila/commit/bd3712e13491ede4610ab1452ae85bde6d92b2db) fix: populate machine label field in the patches created by the UI
* [`ba70b4a`](https://github.com/siderolabs/arges-theila/commit/ba70b4a48623939d31775935bd0338c0d60ab65b) fix: rename to Omni, fix workers scale up, hide join token
* [`47b45c1`](https://github.com/siderolabs/arges-theila/commit/47b45c129160821576d808d9a46a9ec5d14c6469) fix: correct filenames for Digital Ocean images
* [`9d217cf`](https://github.com/siderolabs/arges-theila/commit/9d217cf16d432c5194110ae16a566b44b02a567e) feat: introduce new resources, deprecate `ClusterMachineTemplate`
* [`aee153b`](https://github.com/siderolabs/arges-theila/commit/aee153bedb2f7856913a54b282603b07bf20059b) fix: address style issue in the Pods paginator
* [`752dd44`](https://github.com/siderolabs/arges-theila/commit/752dd44ac42c95c644cad5640f6b2c5536a29676) chore: update Talos machinery to 1.2.0 and use client config struct
* [`88d7079`](https://github.com/siderolabs/arges-theila/commit/88d7079a6656605a1a8dfed56d392414583a283e) fix: regenerate sources from proto files that were rolled back.
* [`84062c5`](https://github.com/siderolabs/arges-theila/commit/84062c53417197417ff636a667289342089f390c) chore: update Talos to the latest master
* [`5a139e4`](https://github.com/siderolabs/arges-theila/commit/5a139e473abcdf7fd25ad7c61dad8cbdc964a453) fix: properly route theila internal requests in the gRPC proxy
* [`4be4fb6`](https://github.com/siderolabs/arges-theila/commit/4be4fb6a4e0bca29b32e1b732c227c9e7a0b1f43) feat: add support for 'talosconfig' generation
* [`9235b8b`](https://github.com/siderolabs/arges-theila/commit/9235b8b522d4bc0712012425b68ff89e455886b9) fix: properly layer gRPC proxies
* [`9a516cc`](https://github.com/siderolabs/arges-theila/commit/9a516ccb5c892ed8fe41f7cf69aaa5bb1d3fa471) fix: wait for selector of 'View All' to render in e2e tests.
* [`3cf3aa7`](https://github.com/siderolabs/arges-theila/commit/3cf3aa730e7833c0c1abe42a6afb87a85f14b58c) fix: some unhandled errors in the e2e tests.
* [`c32c7d5`](https://github.com/siderolabs/arges-theila/commit/c32c7d55c92007aa1aa10feab3c7a7de2b2afc42) fix: ignore updating cluster machines statuses without machine statuses
* [`4cfa307`](https://github.com/siderolabs/arges-theila/commit/4cfa307b85b410b44e482b259d14670b55e4a237) chore: run rekres, fix lint errors and bump Go to 1.19
* [`eb2d449`](https://github.com/siderolabs/arges-theila/commit/eb2d4499f1a3da7bc1552a6b099c28bed6fd0e4d) fix: skip the machines in `tearingDown` phase in the controller
* [`9ebc769`](https://github.com/siderolabs/arges-theila/commit/9ebc769b89a2bab37fd081e555f84e3e4c99187e) fix: allow all services to be proxied by gRPC router
* [`ea2b01d`](https://github.com/siderolabs/arges-theila/commit/ea2b01d0a0e054b259d710317fe368882534cf4c) fix: properly handle non empty resource id in the K8s resource watch
* [`3bb7da3`](https://github.com/siderolabs/arges-theila/commit/3bb7da3a0fa6b746f6a7b9aa668e055bdf825e6a) feat: show a Cluster column in the Machine section
* [`8beb70b`](https://github.com/siderolabs/arges-theila/commit/8beb70b7f045a218f9cb753e1402a07542b0bf1c) fix: ignore tearing down clusters in the `Cluster` migrations
* [`319d4e7`](https://github.com/siderolabs/arges-theila/commit/319d4e7947cb78135f5a14c02afe5814c56a312c) fix: properly handle `null` memory modules list
* [`6c2120b`](https://github.com/siderolabs/arges-theila/commit/6c2120b5ae2bd947f473d002dfe165646032e811) chore: introduce migrations manager for COSI DB state
* [`ec52139`](https://github.com/siderolabs/arges-theila/commit/ec521397946cc15929472feb7c45435fb48df848) fix: filter out invalid memory modules info coming from Talos nodes
* [`8e87031`](https://github.com/siderolabs/arges-theila/commit/8e870313a3a31d052eecf81acb522433ff98ae79) fix: bump loadbalancer timeout settings
* [`bc0ed26`](https://github.com/siderolabs/arges-theila/commit/bc0ed2672064a6bf148cd9799b35a2790f5aa7f6) feat: introduce websocket, HTTP requests monitoring
* [`857401f`](https://github.com/siderolabs/arges-theila/commit/857401f54e3922a9ab85d7dc703a5afb70c6ee45) feat: add HTTP logging (static, gateway), and websocket logging
* [`eb612a3`](https://github.com/siderolabs/arges-theila/commit/eb612a38e9c71913ebecc9f345e17844d60800b8) fix: do hard stop of events sink gRPC server after 5 seconds
* [`3162513`](https://github.com/siderolabs/arges-theila/commit/31625135e2b971d6b9f92eb4096c010113030a80) fix: populate nodes filter dropdown properly and rewrite filter function
* [`5713a51`](https://github.com/siderolabs/arges-theila/commit/5713a516391a5190fac9b7044a9f71952ce15479) fix: make `TSelectList` search filter the items in the dropdown
* [`f2519ff`](https://github.com/siderolabs/arges-theila/commit/f2519ff51b88766a907f1d7717ef74031157fd56) feat: don't allow using nodes with not enough mem for the cluster
* [`9e474d6`](https://github.com/siderolabs/arges-theila/commit/9e474d69c76a898fc5b6fcd9fdc8e87f25b7dc53) feat: show disconnected warning in the machines list
* [`fa52b48`](https://github.com/siderolabs/arges-theila/commit/fa52b48f54362c7305681ca79a7d98237531f2b4) feat: redesign Installation Media selection menu
* [`01e301a`](https://github.com/siderolabs/arges-theila/commit/01e301a875699cf6fcc887cb31cd7939338f58e9) fix: query node list using `talosctl get members` instead of K8s nodes
* [`e694df5`](https://github.com/siderolabs/arges-theila/commit/e694df59c50fbee356a48c94ade95e924ea46bb2) fix: display all available Talos versions on cluster create page
* [`7a87525`](https://github.com/siderolabs/arges-theila/commit/7a87525ed1b928a8f8e3e6a39feb4c19009ec264) fix: use `v-model` instead of callbacks in the inputs
* [`d681f5f`](https://github.com/siderolabs/arges-theila/commit/d681f5f58788612f144fa1f8d90ec6c996badb0e) feat: support scaling up the clusters
* [`e992b95`](https://github.com/siderolabs/arges-theila/commit/e992b9574d7b8f76497f46e25764618ec274af1a) feat: show notification on image download progress
* [`8ea6d9f`](https://github.com/siderolabs/arges-theila/commit/8ea6d9f1724b271919e538ed55ff6582858470f9) fix: probably fix 'context canceled' on image download
* [`692612b`](https://github.com/siderolabs/arges-theila/commit/692612b7e628588fa7608cff683c5af406f24ca7) fix: improve the Talos image generation process
* [`a69c140`](https://github.com/siderolabs/arges-theila/commit/a69c140e26f4298fcaafb1f96c389269992fc069) feat: introduce Prometheus metrics
* [`e90ca78`](https://github.com/siderolabs/arges-theila/commit/e90ca7875c501391f860f5df9f2a4e4f8e2f2d7a) fix: make grpc api listen only on siderolink interface
* [`99fc28c`](https://github.com/siderolabs/arges-theila/commit/99fc28c36c62a8d8c654c05f9b9c64ff37cedba8) fix: display correct cluster/machine status on ui
* [`eaf7655`](https://github.com/siderolabs/arges-theila/commit/eaf7655395401cd88e6bd47f4f8aa958abee30f1) fix: add a pause before integration tests
* [`19ff1c9`](https://github.com/siderolabs/arges-theila/commit/19ff1c909bedf63fe6cf2f5cc0e44f34046ca568) chore: rename download button
* [`e1c4e1b`](https://github.com/siderolabs/arges-theila/commit/e1c4e1b171eab08585a3315ca5838c88a4d2eb24) feat: add download options for all talos images
* [`24e7863`](https://github.com/siderolabs/arges-theila/commit/24e786369bfc0bb4966712296395db91751e657b) fix: delete cached clients from gRPC proxy when the cluster is destroyed
* [`58c89ef`](https://github.com/siderolabs/arges-theila/commit/58c89ef3fe621ef6909c5d38a0d47cc861667f45) feat: implement `argesctl delete` command
* [`3c99b49`](https://github.com/siderolabs/arges-theila/commit/3c99b49a9b680b091d92455a0d3bc325f8f68ca6) test: add a test which removes allocated machine
* [`75dd28f`](https://github.com/siderolabs/arges-theila/commit/75dd28f56d7ce9a92b96822a867fbfe2655cd0fa) chore: fill in resource definitions for table headers
* [`028f168`](https://github.com/siderolabs/arges-theila/commit/028f16886c41b7aa7eafb65308cc4adf4d624037) feat: End-to-end tests with playwright
* [`6be6b36`](https://github.com/siderolabs/arges-theila/commit/6be6b3605583ce8e8068746624ca86ed6adc37af) chore: bump goimports from 0.1.10 to 0.1.11 and node from 18.5.0 to 18.6.0
* [`af4da08`](https://github.com/siderolabs/arges-theila/commit/af4da086d4b709f504eda7909a36a8f0cf84e480) test: implement kernel log streaming test
* [`1eacfee`](https://github.com/siderolabs/arges-theila/commit/1eacfee2c1084040ae2201eba957409218a92c66) feat: implement argesctl machine-logs output in 'zap-like' and 'dmesg' form.
* [`96ab7ab`](https://github.com/siderolabs/arges-theila/commit/96ab7ab8317898dd45d129d5cecd2aaf1d379fba) chore: ignore memory modules with zero size
* [`fd0575f`](https://github.com/siderolabs/arges-theila/commit/fd0575ff4050702c9d07e34c7d9d5596b4ad7311) chore: retrieve k8s versions from github registry
* [`8651527`](https://github.com/siderolabs/arges-theila/commit/86515275a77741bacc790d2006f3671a5cfb27c6) feat: redo errgroup to return error on first nil error
* [`944222d`](https://github.com/siderolabs/arges-theila/commit/944222d06607079b5d982afe4b19fc1dda7f1ec2) fix: show ClusterMachineStatus.Stage in 'Clusters' view
* [`f3f6b6e`](https://github.com/siderolabs/arges-theila/commit/f3f6b6eecd3ffc13b69845dff50d2e8ab31bc0d2) chore: refactor run method and no longer ignore log receiver listener errors
* [`b316377`](https://github.com/siderolabs/arges-theila/commit/b316377b277f87a184b969b3bbf20ebe6047a0a8) chore: rename 'Dmesg' to 'Console'
* [`19ee857`](https://github.com/siderolabs/arges-theila/commit/19ee8578a6f1c1bf742699d1b5720dc4c2674c82) test: add a way to recover deleted machines
* [`e5b5bdc`](https://github.com/siderolabs/arges-theila/commit/e5b5bdc39fa6f3812b15771366f942ddcbe7f328) fix: update SideroLink library for EEXIST fixes
* [`363de69`](https://github.com/siderolabs/arges-theila/commit/363de69a50b5c1e9d07fa42152cca935844d118b) fix: spec collector equality
* [`841f3b2`](https://github.com/siderolabs/arges-theila/commit/841f3b22aacc6d2875062ef324d900c5f2091f9d) feat: add ability to supply machine config patches on the machines
* [`907ca93`](https://github.com/siderolabs/arges-theila/commit/907ca93247267d80125866c2b60225ceca3ada27) test: fix link destroy test
* [`4c9f99d`](https://github.com/siderolabs/arges-theila/commit/4c9f99d32874cdaff1eb651bf6d74ef39167c273) fix: remove machine status if the machine is in tearing down phase
* [`d9747e5`](https://github.com/siderolabs/arges-theila/commit/d9747e552e52156a9baeae962a9478231e26c566) fix: make cluster machine status test more reliable
* [`3bfff3b`](https://github.com/siderolabs/arges-theila/commit/3bfff3bb0eea9d18956dee21aff7f3de900c6b82) fix: do not set up full theila runtime during clients tests
* [`4bf33bc`](https://github.com/siderolabs/arges-theila/commit/4bf33bc9d37404a733c5039784c80e92800fb3dc) fix: immediately fail the request if the cluster is down
* [`124a5c2`](https://github.com/siderolabs/arges-theila/commit/124a5c2947978e6bc86d1b19c9eacbcf7f870b53) fix: ensure the created date on resources is set
* [`14161bf`](https://github.com/siderolabs/arges-theila/commit/14161bf3dad4484868359d186d99d9198b6eed95) feat: add scale up integration test and minor log fixes
* [`7af06fd`](https://github.com/siderolabs/arges-theila/commit/7af06fd75959eb9e807680ac8a6ba4f0a7f59255) feat: make integration tests a subtests of one global test
* [`f7c1464`](https://github.com/siderolabs/arges-theila/commit/f7c1464a1002f63daab29b36d19ea16de0cd5794) feat: implement log receiver for logs from Talos
* [`5b800ea`](https://github.com/siderolabs/arges-theila/commit/5b800ea970215fb4e100ed7b3b73d7e218fd6d86) fix: accumulate bytes received/send in the link resource
* [`b3b1e9b`](https://github.com/siderolabs/arges-theila/commit/b3b1e9bbfbf62632dc0d8c2239a72793883101ce) feat: machine removal
* [`fb01bc4`](https://github.com/siderolabs/arges-theila/commit/fb01bc4b26c5b37f15bac923450e1f58fb7a3d89) fix: use Talos 1.2.0
* [`3a50efe`](https://github.com/siderolabs/arges-theila/commit/3a50efe363c4724f369a02f672848ad7c284847c) feat: filter machines that can be added to cluster
* [`ba62db5`](https://github.com/siderolabs/arges-theila/commit/ba62db521b47049e92557bf8cfc5f737e496bf57) fix: properly parse `siderolink-api-advertised-url` if there's no port
* [`96f835a`](https://github.com/siderolabs/arges-theila/commit/96f835a91136f62d9dbdf5c1d1c46c729d57e51e) fix: properly display node selectors in FireFox
* [`12c20a4`](https://github.com/siderolabs/arges-theila/commit/12c20a42c9dfdea5f88e0e7942fbdb42ea543b95) fix: populate disks when machines are connected during cluster create
* [`0dc97f8`](https://github.com/siderolabs/arges-theila/commit/0dc97f8696a7c571d5318daf794700342e06f639) fix: adjust overview page to look closer to the mockups
* [`2b77af8`](https://github.com/siderolabs/arges-theila/commit/2b77af8d39e555970487c3265dfbd63412e90d2f) feat: add the chart showing the count of clusters
* [`a1dff65`](https://github.com/siderolabs/arges-theila/commit/a1dff6589d64207e6e7331d0407e7857f9c4079d) feat: implement ISO download with embedded kernel args
* [`37c03d8`](https://github.com/siderolabs/arges-theila/commit/37c03d8cb04b02e79f42e70eeea1e4368445604d) test: pull kubeconfig and interact with Kubernetes API
* [`75bfb08`](https://github.com/siderolabs/arges-theila/commit/75bfb08f0738fc9f67259caf12902db67860370f) fix: ignore the error on splitting host/port
* [`3be5a32`](https://github.com/siderolabs/arges-theila/commit/3be5a3254168cddec8f1629789c2ae50d9eaa08e) feat: make the whole cluster list item clickable, add dropdown menu item
* [`2c9dc99`](https://github.com/siderolabs/arges-theila/commit/2c9dc99000266b3d4c139f27dea4f6283709251e) fix: adjust the look of the Overview page a bit
* [`aa4a926`](https://github.com/siderolabs/arges-theila/commit/aa4a926cbb85bf63312493b937440a174aed5070) feat: add the button for downloading cluster Kubeconfig on overview page
* [`4532de6`](https://github.com/siderolabs/arges-theila/commit/4532de6f3d514a534c38a63731c43075698f5c01) feat: support basic auth in `argesctl` command
* [`b66bb3c`](https://github.com/siderolabs/arges-theila/commit/b66bb3cbcc85d7be4348ecd9a6d5d62f72a90e11) feat: add summary information Overview page
* [`3bdbce4`](https://github.com/siderolabs/arges-theila/commit/3bdbce41a3ed89a42556d837bc0c5cfe417e22e6) test: more cluster creation tests, two clusters, cleanup
* [`3b00bd5`](https://github.com/siderolabs/arges-theila/commit/3b00bd5bf417c5c9cb42471d27811c1849a40c78) fix: improve cluster deletion and node reset flow
* [`2d83d16`](https://github.com/siderolabs/arges-theila/commit/2d83d1694ec73da818004f91ede76a0bca30fe79) test: create a cluster and verify cluster machine statuses
* [`f471cfd`](https://github.com/siderolabs/arges-theila/commit/f471cfdcf7c9e70f37436e173c3a58c1965e8bb2) fix: copy all labels from the `ClusterMachine` to `ClusterMachineStatus`
* [`ec32f86`](https://github.com/siderolabs/arges-theila/commit/ec32f8632db104efd6fedc5421179175274d6339) test: add integration tests up to the cluster creation
* [`a8d3ee5`](https://github.com/siderolabs/arges-theila/commit/a8d3ee5b14a57ad1d9d88512a95032bbda61e734) feat: add kubeconfig command to argesctl and fix kubeconfig
* [`10b9a3b`](https://github.com/siderolabs/arges-theila/commit/10b9a3ba676a636e488805ed04a0c908c3d2cf53) test: implement API integration test
* [`3e6b891`](https://github.com/siderolabs/arges-theila/commit/3e6b8913f916dc5e8ac3ef49e14648defa6e1bf6) feat: aggregate cluster machine statuses in cluster status controller
* [`f6cbc58`](https://github.com/siderolabs/arges-theila/commit/f6cbc58a91124833f0cbae4ecd0c0416acbe8bfa) chore: ignore empty processor info
* [`c5fc71b`](https://github.com/siderolabs/arges-theila/commit/c5fc71b86a5492d548ae9098c5c74de240ebd800) fix: clean up Kubernetes client and configs when a cluster is destroyed
* [`e8478fe`](https://github.com/siderolabs/arges-theila/commit/e8478fe5280d5e8a32bb423ec96edacadabc7e43) fix: properly use tracker to cleanup `ClusterMachineConfig` resources
* [`044fcad`](https://github.com/siderolabs/arges-theila/commit/044fcadb66de61742ab871d10f3fcf0f453f6e27) fix: make `MachineStatusController` connect to configured nodes
* [`2867099`](https://github.com/siderolabs/arges-theila/commit/2867099a52d651c3b0f9d3abbae266f2792cafe7) feat: add api endpoint to fetch kubeconfig
* [`5f32667`](https://github.com/siderolabs/arges-theila/commit/5f3266747012b590dd7a7d0ebc23ee0e80abb2ab) test: support registry mirrors for development purposes
* [`5114695`](https://github.com/siderolabs/arges-theila/commit/5114695cfeb0b6c792002ff5f0f31c1944c269ab) refactor: consistent flag naming
* [`9ffb19e`](https://github.com/siderolabs/arges-theila/commit/9ffb19e77968c6e411903a2c59fd9a18063b46d4) chore: use latest node
* [`5512321`](https://github.com/siderolabs/arges-theila/commit/5512321f05b6b657a28abc25470664f6eb6e3d0a) refactor: set better defaults for cli args
* [`ff88242`](https://github.com/siderolabs/arges-theila/commit/ff882427f56e42039b79900380b61b86d3290269) chore: mark 'siderolink-wireguard-endpoint' flags as required
* [`4a9d9ad`](https://github.com/siderolabs/arges-theila/commit/4a9d9adef1e521d3c0293b6dc414f572bd8a93d4) feat: add the ClusterMachineStatus resource
* [`e4e8b62`](https://github.com/siderolabs/arges-theila/commit/e4e8b6264cb48edd014f97129f52aefaa129fd63) refactor: unify all Arges API under a single HTTP server
* [`5af9049`](https://github.com/siderolabs/arges-theila/commit/5af9049bdc2e09bf410e1b0646e4e08a4366f33b) chore: rename sidebar item
* [`a4fc47f`](https://github.com/siderolabs/arges-theila/commit/a4fc47f97d79259532b91a8d391e84b59554ed8e) chore: fix build warning
* [`547b83c`](https://github.com/siderolabs/arges-theila/commit/547b83c4a2a543d5b6ce4dca6cf6f5de87c33dcb) chore: bump siderolink version
* [`11c31f3`](https://github.com/siderolabs/arges-theila/commit/11c31f39d834e3352b086c1aec665065fd74e944) refactor: drop one of the layered gRPC servers
* [`0adbbb7`](https://github.com/siderolabs/arges-theila/commit/0adbbb7edfeacedd98a7e84c2f45ac458750a281) feat: introduce a way to copy kernel arguments from the UI
* [`ce5422a`](https://github.com/siderolabs/arges-theila/commit/ce5422a27771a94cc25be70ec756711d140b2758) fix: import new COSI library to fix YAML marshaling
* [`d6cec09`](https://github.com/siderolabs/arges-theila/commit/d6cec099cb6f4c3118e4263b9517176858bb9cfb) feat: implement Arges API client, and minimal `argesctl`
* [`65c8d68`](https://github.com/siderolabs/arges-theila/commit/65c8d683187d82dc730752294c1bc03657f5df78) feat: implement cluster creation view
* [`8365b00`](https://github.com/siderolabs/arges-theila/commit/8365b00df90ac55f99e0f82e1fa6d4367ebd6a3f) feat: re-enable old Theila UI
* [`63e703c`](https://github.com/siderolabs/arges-theila/commit/63e703c4e1dfb4bf645fbc9cd28ba2a722e04dc2) fix: update Talos to the latest master
* [`d33e27b`](https://github.com/siderolabs/arges-theila/commit/d33e27b49113729c5538fce688832152ff96a7ea) feat: implement clusters list view
* [`cb9e23c`](https://github.com/siderolabs/arges-theila/commit/cb9e23ca6f420ac7b71acf6b19e9012265f3c69b) feat: protect Theila state from external API access
* [`952c235`](https://github.com/siderolabs/arges-theila/commit/952c2359b32fdd077d85e312707f8b9c9e01ea0c) fix: properly allocated ports in the loadbalancer
* [`a58c479`](https://github.com/siderolabs/arges-theila/commit/a58c479e9e31f70e806a1f3482b9b984c5c0ca68) chore: report siderolink events kernel arg
* [`8a56fe3`](https://github.com/siderolabs/arges-theila/commit/8a56fe34ce1966fe28f9e432c696fdd779dfb638) refactor: move Theila resources to public `pkg/`
* [`1251699`](https://github.com/siderolabs/arges-theila/commit/12516996eda859db6677403ad1f72a3994ea180b) fix: reset the `MachineEventsSnapshot` after the node is reset
* [`9a2e6af`](https://github.com/siderolabs/arges-theila/commit/9a2e6af3113b795f57c4e3a86c1348b120fa3bbd) feat: implement bootstrap controller
* [`7107e27`](https://github.com/siderolabs/arges-theila/commit/7107e27ee6b9ba644fc803e4463cbfcf26cf97de) feat: implement apply and reset config controller
* [`1579eb0`](https://github.com/siderolabs/arges-theila/commit/1579eb09eb58f2cb679205e9e204369f3a362e07) feat: implement machine events handler and `ClusterStatus`
* [`7214f4a`](https://github.com/siderolabs/arges-theila/commit/7214f4a514a921d6b9df7515116613996416f383) feat: implement cluster load balancer controller
* [`9c4fafa`](https://github.com/siderolabs/arges-theila/commit/9c4fafaf6b8dc9b7ff08fe28704ca6a2e7efc097) feat: add a controller that manages load balancers for talos clusters
* [`7e3d80c`](https://github.com/siderolabs/arges-theila/commit/7e3d80ce956d621ed79e4db094808831e18db85b) feat: add a resources that specify configurations for load balancers
* [`dc0d356`](https://github.com/siderolabs/arges-theila/commit/dc0d356a181b4c37670d2ed4e8d7af370dccef60) feat: support Theila runtime watch with label selectors
* [`6a568a7`](https://github.com/siderolabs/arges-theila/commit/6a568a72922e34e91f5448d3c1caa2f0b3a02e96) feat: implement `ClusterMachineConfig` resource and it's controller
* [`3db0f1c`](https://github.com/siderolabs/arges-theila/commit/3db0f1c9d4e2d6f962b6f3216a4f9c7e2575dd21) feat: implement `TalosConfig` controller
* [`b7ae8e1`](https://github.com/siderolabs/arges-theila/commit/b7ae8e113dc68acd87c4cfe5e3c8349d32bc392d) feat: introduce `Cluster` controller that adds finalizers on Clusters
* [`8d7ea02`](https://github.com/siderolabs/arges-theila/commit/8d7ea0293e8f57388fd483dc82e79e6b4c76a53f) chore: use label selectors in `TalosConfig`, set labels on the resources
* [`cff9cb1`](https://github.com/siderolabs/arges-theila/commit/cff9cb19ba8718fdad509b5e91cb8221c6c1ff00) fix: separate advertised endpoint from the actual wireguard endpoint
* [`5be6cc3`](https://github.com/siderolabs/arges-theila/commit/5be6cc391adf8bcb58b8d47f09dad5aa75d1ad98) feat: implement cluster creation UI
* [`a1633eb`](https://github.com/siderolabs/arges-theila/commit/a1633eb18772b9e99d687dfddd12fc09fd1ea5c4) chore: add typed wrappers around State, Reader and Writer
* [`5515f3d`](https://github.com/siderolabs/arges-theila/commit/5515f3d004f54455a1eb1f4977bbb9d663fd1bca) feat: add `ClusterSecrets` resource and controller and tests
* [`7226f6c`](https://github.com/siderolabs/arges-theila/commit/7226f6cdc60eeb4d6040d1aa0711fed378c50b33) feat: add `Cluster`, `ClusterMachine` and `TalosConfig` resources
* [`ec44930`](https://github.com/siderolabs/arges-theila/commit/ec44930672ca8954c6ba68975c1799a087ec0c43) feat: enable vtprotobuf optimized marshaling
* [`15be219`](https://github.com/siderolabs/arges-theila/commit/15be2198872fb637f7ba2e1ff550e4466179f2b1) feat: generate TS constants from go `//tsgen:` comments
* [`caa4c4d`](https://github.com/siderolabs/arges-theila/commit/caa4c4d285dcd1176a70d87f28ee303cd0483ca8) fix: resource equality for proto specs
* [`beeca88`](https://github.com/siderolabs/arges-theila/commit/beeca886213332f313f7f3a477d7e7c508e6d058) refactor: clarify code that creates or gets links for nodes
* [`340c63a`](https://github.com/siderolabs/arges-theila/commit/340c63ad4ba918d4b11ab1f57fdbd3b5e5d8b3dc) feat: implement `Machines` page
* [`f7bc0c6`](https://github.com/siderolabs/arges-theila/commit/f7bc0c69c69fe515cfa729bc062c730756a53019) feat: accept nodes if they provide the correct join token
* [`bdf789a`](https://github.com/siderolabs/arges-theila/commit/bdf789a35da5491a4fcbd2af35a1c6efd22ab1fc) feat: immediately reconnect SideroLink peers after Arges restart
* [`6b74fa8`](https://github.com/siderolabs/arges-theila/commit/6b74fa82ca5757d6f3809853c1ac3e7754efb06d) feat: implement MachineStatusController
* [`f5db0e0`](https://github.com/siderolabs/arges-theila/commit/f5db0e05a87d5c11b4a1029b14020b19ca67035d) feat: add more info to the siderolink connection spec
* [`d3e4a71`](https://github.com/siderolabs/arges-theila/commit/d3e4a71af8fd79328e4edda6d9642b83902b2003) refactor: simplify the usage of gRPC resource CRUD API
* [`2430115`](https://github.com/siderolabs/arges-theila/commit/2430115af1aaac4226b7d5821e1fe706a1088501) feat: implement MachineController and small fixes
* [`e31d22d`](https://github.com/siderolabs/arges-theila/commit/e31d22d7639753df53c130461ae1f96b9126f3a5) feat: support running Theila without contexts
* [`a6b3646`](https://github.com/siderolabs/arges-theila/commit/a6b364626bd808687d5ad95307766344b16dd042) refactor: small fixes
* [`33d2b59`](https://github.com/siderolabs/arges-theila/commit/33d2b59c202f03785580209c885aa297c023fa60) refactor: clean up a bit SideroLink code, fix shutdown
* [`98ec883`](https://github.com/siderolabs/arges-theila/commit/98ec8830308755c7073a5d4510483e97d8e1d02d) chore: rename main executable to avoid clashing with Theila project
* [`828721d`](https://github.com/siderolabs/arges-theila/commit/828721d9aa5d912cce628256f75579309d1ad67d) feat: enable COSI persistence for resources
* [`f1f7883`](https://github.com/siderolabs/arges-theila/commit/f1f788344254e18bcab00a25b56a86289bfb1638) feat: set up siderolink endpoints in Theila
* [`6439335`](https://github.com/siderolabs/arges-theila/commit/64393353ca7cf430f82bfe73a004da319da28261) refactor: migrate to `typed.Resource` in Theila internal state
* [`6195274`](https://github.com/siderolabs/arges-theila/commit/61952742a47ea89e89228f057d0d3de351766150) refactor: restructure folders in the project
* [`1abf72b`](https://github.com/siderolabs/arges-theila/commit/1abf72b4b2e382fe0cf9302b42242152c255a3ee) chore: update Talos libs to the latest version
* [`16dffd9`](https://github.com/siderolabs/arges-theila/commit/16dffd9058570477b3a648896a89e6445e5b0162) fix: display delta time for pod's age
* [`8b80726`](https://github.com/siderolabs/arges-theila/commit/8b807262b23cfa830f3ff444d49f11b3a1654703) feat: update favicon to sidero logo
* [`2da7378`](https://github.com/siderolabs/arges-theila/commit/2da737841c2ae0bf1f1f916dc6f45b1e3996d6e4) feat: show the extended hardware info
* [`d3c6004`](https://github.com/siderolabs/arges-theila/commit/d3c6004f9767bf0cff9191dc130308c848ede077) chore: allow getting resources without version and group
* [`eb19087`](https://github.com/siderolabs/arges-theila/commit/eb190875b30275195e52f1a95ed0bb3aae08424f) fix: remove t-header error notification
* [`5a28202`](https://github.com/siderolabs/arges-theila/commit/5a28202c939ef9683d14fb3d873e0bacb35577db) feat: restyle t-alert component
* [`9f2b482`](https://github.com/siderolabs/arges-theila/commit/9f2b48228bbfa39d33b07ae43e9fdb34192c3eed) fix: get rid of racy code in the kubeconfig request code
* [`c40824e`](https://github.com/siderolabs/arges-theila/commit/c40824ecc5d10cb5289e133b8b1f51213aa12f7f) feat: add text Highlight feature
* [`9018c81`](https://github.com/siderolabs/arges-theila/commit/9018c81bd0d7c58bb5c632c06f3c3904f6674e03) feat: use `~/.talos/config` as a primary source for clusters
* [`e10547b`](https://github.com/siderolabs/arges-theila/commit/e10547b5761ad96ab8b5766fe5c3f06fcdf86477) refactor: remove old components and not used code parts
* [`f704684`](https://github.com/siderolabs/arges-theila/commit/f7046846ea8e83a0e39647c4fcc49addf4c56061) fix: properly calculate servers capacity
* [`755a077`](https://github.com/siderolabs/arges-theila/commit/755a0779014b0a4177e0fc5180db20720be5a814) fix: use proper units for memory and CPU charts on the node monitor page
* [`d0a083d`](https://github.com/siderolabs/arges-theila/commit/d0a083d1c15c319e236dd258fabcc9a231f797a1) release(v0.2.0-alpha.0): prepare release
* [`53878ee`](https://github.com/siderolabs/arges-theila/commit/53878eea09c18f2bc0dd55ca11a6743587748319) fix: properly update servers menu item when the context is changed
* [`b4cb9c7`](https://github.com/siderolabs/arges-theila/commit/b4cb9c7989ec5299785b86acb3fa0ee648efd259) feat: restyle TMonitor page
* [`f0377e2`](https://github.com/siderolabs/arges-theila/commit/f0377e2ad5da702af71f2706141f4d7c638c7a15) fix: invert chart value for cpu, storage and memory on the overview page
* [`6ea6ecf`](https://github.com/siderolabs/arges-theila/commit/6ea6ecf12c4d8b5253b4dfc2e64f5b5d787d022a) fix: update capi-utils to fix talosconfig requests for CAPI clusters
* [`e3796d3`](https://github.com/siderolabs/arges-theila/commit/e3796d3876d33248fd0998901273a14d29a487a3) chore: update capi-utils
* [`39186eb`](https://github.com/siderolabs/arges-theila/commit/39186ebe50da531f35d21ac2488f8a58c1ef8e78) feat: implement overview page, cluster dropdown, ongoing tasks
* [`59f2b27`](https://github.com/siderolabs/arges-theila/commit/59f2b27be4d7f5a591fdeae533d649494356250d) docs: update README.md
* [`2b7831f`](https://github.com/siderolabs/arges-theila/commit/2b7831f2d22106ac8a82f890d73c2705841b0739) feat: add Kubernetes and Servers pages
* [`4451a5b`](https://github.com/siderolabs/arges-theila/commit/4451a5bc9f5c6b058c6bcf1252b7c83a001cafbe) fix: properly set TaskStatus namespace in the initial call
* [`4545464`](https://github.com/siderolabs/arges-theila/commit/454546425f2fd7e4418aa8a03465f3a062de804e) fix: add new fields to the TaskStatus spec, update Talos
* [`891cf3b`](https://github.com/siderolabs/arges-theila/commit/891cf3b79c8430deeed8a168955afd6e97083baa) docs: describe client context types, usage
* [`309b515`](https://github.com/siderolabs/arges-theila/commit/309b51545ead2ee144244591df2e5ead2849fb11) feat: update k8s upgrades tasks structure for the new UI representation
* [`5aa8ca2`](https://github.com/siderolabs/arges-theila/commit/5aa8ca24bd3159879c46c8e8a134702b174e3362) feat: add NodesPage
* [`db434e0`](https://github.com/siderolabs/arges-theila/commit/db434e07b9f23562bd746a0f78e3868b079006e2) feat: add TPagination component
* [`0b51727`](https://github.com/siderolabs/arges-theila/commit/0b51727efed31f13f52fa20b360071e7e2a6d9eb) feat: add Pods, Dashboard, Upgrade views, etc
* [`c549b8b`](https://github.com/siderolabs/arges-theila/commit/c549b8b9ee8a563f14b2e791f91a7b3cb0430aa7) feat: add Overview and Upgrade Kubernetes pages
* [`cec2e85`](https://github.com/siderolabs/arges-theila/commit/cec2e854f4f3999109220902bccaee6c25d1f502) chore: define constants for all used resource types
* [`962bdaf`](https://github.com/siderolabs/arges-theila/commit/962bdaf6406ab8e5febea0ad8d32da9c86fa39e7) feat: add TSideBar
* [`fa28ccb`](https://github.com/siderolabs/arges-theila/commit/fa28ccb67f52c1dd9096b23388427d78be526275) feat: add TheHeader component
* [`f3418a5`](https://github.com/siderolabs/arges-theila/commit/f3418a59e38e551bd0be7cc7ae66ef4645719aa7) feat: button;icons;config
* [`db30f50`](https://github.com/siderolabs/arges-theila/commit/db30f503730bdbd8ed359d4070dea0214df67fcd) fix: add `frontend/node_modules` to gitignore
* [`a675b86`](https://github.com/siderolabs/arges-theila/commit/a675b86f7d55cecd4ae1277cbf057a6bc264940c) fix: properly pass label selector to the metadata in ClusterListItem
* [`7911d6a`](https://github.com/siderolabs/arges-theila/commit/7911d6a31abdb51e86586a025b705ddfeb1dd19e) chore: add ability to start local development server for the frontend
* [`076fee1`](https://github.com/siderolabs/arges-theila/commit/076fee10c6583dc49e6530b02cab1f757da0e853) feat: use CAPI utils for CAPI requests
* [`5ed5ba2`](https://github.com/siderolabs/arges-theila/commit/5ed5ba2a122585a97cf65c3ff081126752cd26fa) fix: more websocket client bugfixes
* [`6fe22ad`](https://github.com/siderolabs/arges-theila/commit/6fe22ad370026380ba75b38e261870addc341e6f) fix: reset reconnect timeouts after the client is reconnected
* [`c4b144a`](https://github.com/siderolabs/arges-theila/commit/c4b144af272a46dbdc8d1bb35784e09ba1b79987) fix: talosconfig/kubeconfig when using the default context
* [`b439a37`](https://github.com/siderolabs/arges-theila/commit/b439a371c13a8d46d986a1dae3d6f4b7cba4a298) fix: properly handle Same-Origin header in websockets
* [`ffffed1`](https://github.com/siderolabs/arges-theila/commit/ffffed100cec18209bae723b9919eb8613950649) fix: read node name from nodename resource instead of hostname
* [`2d6f984`](https://github.com/siderolabs/arges-theila/commit/2d6f9844440a6d18b3093dea6228ac6a237dc86b) fix: use secure websockets if the page itself is using https
* [`799f2d2`](https://github.com/siderolabs/arges-theila/commit/799f2d2d00762d5270dd4a3f4b4b312b32dbb7dd) feat: rework the node overview page
* [`0d0eaf4`](https://github.com/siderolabs/arges-theila/commit/0d0eaf4b2721dfa1b04bce24e4a1e476579e3a74) fix: make charts height resize depending on the screen height
* [`7de0101`](https://github.com/siderolabs/arges-theila/commit/7de0101bf0e613653caadd5733db0e29a6bb5bfb) fix: use polyfill to fix streaming APIs on Firefox
* [`0cff2b0`](https://github.com/siderolabs/arges-theila/commit/0cff2b02b5d8b2c2c644067cf6bd3ed573cb784d) feat: small UI adjustments
* [`d70bd41`](https://github.com/siderolabs/arges-theila/commit/d70bd41992e13fb3dacc1740532083a8f6ce9afa) feat: implement accept Sidero server functional
* [`f3a6e16`](https://github.com/siderolabs/arges-theila/commit/f3a6e16a79e1bca9ea6c87eb0d3e0f2a6c65ff2e) feat: add top processes list to the Overview page
* [`3cf97e4`](https://github.com/siderolabs/arges-theila/commit/3cf97e4b9e07f8383da8a6fb7a993b70c8f82503) refactor: use the same object for gRPC metadata context and messages
* [`243206f`](https://github.com/siderolabs/arges-theila/commit/243206f95aa6ba944bd4361db6274e7072bae1fc) release(v0.1.0-alpha.2): prepare release
* [`e5b6f29`](https://github.com/siderolabs/arges-theila/commit/e5b6f29fd298904e06284a67681cc0ce5135145f) feat: implement node Reset
* [`bcb7d23`](https://github.com/siderolabs/arges-theila/commit/bcb7d237c31f42a35f5c3b53e7615ddae1ce0a8b) fix: node IP not being truncated
* [`e576d33`](https://github.com/siderolabs/arges-theila/commit/e576d33ba40f629eed14668f2d9bf77d7fef62c2) feat: add upgrade UI for CAPI clusters
* [`10cdce7`](https://github.com/siderolabs/arges-theila/commit/10cdce7fcc219af969a85a41d18fb904936faa0a) fix: server labels key/value order and chevron orientation
* [`4007177`](https://github.com/siderolabs/arges-theila/commit/40071775d6de1eea697f67e55441c384c86e75d9) feat: implement Kubernetes upgrade UI components
* [`f4917ee`](https://github.com/siderolabs/arges-theila/commit/f4917eecfb3173acf7518883c738118c8537d657) fix: accumulate chart updates into a single update
* [`414d76c`](https://github.com/siderolabs/arges-theila/commit/414d76c1c926695e5d66787b34decae92e151b45) feat: implement upgrade controller
* [`36742ea`](https://github.com/siderolabs/arges-theila/commit/36742ea5ab1e8a983b73f73443c1cf122a90d054) feat: introduce create, delete and update gRPC APIs
* [`2b3d314`](https://github.com/siderolabs/arges-theila/commit/2b3d314a460b385d8c13bdd025fadb37b5508bdc) feat: install internal COSI runtime alongside with K8s and Talos
* [`ae7f784`](https://github.com/siderolabs/arges-theila/commit/ae7f784d08621d18075b1763f026a7513d9d9dcb) refactor: move all generated TypeScript files under `frontend/src/api`
* [`61bad64`](https://github.com/siderolabs/arges-theila/commit/61bad64540c28fb0520a39a6c64d64c3e9353361) release(v0.1.0-alpha.1): prepare release
* [`8e5e722`](https://github.com/siderolabs/arges-theila/commit/8e5e7229470713d2fbd5ad0df027bd825f5481e3) feat: implement node reboot controls
* [`9765a88`](https://github.com/siderolabs/arges-theila/commit/9765a88069f05c49f5a7d854675ee37e1c7a8273) feat: dmesg logs page
* [`ecbbd67`](https://github.com/siderolabs/arges-theila/commit/ecbbd67936b1fb570d706fe3b93b81f6089b5124) feat: use updated timestamp to display event time on the graph
* [`7c56773`](https://github.com/siderolabs/arges-theila/commit/7c56773448a496fe1ceeec3c47978975ce336b3a) refactor: use Metadata to pass context in all gRPC calls
* [`abb4733`](https://github.com/siderolabs/arges-theila/commit/abb47330222217d7d8b5c36ff28902415bc755d8) feat: implement service logs viewer
* [`8e8e032`](https://github.com/siderolabs/arges-theila/commit/8e8e032b20d082bfd71a26c2af2bbc821d9c2a7b) feat: add ability to pick sort order on the servers page
* [`1a1c728`](https://github.com/siderolabs/arges-theila/commit/1a1c728ac929bb02db7f1bd0b991a747e63fe81a) fix: resolve the issue with idFn value generating undefined ids
* [`2e83fe2`](https://github.com/siderolabs/arges-theila/commit/2e83fe23a7feb51b73bc7b53997636b641ae42b9) feat: allow filtering servers by picking from predefined categories
* [`48f776e`](https://github.com/siderolabs/arges-theila/commit/48f776e10f6c79772481393d7397557419520046) fix: navigate home when changing the context
* [`a1ce0ca`](https://github.com/siderolabs/arges-theila/commit/a1ce0ca8c8fabb2267c3dc6f6b1509f131e18ba8) fix: resolve services search issues
* [`5b768f8`](https://github.com/siderolabs/arges-theila/commit/5b768f85277ee31131994ae0b253700a5d26978d) feat: make stacked lists searchable
* [`ec1bc5b`](https://github.com/siderolabs/arges-theila/commit/ec1bc5b48943e473c756ebc7a8c943a34cdeaeac) feat: implement stats component and add stats to the servers page
* [`1a85999`](https://github.com/siderolabs/arges-theila/commit/1a8599981f93fc5ce68e23b1b4cd7aabbb43c90c) feat: align Sidero servers list outlook with the wireframes
* [`524264c`](https://github.com/siderolabs/arges-theila/commit/524264c515a9efdce9f06a3c2ebd59c2979f9b2a) fix: display error message and use proper layout for the spinner
* [`5263d16`](https://github.com/siderolabs/arges-theila/commit/5263d16cfb936aad9ba461e0cc7b150ff9b806d5) feat: introduce node stats page
* [`8feb35e`](https://github.com/siderolabs/arges-theila/commit/8feb35e95a6d588e1d9c605231308976be452a2e) feat: make root sidebar sections collapsible
* [`36ad656`](https://github.com/siderolabs/arges-theila/commit/36ad656a3bbdc1e2915a87c0d09c31738ae3f3c4) feat: detect cluster capabilities
* [`a25d90d`](https://github.com/siderolabs/arges-theila/commit/a25d90d58a85b3b73432858f134fa09cd1338d5c) feat: support switching context in the UI
* [`67903e2`](https://github.com/siderolabs/arges-theila/commit/67903e23f49623ae9a9a6b297282c62aa8579aa8) refactor: separate Watch from StackedList
* [`76b9e1d`](https://github.com/siderolabs/arges-theila/commit/76b9e1dc88cccf74cebb28470eae5e9249809d40) release(v0.1.0-alpha.0): prepare release
* [`7bde4c8`](https://github.com/siderolabs/arges-theila/commit/7bde4c8c6e16c197578cbb4e037a05d50194958f) fix: cobra command was initialized but not actually used
* [`04624c9`](https://github.com/siderolabs/arges-theila/commit/04624c95cec587ae0b0d8888d95d484ef8d98cfa) feat: support getting Talos and Kubernetes client configs for a cluster
* [`219b9c8`](https://github.com/siderolabs/arges-theila/commit/219b9c8663fe03af65796b0b6299cff5e66b3efc) feat: implement notifications component
* [`f8b19a0`](https://github.com/siderolabs/arges-theila/commit/f8b19a0585e6e19c0e7da4e4afad5bbd264e0029) feat: decouple watch list from the view
* [`2f8c96e`](https://github.com/siderolabs/arges-theila/commit/2f8c96e44012e7bd0db9869eeb90ab48ff41e162) feat: implement appearance settings modal window
* [`de745d6`](https://github.com/siderolabs/arges-theila/commit/de745d6b7170a9c509cc835a8b675a1c788e80f4) feat: implement Talos runtime backend
* [`af69a0d`](https://github.com/siderolabs/arges-theila/commit/af69a0d58906a86974bc7dbec2c09ca9f78b152f) feat: support getting Kubernetes resource through gRPC gateway
* [`2c50010`](https://github.com/siderolabs/arges-theila/commit/2c50010b0d9f7b168354fedd698600d94123c354) feat: implement breadcrumbs component, add support for table header
* [`3fc1e80`](https://github.com/siderolabs/arges-theila/commit/3fc1e808875f6f502cd2657c4548dd886fbf465d) feat: implement nodes view
* [`961e93a`](https://github.com/siderolabs/arges-theila/commit/961e93a4af430eaa9efcd1e2922af8072fe4cf85) feat: implement clusters view
* [`e8248ff`](https://github.com/siderolabs/arges-theila/commit/e8248ffab89633cae8834631e39cf4dce5e4147a) feat: use plain zap instead of SugaredLogger everywhere
* [`81ba93d`](https://github.com/siderolabs/arges-theila/commit/81ba93dffdc37efdde06557a1c63511a7d61b2f2) chore: generate websocket protocol messages using protobuf
* [`37a878d`](https://github.com/siderolabs/arges-theila/commit/37a878dd396b650df8afaf6730f9afe52d35569c) feat: make JS websocket reconnect on connection loss
* [`23b3281`](https://github.com/siderolabs/arges-theila/commit/23b3281f8880800a9084e1c8a74617fcf966c846) feat: use dynamic watcher to allow listing any kinds of resources
* [`16475f5`](https://github.com/siderolabs/arges-theila/commit/16475f51cc9651736213b36c57381b24dcabdc62) feat: implement real time update server on top of web sockets
* [`76b39ae`](https://github.com/siderolabs/arges-theila/commit/76b39ae563d9f09ecac3451389e3d260abdad48d) feat: create hello world Vue app using Kres
* [`baab493`](https://github.com/siderolabs/arges-theila/commit/baab493f155cbd78c2e8af6ce45268c40ef6aeed) Initial commit
</p>
</details>

### Dependency Changes

This release has no dependency changes

