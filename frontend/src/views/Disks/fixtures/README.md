# Captured machines

The disks view's Talos resources, captured from real machines. The stories in
`MachineDisks.stories.ts` and the tests in `lib/diskTopology.fixtures.spec.ts`
use them, so the view is checked against what Talos reports, not only against
hand-written mocks.

| Fixture                          | Machine                                                                                                                                      |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `talos-1.14-maintenance.json`    | Talos 1.14.5 QEMU VM in maintenance mode, with four virtio disks.                                                                            |
| `talos-1.14-lvm-raid1.json`      | Talos 1.14.5: LVM RAID1 `data/data` over `vdc` and `vdd`.                                                                                    |
| `talos-1.14-md-raid1.json`       | Talos 1.14.5: MD RAID1 `md0` over `vdc` and `vdd`, with a user volume on it.                                                                 |
| `talos-1.14-luks-lvm.json`       | Talos 1.14.5: on `vda`, an encrypted user volume, and an encrypted raw volume holding the PV of `vg0/data`.                                  |
| `talos-1.15-maintenance.json`    | Talos 1.15.0-beta.0 QEMU VM in maintenance mode.                                                                                             |
| `talos-1.15-md-raid1.json`       | Talos 1.15.0-beta.0: as `talos-1.14-md-raid1.json`.                                                                                          |
| `talos-1.15-luks-lvm-raid1.json` | Talos 1.15.0-beta.0: as `talos-1.14-luks-lvm.json` (the logical volume named `vg0/secure`), plus LVM RAID1 `data/data` over `vdc` and `vdd`. |

In all of them, `vdb` is the system disk. Only the Talos 1.15 fixtures include
the `SystemDisk` resource; it wasn't captured from the Talos 1.14 machines.

## Capturing a machine

Read each resource type from the machine as JSON, through Omni or directly:

```sh
for t in disks blockdevices discoveredvolumes volumestatuses lvmlogicalvolumestatuses systemdisks; do
  talosctl -n <machine> get $t -o json > $t.json
done
```

Then combine them into one fixture, keeping only each resource's ID, namespace,
type and spec:

```sh
r() { [ -s $1.json ] && jq -s '[.[] | {metadata: {id: .metadata.id, namespace: .metadata.namespace, type: .metadata.type}, spec}] | sort_by(.metadata.id)' $1.json || echo '[]'; }

jq -n --arg v <talos-version> \
  --argjson disks "$(r disks)" \
  --argjson blockDevices "$(r blockdevices)" \
  --argjson volumes "$(r discoveredvolumes)" \
  --argjson volumeStatuses "$(r volumestatuses)" \
  --argjson logicalVolumes "$(r lvmlogicalvolumestatuses)" \
  --argjson systemDisks "$(r systemdisks)" \
  '{talosVersion: $v, $disks, $blockDevices, $volumes, $volumeStatuses, $logicalVolumes, $systemDisks}' \
  > talos-<version>-<setup>.json
```

Add it to `index.ts`, with a story and a test.

The storage config used for `talos-1.14-lvm-raid1.json`. The others use the
same documents, and `RAIDArrayConfig`, `UserVolumeConfig` and `RawVolumeConfig`
with `disk.dev_path` selectors:

```yaml
apiVersion: v1alpha1
kind: LVMVolumeGroupConfig
name: data
provisioning:
  volumeSelector:
    match: disk.dev_path == "/dev/vdc" || disk.dev_path == "/dev/vdd"
---
apiVersion: v1alpha1
kind: LVMLogicalVolumeConfig
name: data
type: raid1
provisioning:
  volumeGroup: data
  maxSize: 80%
```
