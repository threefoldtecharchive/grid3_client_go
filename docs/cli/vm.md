# Virtual Machine

This document explains Virtual Machine related commands using tf-grid cli.

## Deploy

```bash
tf-grid deploy vm [flags]
```

### Required Flags

- name: name for the VM deployment also used for canceling the deployment. must be unique.
- ssh: path to public ssh key to set in the VM.

### Optional Flags

- cpu: number of cpu units (default 1).
- disk: size of disk in GB mounted on /data. if not set no disk workload is made.
- entrypoint: entrypoint for VM flist (default "/sbin/zinit init"). note: setting this without the flist option will fail.
- flist: flist used in VM (default "https://hub.grid.tf/tf-official-apps/threefoldtech-ubuntu-22.04.flist"). note: setting this without the entrypoint option will fail.
- ipv4: assign public ipv4 for VM (default false).
- ipv6: assign public ipv6 for VM (default false).
- memory: memory size in GB (default 1).
- rootfs: root filesystem size in GB (default 2).
- ygg: assign yggdrasil ip for VM (default true).

Example:

```bash
tf-grid deploy vm --name examplevm --ssh ~/.ssh/id_rsa.pub --cpu 2 --memory 4 --disk 10
```

You should see an output like this:

```bash
12:06PM INF deploying network
12:06PM INF deploying vm
12:07PM INF vm yggdrasil ip: 300:e9c4:9048:57cf:7da2:ac99:99db:8821
```

## Get

```bash
tf-grid get vm <vm>
```

vm is the name used when deploying vm using tf-grid.

Example:

```bash
tf-grid get vm examplevm
```

You should see an output like this:

```bash
12:08PM INF vm yggdrasil ip: 300:e9c4:9048:57cf:7da2:ac99:99db:8821
```
