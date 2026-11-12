# Captured machines

The disks view's Talos resources, captured from real machines. The stories in
`MachineDisks.stories.ts` and the tests in `lib/diskTopology.fixtures.spec.ts`
use them, so the view is checked against what Talos reports, not only against
hand-written mocks.

| Fixture                               | Machine                                                                                                                                                                                                                                                                                                                                              |
| ------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `talos-1.14-lvm-raid1.json`           | Talos 1.14.5: LVM RAID1 `data/data` over `vdc` and `vdd`. `vdb` is the system disk.                                                                                                                                                                                                                                                                  |
| `talos-1.14-luks-lvm.json`            | Talos 1.14.5: on `vda`, an encrypted user volume, and an encrypted raw volume holding the PV of `vg0/data`. `vdb` is the system disk.                                                                                                                                                                                                                |
| `talos-1.14-md-raid1-failed.json`     | Talos 1.14.5: an MD RAID1 array Talos failed to create over `vdc` and `vdd`, because an LVM volume group still held them.                                                                                                                                                                                                                            |
| `talos-1.15-maintenance.json`         | Talos 1.15.0-beta.0 in maintenance mode, with four virtio disks.                                                                                                                                                                                                                                                                                     |
| `talos-1.15-luks-lvm-raid1.json`      | Talos 1.15.0-beta.0 with emulated NVMe and SATA disks: on `vda`, an encrypted user volume and an encrypted raw volume holding the PV of `vg0/secure`; LVM RAID1 `data/data` over `sda` and `vdb`; and a user volume on each logical volume. `nvme0n1` is the system disk.                                                                            |
| `talos-1.15-md-raid1-smart.json`      | Talos 1.15.0-beta.0 with emulated NVMe and SATA disks monitored by SMART, the NVMe failing (critical warning set through the QEMU monitor): MD RAID1 `md127` over `sda` and `vdb`, holding a user volume; an array `spare` waiting for a second member; and a user volume `u-nowhere` that matches no disk, so failed. `nvme0n1` is the system disk. |
| `talos-1.15-md-raid1-rebuilding.json` | The same machine rebuilding `md127`, after the start of `vdb` was zeroed and the VM restarted.                                                                                                                                                                                                                                                       |
| `talos-1.15-lvm-raid1-degraded.json`  | Talos 1.15.0-beta.0: LVM RAID1 `data/data` over `sda` and `vdb`, its volume group missing `vdb` (zeroed, then the VM restarted), so its logical volume is inactive and has no device.                                                                                                                                                                |

The failures were made on the running VMs through their QEMU monitor
(`~/.talos/clusters/<cluster>/<node>.monitor`): `qom-set /machine/peripheral/nvme-ctrl-0
smart_critical_warning 4` for a failing NVMe disk, and zeroing the first 16 MiB of a
member's disk file, then `q` to restart the VM, for a lost RAID member.

Each fixture covers a setup once, or once per Talos version where the two
take different paths (LVM and LUKS detection). The Talos 1.14 fixtures were
captured without the `SystemDisk` resource.

## Space usage

`mountStatuses` and `statfs` give how full each mounted volume is, as the page
gets it on Talos 1.15+: the `MountStatuses.block.talos.dev` resources say where
each Talos volume is mounted, and `statfs` holds Talos's `Statfs` result for
each mount target, keyed by the target. Only `talos-1.15-luks-lvm-raid1.json`
has them. Capture them with:

```sh
talosctl -n <machine> get MountStatuses.block.talos.dev -o json > mountstatuses.json
talosctl -n <machine> df -b   # talosctl 1.15+: SIZE, USED, AVAILABLE and MOUNT POINT
```

## Capturing a machine

Read each resource type from the machine as JSON, through Omni or directly:

```sh
for t in disks blockdevices discoveredvolumes volumestatuses lvmlogicalvolumestatuses \
  systemdisks smartstatuses mdarraystatuses lvmvolumegroupstatuses; do
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
  --argjson smartStatuses "$(r smartstatuses)" \
  --argjson mdArrays "$(r mdarraystatuses)" \
  --argjson volumeGroups "$(r lvmvolumegroupstatuses)" \
  '{talosVersion: $v, $disks, $blockDevices, $volumes, $volumeStatuses, $logicalVolumes, $systemDisks, $smartStatuses, $mdArrays, $volumeGroups}' \
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
