# Kubernetes

This document explains Kubernetes related commands using tf-grid cli.

## Deploy

```bash
tf-grid deploy kubernetes [flags]
```

### Required Flags

- name: name for the master node deployment also used for canceling the cluster deployment. must be unique.
- ssh: path to public ssh key to set in the master node.
- master-node: node id to deploy master node on.

### Optional Flags

- ipv4: assign public ipv4 for master node (default false).
- ipv6: assign public ipv6 for master node (default false).
- ygg: assign yggdrasil ip for master node (default true).
- master-cpu: number of cpu units for master node (default 1).
- master-memory: master node memory size in GB (default 1).
- master-disk: master node disk size in GB (default 2).
- workers-number: number of workers nodes (default 0).
- workers-cpu: number of cpu units for each worker node (default 1).
- workers-memory: memory size for each worker node in GB (default 1).
- workers-disk: disk size in GB for each worker node (default 2).
- workers-node: node id to deploy all workers nodes on.

Example:

```bash
./tf-grid deploy kubernetes -n kube --ssh ~/.ssh/id_rsa.pub --master-node 14 --workers-number 2 --workers-node 14
```

You should see an output like this:

```bash
4:21PM INF deploying network
4:22PM INF deploying cluster
4:22PM INF master yggdrasil ip: 300:e9c4:9048:57cf:504f:c86c:9014:d02d
```

## Cancel

```bash
tf-grid cancel <deployment-name>
```

deployment-name is the name of the deployment specified in while deploying using tf-grid.

Example:

```bash
tf-grid cancel kube
```

You should see an output like this:

```bash
3:37PM INF canceling contracts for project kube
3:37PM INF kube canceled
```
