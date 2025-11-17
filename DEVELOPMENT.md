# Developing Omni

This guide assumes a MacOS or a Linux system.
Using [OrbStack](https://orbstack.dev/) is recommended on Macos.

Make sure you have a running [Talos development environment](https://www.talos.dev/latest/advanced/developing-talos/) before proceeding with this guide.

Decide on the CIDR for the local set of QEMU Talos machines.
In this document, we are going to use the `172.20.0.0/24` CIDR, but you can use any CIDR you want.

With the CIDR `172.20.0.0/24`, the bridge IP is going to be `172.20.0.1`, so we are going to use that IP
as the Omni endpoint QEMU VMs can reach.

## Build Omni and omnictl

For example for `linux` on `amd64`.

```shell
make omni-linux-amd64
```

```shell
make omnictl-linux-amd64
```

## Dev Environment

### mkcert

First, install and run [mkcert](https://github.com/FiloSottile/mkcert) to generate the TLS root certificates,
and add them to your system's trust store.
After installing dependencies, you can run the following command (requires `sudo`) to generate the root certificates:

```shell
make mkcert-install
```

or

```shell
go run ./hack/generate-certs install
```

Then you will need to create `generate-certs.yml` which you customize using `generate-certs.example.yml` as an example.
The example file also contains a commented-out registry mirror configuration which speeds up local testing.
After that, you can run the following command to generate the certificates and `hack/compose/docker-compose.override.yml` file:

```shell
make mkcert-generate
```

or

```shell
go run ./hack/generate-certs -config ./hack/generate-certs.yml generate
```

This should result in the following (git-ignored) files:

- `hack/generate-certs/ca-root/rootCA.pem`
- `hack/generate-certs/ca-root/rootCA-key.pem`
- `hack/generate-certs/certs/localhost.pem`
- `hack/generate-certs/certs/localhost-key.pem`
- `hack/compose/docker-compose.override.yml`

Depending on your environment you'll need to rewrite or add some options to the `hack/compose/docker-compose.override.yml` file.

- If you are on MacOs the following options have to be set due to networking limitations on the platform:
  - Set `--machine-api-bind-addr` to `0.0.0.0:8090`
  - Set `--siderolink-wireguard-bind-addr` to `0.0.0.0:50180`
  - Set `--siderolink-wireguard-advertised-addr` to `172.20.0.1:50180`
  - Set `--event-sink-port` to `8091`
  - Set `--machine-api-advertised-url` to `grpc://172.20.0.1:8090`

After that, you can run the following command to start the docker-compose environment:

```shell
make docker-compose-up WITH_DEBUG=1
```

When you're done, you can run the following command to stop the docker-compose environment:

```shell
make docker-compose-down
```

If you need to clean up `etcd` state, run:

```shell
docker volume rm compose_etcd
```

If you want to remove all volumes created by docker compose (e.g. `etcd`, `logs`, `secondary-storage`), run:

```shell
make docker-compose-down REMOVE_VOLUMES=true
```

### mkcert uninstall

You can always remove the root certificates from your system's trust store using the following command:

```shell
make mkcert-uninstall
```

or

```shell
go run ./hack/generate-certs uninstall
```

## Start Talos VMs

```shell
sudo -E _out/talosctl-linux-amd64 cluster create \
    --provisioner=qemu --cidr=172.20.0.0/24 --install-image=ghcr.io/siderolabs/installer:v1.3.2 --memory 2048 --memory-workers 2048 --disk 6144 --cpus 2 --controlplanes 1 --workers 3 \
    --extra-boot-kernel-args ''siderolink.api=grpc://<OMNI_IP>:8090?jointoken=w7uVuW3zbVKIYQuzEcyetAHeYMeo5q2L9RvkAVfCfSCD talos.events.sink=[fdae:41e4:649b:9303::1]:8091 talos.logging.kernel=tcp://[fdae:41e4:649b:9303::1]:8092''
    --skip-injecting-config --wait=false --with-init-node
```

> Note: `<OMNI_IP>` is the IP address on which the Omni instance can be accessed by the machines.
> It can also be the bridge IP if the Omni is running on the host network or it can be the host machine's local IP.
> Omni also prints these args in the startup logs.

## Open Omni UI

By default, Omni serves the frontend and API on `*:443`, so you can open the Omni UI in your browser with e.g. `https://my.host/`.

You should see your Talos VMs registered in the `Machines` tab, and a cluster can be created in the `Clusters` tab.

Node.js development server can be used to get immediate feedback on frontend changes: `https://my.host:8120/`.
When making frontend changes, `https://my.host/` will only update after stopping docker-compose environment with `^C` and running `make docker-compose-up WITH_DEBUG=1` again.
At the same time `https://my.host:8120/` will update immediately.

## Use `omnictl`

Download `omniconfig` from the Omni UI.

Fetch some resources with `omnictl`:

```shell
$ _out/omnictl-linux-amd64 --omniconfig=omniconfig get machines
NAMESPACE   TYPE      ID                                     VERSION
default     Machine   17e1d2c1-60f0-452e-87a9-bc949953643b   1
default     Machine   20141377-15f2-43e2-a0a9-ff68ca21d90e   1
```

If the browser can't be launched from your machine (e.g., in a headless environment), you can use environment variable `BROWSER=echo` to see the URL instead.

## Running Integration Tests

Make sure the Omni database is clean, and it has some machines connected to it.

Then run the integration tests:

```shell
$ sudo -E make run-integration-test WITH_DEBUG=true
=== RUN   TestSideroLinkDiscovery
    siderolink.go:54: links discovered: 1
    siderolink.go:54: links discovered: 2
    siderolink.go:54: links discovered: 3
    siderolink.go:54: links discovered: 4
--- PASS: TestSideroLinkDiscovery (0.00s)
PASS
```

Another way to run integration tests directly:

```shell
$ make _out/integration-test-linux-amd64
$ sudo -E _out/integration-test-linux-amd64 \
    --endpoint=https://my.host \
    --expected-machines=6
```

Tests need a hint on number of available Talos VMs with `--expected-machines` flag: make it equal to the sum of `--controlplanes` and `--workers` in the `talosctl cluster create` above.

Specific tests can be run by appending a flag `--test.run=TestSideroLinkDiscovery` to the command above.

## Local Network Use

When using Omni to provision Talos clusters in your LAN, it makes sense to launch it with default args, this way Omni advertises the first host IP address by default.

## Etcd Backups

You can set up Omni to periodically back up its etcd database to a local directory or s3 storage.
By default, Omni uses s3 storage.

To enable etcd backups into the local directory, set the following command line flag:

```shell
--etcd-backup-local-path /path/to/backup/dir
```

You can also set the `etcd-backup-tick-interval` and `etcd-backup-jitter` to control the frequency of backups checks, that is when the backup check will be performed and the jitter value will be added to the backup check time.
Keep in mind that jitter applied as delta to the backup time, so if you set jitter to 10s, backup will be performed in -5s to 5s from the backup time.

For s3 you should also create `EtcdBackupS3Configs.omni.sidero.dev` resource in the default namespace, since this is
the place where Omni gets s3 credentials and options from.
For example, for minio s3 with bucket `mybucket` and operating locally on port `9000` that would be:

```yaml
metadata:
  namespace: default
  type: EtcdBackupS3Configs.omni.sidero.dev
  id: etcd-backup-s3-conf
  version: undefined
  owner:
  phase: running
  created: 2023-12-12T17:43:12+00:00
  updated: 2023-12-12T17:43:12+00:00
spec:
  bucket: mybucket
  region: us-east-1
  endpoint: http://127.0.0.1:9000
  accesskeyid: access
  secretaccesskey: secret123
  sessiontoken: ""
```

Keep in mind that `etcd-backup-local-path` and `etcd-backup-s3` are mutually exclusive.

### Manual Etcd Backups

You can create manual etcd backup if s3 or local backup is enabled.
To do that, create a resource:

```yaml
metadata:
  namespace: ephemeral
  type: EtcdManualBackups.omni.sidero.dev
  id: <your-cluster-name>
  version: undefined
  owner:
  phase: running
spec:
  backupat:
    seconds: <unix-timestamp>
    nanos: 0
```

`unix-timestamp` should be no more than one minute in the future or in the past.

## Controller Dependency Graphs

If Omni is built `WITH_DEBUG=1`, it provides an additional handler under `/debug` prefix:

```shell
curl https://my.host/debug/controller-graph | dot -Tsvg -o controller.svg
curl https://my.host/debug/controller-resource-graph | dot -Tsvg -o controller-resource.svg
```
