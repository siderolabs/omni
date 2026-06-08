# Omni MCP server: design notes and verification findings

Status: design exploration. Captures the conclusions of the "MCP server vs. API vs.
declarative orchestration" discussion, with each open item verified against the Omni
source tree and the current Model Context Protocol (MCP) authorization spec.

## TL;DR

- **MCP server and the agent skill are complementary, not alternatives.** The MCP server is
  the *capability + authorization* layer; the skill is the *workflow + judgment* layer. A
  skill can reference MCP tools, so they compose.
- **An MCP server is, in implementation, a thin typed adapter over the existing API** — but
  its reason to exist is that the consumer is a model at runtime, not a developer writing
  glue ahead of time. It delivers a curated, typed, runtime-discoverable tool surface that
  every agent client already speaks.
- **The robust agent interface is "declare what you want," not "run these N steps."** Omni
  is already a declarative reconciliation system (COSI resource/controller runtime). The
  MCP server should be a typed front door onto that intent layer, not a new tier of logic.
- **The premise that mutating ops must wait for "RBAC v2 + audit" overstates the gap:**
  role-based access control, per-cluster scoping (`AccessPolicy`), effective-permission
  resources, and a comprehensive audit log already exist. The real missing piece is an
  **OAuth 2.1 resource-server adapter** in front of Omni's PGP-signature identity model.

---

## Open item 1 — Current remote MCP authorization spec

Verified against the MCP specification (versions `2025-06-18` and `2025-11-25`). The spec
has stabilized since early 2025; the moving target the discussion flagged has largely
settled.

| Property | Status in current spec |
|---|---|
| MCP server role | **OAuth 2.1 Resource Server** |
| Protected Resource Metadata (RFC 9728) | Server **MUST** publish; client **MUST** use it for authorization-server discovery |
| Resource Indicators (RFC 8707) | Client **MUST** send; binds the access token's audience to a single resource server (prevents token redirection by a malicious server) |
| PKCE | Mandatory |
| Client registration | Priority: pre-registration → **Client ID Metadata Documents (CIMD)** → Dynamic Client Registration → manual. CIMD (client identifies via a URL it controls) is the new default added in `2025-11-25` |
| Enterprise-managed auth | `2025-11-25` adds Cross-App-Access so enterprises broker MCP connections through their IdP without OAuth redirects |

**Mapping onto Omni.** Omni does *not* use OAuth bearer tokens for non-human callers
today. Service accounts and CLI users authenticate with **PGP-signed requests**
(`github.com/siderolabs/go-api-signature`); humans use SAML / OIDC / Auth0
(`client/api/omni/specs/auth.proto`, `internal/pkg/auth`). A hosted Omni MCP server
therefore needs an OAuth 2.1 resource-server front end that:

1. Publishes RFC 9728 metadata pointing at Omni's authorization server (or a federated
   IdP).
2. Accepts audience-bound tokens (RFC 8707) — exactly the property you want before exposing
   cluster-mutating tools.
3. Maps the validated token to an Omni identity + role, reusing the existing authorization
   and audit path rather than inventing a parallel one.

The audience-binding requirement is a feature, not a cost: it is the mechanism that keeps a
token minted for the Omni MCP server from being replayed against another resource server.

---

## Open item 2 — omnictl audit: declarative vs. client-side orchestration

Audited every omnictl subcommand against source. The architecture is overwhelmingly
declarative — most commands are a single CRUD call on a COSI resource, and the heavy
lifting lives in server-side controllers. A minority do multi-step client-side work.

A crucial distinction the raw command list obscures: **a single declarative write followed
by an optional read-only `--wait`/watch is not brittle.** Losing the watch connection is
harmless — the intent is already persisted and a controller drives convergence. Only
commands that issue **multiple client-side writes** (or a client-driven multi-phase
delete) can strand state mid-sequence.

### Genuinely brittle (multi-write client-side orchestration) — MCP-tool / server-side-job candidates

| Command | File | Why brittle |
|---|---|---|
| `cluster template sync` | `client/pkg/template/operations/sync.go:35` | Computes a create/update/destroy plan client-side, then issues a sequence of `Create`/`Update` calls and a **client-driven teardown→destroy loop** (`sync.go:169-198`): it watches for finalizers to clear and issues `Destroy` itself. Drop the connection between teardown and destroy and resources sit in `PhaseTearingDown` with nothing client-side to finish them. This is Omni's `kubectl drain`. |
| `cluster template delete` | `client/pkg/template/operations/delete.go:24` | Same client-driven multi-resource teardown→destroy as sync, no transactional boundary. |
| `cluster delete` | `client/pkg/omnictl/cluster/delete.go:23` | `operations.DeleteCluster` performs cascading client-side resource cleanup. |
| `cluster machine delete` | `client/pkg/omnictl/cluster/machine.go:55` | Creates a `ForceDestroyRequest`, tears down the `Link`, destroys the `MachineSetNode`, then watches `ClusterMachine` deletion — several ordered writes. |
| `cluster import` / `import abort` | `client/pkg/omnictl/cluster/import.go:106` / `:28` | Highest-complexity flow: builds Talos + image-factory clients and runs multi-step client-side reconciliation; abort partially undoes it. |
| `cluster kubernetes manifest-sync` | `client/pkg/omnictl/cluster/kubernetes/manifest-sync.go:23` | Streams manifests and applies each in a client loop; mid-stream disconnect leaves a partial apply. |
| `user delete` (legacy path) | `client/pkg/omnictl/user/delete.go:22` | Loops over emails, tears down + destroys both `Identity` and `User` per user; partial completion possible. |

### Not brittle — single declarative write + optional read-only wait

These were easy to misclassify as "orchestration" because they have timeouts/watches, but
the mutation is one atomic write and the wait is benign read polling:

- `cluster secret rotate talos-ca` / `kubernetes-ca` / `status`
  (`client/pkg/omnictl/cluster/secret/rotate.go`) — sets one annotation; `--wait` is a
  read-only watch for `Phase=OK`.
- `cluster status`, `cluster template status` — pure read/watch.
- `cluster lock`/`unlock`, `cluster machine lock`/`unlock`, all `jointoken` mutations,
  `user set-role`, `configure machine`, `apply`, `delete`, `edit`, `get` — single
  `Create`/`Update`/`Destroy`/`List`.

### Low blast-radius multi-step (local key gen + one API call)

`serviceaccount create`/`renew`, `infraprovider create`/`renewkey`, `media download`,
`user create` (legacy two-resource path). Failure means "retry," not "stranded cluster."

### Implication for the MCP surface

The brittle commands are precisely the ones that should **not** be exposed to an agent as a
sequence of raw calls. Each is a candidate to either (a) push the orchestration server-side
behind a single reconciled operation/resource, or (b) expose as one composed MCP tool whose
handler owns the sequence with idempotency/resumability. `cluster template sync`'s
teardown→destroy loop is the strongest argument for moving that orchestration into a
controller — the client is the wrong place for it, the same way the Kubernetes project moved
three-way merge into server-side apply.

---

## Open item 3 — MCP server scope decision

RBAC and audit were verified to already exist and be reasonably mature (no in-progress
"v2" migration is present in source; the only versioning artifact is the **deprecated**
`scopes` proto field being phased out *in favor of* roles).

What exists today:

- **Roles** (`internal/pkg/auth/role/role.go`): `None < InfraProvider < Reader < Operator < Admin`, hierarchical with `Check()`.
- **Per-cluster scoping** via `AccessPolicy` (`internal/pkg/auth/accesspolicy`): user-groups × cluster-groups → role override (+ K8s impersonation). This is the primitive for confining an agent service account to specific clusters.
- **Effective-permission virtual resources** (`internal/backend/runtime/omni/virtual/state.go`): `Permissions` (global) and `ClusterPermissions` (per-cluster) already enumerate capability booleans (`CanUpdateTalos`, `CanRebootMachines`, `CanRemoveMachines`, `CanDownloadSupportBundle`, …). These map almost 1:1 onto an MCP tool-gating layer.
- **Audit** (`internal/backend/runtime/omni/audit`): captures actor (id / role / email / PGP fingerprint), event type (create/update/teardown/destroy/talos_access/k8s_access), resource, and mutation detail to a queryable, retention-bounded SQLite store; exposed via `ManagementService.ReadAuditLog`. Talos and Kubernetes API access are audited too.

### API surface the MCP server would front

Omni's API is two layers, which reinforces the "declarative front door" framing:

- **COSI state API** — declarative CRUD on resources (clusters, machine sets, config
  patches). This is where desired state is written; controllers reconcile.
- **`ManagementService`** (`client/api/omni/management/management.proto:349`) — the
  imperative helpers that don't fit CRUD: `Kubeconfig`, `Talosconfig`,
  `CreateServiceAccount`, `CreateSchematic`, `KubernetesSyncManifests`,
  `KubernetesUpgradePreChecks`, `MaintenanceUpgrade`, `GetSupportBundle`, `ReadAuditLog`,
  `MachinePowerOff/On`, etc.

### Recommended scope (phased)

1. **Phase 1 — read-mostly, declarative.** Expose `get`/`list`/`watch` over the COSI
   resources plus read-only Management RPCs (`Kubeconfig`/`Talosconfig` download, status,
   pre-checks, `ReadAuditLog`). Gate every tool on the existing `Permissions` /
   `ClusterPermissions` booleans for the caller. Auth via the OAuth 2.1 resource-server
   adapter from item 1, mapping tokens to existing roles.
2. **Phase 2 — declarative mutations.** Allow writing desired-state resources (set Talos /
   Kubernetes version, scale machine sets, apply config patches) — the failure-safe
   operations, since the controller owns convergence after the write. Confine agent
   service accounts via `AccessPolicy`.
3. **Phase 3 — composed lifecycle tools.** Only after the brittle client-side flows from
   item 2 are moved server-side (or wrapped in idempotent, resumable handlers) should they
   be exposed as single MCP tools (e.g. "upgrade this cluster safely", "delete this
   cluster"). Do not expose client-side multi-write choreography to an agent.

Audit requires no new work to cover agent mutations — they flow through the same hooks. The
one true prerequisite for mutating tools is the OAuth resource-server front end and binding
agent identities to scoped `AccessPolicy` rules.

---

## Sources

- MCP authorization spec (`2025-11-25` and `2025-06-18`): https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization
- June 2025 auth changes (RFC 9728 / RFC 8707, resource-server model): https://auth0.com/blog/mcp-specs-update-all-about-auth/
- November 2025 changes (CIMD, enterprise-managed auth): https://aaronparecki.com/2025/11/25/1/mcp-authorization-spec-update , https://auth0.com/blog/mcp-november-2025-specification-update/
