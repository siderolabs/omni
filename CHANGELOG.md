## [Omni 0.32.0](https://github.com/siderolabs/omni/releases/tag/v0.32.0) (2024-03-28)

Welcome to the v0.32.0 release of Omni!



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### Maintenance Mode Configs

Talos nodes which do not have Omni SideroLink parameters can now be joined by applying the maintenance machine config with the Omni parameters.
Omni now properly handles them and preserves them over resets.


### `omnictl support`

`omnictl` CLI tool now has support bundle collection utility.
It collects all cluster related resources from Omni and includes all data which can be collected by `talosctl support` command.


### Omni Workload Proxy

Kubernetes services exposed through Omni now open on a new page, instead of using an iframe.
Iframe often doesn't work due to headers restrictions.


### Contributors

* Artem Chernyshev
* Utku Ozdemir
* Andrey Smirnov
* Dmitriy Matrenichev
* Noel Georgi

### Changes
<details><summary>12 commits</summary>
<p>

* [`2f5b21b`](https://github.com/siderolabs/omni/commit/2f5b21ba2547bd8eeee0d7806ad2b210de670f48) release(v0.32.0): prepare release
* [`910cbf2`](https://github.com/siderolabs/omni/commit/910cbf2c3a8aa9333eb529f54e8bc7727e1fc460) release(v0.32.0-beta.0): prepare release
* [`176f9d9`](https://github.com/siderolabs/omni/commit/176f9d9f57530832a9ebbb64d008bc98300b2cc7) feat: compute schematic id only from the extensions
* [`1e4e303`](https://github.com/siderolabs/omni/commit/1e4e303c098fa18bd63913d7a1d09538ed637cd0) feat: implement `omnictl support` command
* [`a835cc7`](https://github.com/siderolabs/omni/commit/a835cc730c9bedd7f16a9a8b5c2c38464b71e189) fix: fix error handling in image pre pull task
* [`2d1b776`](https://github.com/siderolabs/omni/commit/2d1b776f6c61db029a72e62deef81eafe34c29e9) fix: properly handle upgrades for the machines with invalid schematics
* [`4db7630`](https://github.com/siderolabs/omni/commit/4db76307924bf86bab458b029923ddd98a22ad0e) feat: add the context menu for copying the machine id
* [`5a8abf5`](https://github.com/siderolabs/omni/commit/5a8abf584edd605ac7bfc6f72b31142c857128c4) fix: get rid of the issue with `MachineSets` stuck in `Reconfiguring`
* [`8173377`](https://github.com/siderolabs/omni/commit/8173377c122e5ff0c6e710362f8481ce01ca929d) feat: preserve maintenance machine configs
* [`190218a`](https://github.com/siderolabs/omni/commit/190218ad2fcf07f94d0d55178ad08a21b2bc3bfa) feat: open exposed services in a new window
* [`0960100`](https://github.com/siderolabs/omni/commit/0960100f11b4229d67c84de5d136d6f56d5a05b0) chore: drop integration binary from releases
* [`6e3ba5c`](https://github.com/siderolabs/omni/commit/6e3ba5c389622987028af4e5ee50ac638f70c32e) chore: bump Go, build arm64 container images, rekres
</p>
</details>

### Changes since v0.32.0-beta.0
<details><summary>1 commit</summary>
<p>

* [`2f5b21b`](https://github.com/siderolabs/omni/commit/2f5b21ba2547bd8eeee0d7806ad2b210de670f48) release(v0.32.0): prepare release
</p>
</details>

### Changes from siderolabs/crypto
<details><summary>1 commit</summary>
<p>

* [`1c94bb3`](https://github.com/siderolabs/crypto/commit/1c94bb3967a427ba52c779a1b705f5aea466dc57) chore: bump dependencies
</p>
</details>

### Changes from siderolabs/gen
<details><summary>1 commit</summary>
<p>

* [`238baf9`](https://github.com/siderolabs/gen/commit/238baf95e228d40f9f5b765b346688c704052715) chore: add typesafe `SyncMap` and bump stuff
</p>
</details>

### Changes from siderolabs/go-api-signature
<details><summary>1 commit</summary>
<p>

* [`cf2bd06`](https://github.com/siderolabs/go-api-signature/commit/cf2bd06af87c946d6cdd61e127528f89e6f50591) chore: bump dependencies
</p>
</details>

### Changes from siderolabs/go-loadbalancer
<details><summary>1 commit</summary>
<p>

* [`aab4671`](https://github.com/siderolabs/go-loadbalancer/commit/aab4671fae0d14662a8d7167829c8c6725d28b38) chore: rekres, update dependencies
</p>
</details>

### Changes from siderolabs/go-talos-support
<details><summary>2 commits</summary>
<p>

* [`20a1135`](https://github.com/siderolabs/go-talos-support/commit/20a11358e84e055e6f47d468e66e57f561c90249) feat: add modules for getting Talos support bundle (#1)
* [`afa24c4`](https://github.com/siderolabs/go-talos-support/commit/afa24c4452a1cdb6f6836f9c8529645a2ccb9014) feat: initial commit
</p>
</details>

### Dependency Changes

* **github.com/emicklei/dot**                    v1.6.0 -> v1.6.1
* **github.com/siderolabs/crypto**               v0.4.1 -> v0.4.2
* **github.com/siderolabs/gen**                  v0.4.7 -> v0.4.8
* **github.com/siderolabs/go-api-signature**     v0.3.1 -> v0.3.2
* **github.com/siderolabs/go-loadbalancer**      v0.3.2 -> v0.3.3
* **github.com/siderolabs/go-talos-support**     v0.1.0 **_new_**
* **github.com/siderolabs/talos/pkg/machinery**  v1.6.4 -> v1.7.0-alpha.1
* **github.com/stretchr/testify**                v1.8.4 -> v1.9.0
* **google.golang.org/grpc**                     v1.62.0 -> v1.62.1
* **k8s.io/api**                                 v0.29.1 -> v0.29.2
* **k8s.io/client-go**                           v0.29.1 -> v0.29.2

Previous release can be found at [v0.31.0](https://github.com/siderolabs/omni/releases/tag/v0.31.0)

## [Omni 0.32.0-beta.0](https://github.com/siderolabs/omni/releases/tag/v0.32.0-beta.0) (2024-03-26)

Welcome to the v0.32.0-beta.0 release of Omni!  
*This is a pre-release of Omni*



Please try out the release binaries and report any issues at
https://github.com/siderolabs/omni/issues.

### `omnictl support`

`omnictl` CLI tool now has support bundle collection utility.
It collects all cluster related resources from Omni and includes all data which can be collected by `talosctl support` command.


### Omni Workload Proxy

Kubernetes services exposed through Omni now open on a new page, instead using the iframe.
Iframe often doesn't work due to headers restrictions.


### Contributors

* Artem Chernyshev
* Utku Ozdemir
* Andrey Smirnov
* Dmitriy Matrenichev
* Noel Georgi

### Changes
<details><summary>11 commits</summary>
<p>

* [`78bb689`](https://github.com/siderolabs/omni/commit/78bb6899ed377ffd83b748371c84a937f02dc83f) release(v0.32.0-beta.0): prepare release
* [`176f9d9`](https://github.com/siderolabs/omni/commit/176f9d9f57530832a9ebbb64d008bc98300b2cc7) feat: compute schematic id only from the extensions
* [`1e4e303`](https://github.com/siderolabs/omni/commit/1e4e303c098fa18bd63913d7a1d09538ed637cd0) feat: implement `omnictl support` command
* [`a835cc7`](https://github.com/siderolabs/omni/commit/a835cc730c9bedd7f16a9a8b5c2c38464b71e189) fix: fix error handling in image pre pull task
* [`2d1b776`](https://github.com/siderolabs/omni/commit/2d1b776f6c61db029a72e62deef81eafe34c29e9) fix: properly handle upgrades for the machines with invalid schematics
* [`4db7630`](https://github.com/siderolabs/omni/commit/4db76307924bf86bab458b029923ddd98a22ad0e) feat: add the context menu for copying the machine id
* [`5a8abf5`](https://github.com/siderolabs/omni/commit/5a8abf584edd605ac7bfc6f72b31142c857128c4) fix: get rid of the issue with `MachineSets` stuck in `Reconfiguring`
* [`8173377`](https://github.com/siderolabs/omni/commit/8173377c122e5ff0c6e710362f8481ce01ca929d) feat: preserve maintenance machine configs
* [`190218a`](https://github.com/siderolabs/omni/commit/190218ad2fcf07f94d0d55178ad08a21b2bc3bfa) feat: open exposed services in a new window
* [`0960100`](https://github.com/siderolabs/omni/commit/0960100f11b4229d67c84de5d136d6f56d5a05b0) chore: drop integration binary from releases
* [`6e3ba5c`](https://github.com/siderolabs/omni/commit/6e3ba5c389622987028af4e5ee50ac638f70c32e) chore: bump Go, build arm64 container images, rekres
</p>
</details>

### Changes from siderolabs/crypto
<details><summary>1 commit</summary>
<p>

* [`1c94bb3`](https://github.com/siderolabs/crypto/commit/1c94bb3967a427ba52c779a1b705f5aea466dc57) chore: bump dependencies
</p>
</details>

### Changes from siderolabs/gen
<details><summary>1 commit</summary>
<p>

* [`238baf9`](https://github.com/siderolabs/gen/commit/238baf95e228d40f9f5b765b346688c704052715) chore: add typesafe `SyncMap` and bump stuff
</p>
</details>

### Changes from siderolabs/go-api-signature
<details><summary>1 commit</summary>
<p>

* [`cf2bd06`](https://github.com/siderolabs/go-api-signature/commit/cf2bd06af87c946d6cdd61e127528f89e6f50591) chore: bump dependencies
</p>
</details>

### Changes from siderolabs/go-loadbalancer
<details><summary>1 commit</summary>
<p>

* [`aab4671`](https://github.com/siderolabs/go-loadbalancer/commit/aab4671fae0d14662a8d7167829c8c6725d28b38) chore: rekres, update dependencies
</p>
</details>

### Changes from siderolabs/go-talos-support
<details><summary>2 commits</summary>
<p>

* [`20a1135`](https://github.com/siderolabs/go-talos-support/commit/20a11358e84e055e6f47d468e66e57f561c90249) feat: add modules for getting Talos support bundle (#1)
* [`afa24c4`](https://github.com/siderolabs/go-talos-support/commit/afa24c4452a1cdb6f6836f9c8529645a2ccb9014) feat: initial commit
</p>
</details>

### Dependency Changes

* **github.com/emicklei/dot**                    v1.6.0 -> v1.6.1
* **github.com/siderolabs/crypto**               v0.4.1 -> v0.4.2
* **github.com/siderolabs/gen**                  v0.4.7 -> v0.4.8
* **github.com/siderolabs/go-api-signature**     v0.3.1 -> v0.3.2
* **github.com/siderolabs/go-loadbalancer**      v0.3.2 -> v0.3.3
* **github.com/siderolabs/go-talos-support**     v0.1.0 **_new_**
* **github.com/siderolabs/talos/pkg/machinery**  v1.6.4 -> v1.7.0-alpha.1
* **github.com/stretchr/testify**                v1.8.4 -> v1.9.0
* **google.golang.org/grpc**                     v1.62.0 -> v1.62.1
* **k8s.io/api**                                 v0.29.1 -> v0.29.2
* **k8s.io/client-go**                           v0.29.1 -> v0.29.2

Previous release can be found at [v0.31.0](https://github.com/siderolabs/omni/releases/tag/v0.31.0)

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

