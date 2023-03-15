# tf-grid commands

## virtual machine

```json
{
    "action": "deploy_vm",
    "params": {
        "name": "<name of the virtual machine>",
        "ssh_key": "<path to public ssh key>",
        "cpu":  "<number of cpu cores, default is 1>",
        "memory":  "<memory size in gb, default is 1>",
        "rootfs":  "<root filesystem size in gb, default is 0>",  
        "disk":  "<disk size in gb mounted on /disk, default is 10>",
        "flist":  "<flist for vm, default is >",
        "entrypoint":  "<entrypoint>",
        "ipv4":  "<if public ipv4 for vm, default is false>",
        "ipv6":  "<if public ipv6 for vm, default is false>",
        "ygg":  "<if yggdrasil ip for vm, default is true>"
    }
}
```

```json
{
    "action": "get_vm",
    "params": {
        "name": "<name of the virtual machine>"
    }  
}
```

## kubernetes

```json
{
    "action": "deploy_k8s",
    "params": {
        "name": "<name of the kubernetes cluster>",
        "ssh_key": "<path to public ssh key>",
        "master_cpu": "<number of cpu cores, default is 1>",
        "master_memory":  "<memory size in gb, default is 1>",
        "master_disk":  "<master disk size in gb, default is 2>",
        "workers_number":  "<workers number you need>",
        "worker_cpu":  "<number of cpu cores, default is 1>",
        "worker_memory":  "<memory size in gb, default is 1>",
        "worker_disk":  "<workers disk size in gb, default is 2>",
        "ipv4":  "<if public ipv4 for vm, default is false>",
        "ipv6":  "<if public ipv6 for vm, default is false>",
        "ygg":  "<if yggdrasil ip for vm, default is true>"
    }  
}
```

```json
{
    "action": "get_k8s",
    "params": {
        "name": "<name of the kubernetes cluster>"
    }  
}
```

## Gateway FQDN

```json
{
    "action": "deploy_fqdn",
    "params": {
        "name": "<name of the gateway fqdn>",
        "node": "<the node ID you want to use>",
        "backends": "backends for the gateway, default is []>",
        "tls":  "if you want to add tls passthrough, default is false>",
        "fqdn":  "<domain defined in the name service pointing to the ip of the gateway node>"
    }  
}
```

```json
{
    "action": "get_fqdn",
    "params": {
        "name": "<name of the gateway fqdn>"
    }  
}
```

## Gateway Name

```json
{
    "action": "deploy_name",
    "params": {
        "name": "<name of the gateway name>",
        "node": "<the node ID you want to use>",
        "backends": "backends for the gateway, default is []>",
        "tls":  "if you want to add tls passthrough, default is false>"
    }  
}
```

```json
{
    "action": "get_name",
    "params": {
        "name": "<name of the gateway name>"
    }  
}
```

## Cancel a deployment

```json
{
    "action": "cancel",
    "params": {
        "name": "<name of the deployment>"
    }  
}
```
