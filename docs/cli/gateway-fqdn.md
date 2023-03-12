# Gateway FQDN

This document explains Gateway FQDN related commands using tf-grid cli.

## Deploy

```bash
tf-grid deploy gateway-fqdn [flags]
```

### Required Flags

- name: name for the gateway deployment also used for canceling the deployment. must be unique.
- node: node id to deploy gateway on.
- backends: list of backends the gateway will forward requests to.
- fqdn: FQDN pointing to the specified node.

### Optional Flags

-tls: add TLS passthrough option (default false).

Example:

```bash
./tf-grid deploy gateway-name -n gatewaytest --node 14 --backends http://93.184.216.34:80 --fqdn example.com
```

You should see an output like this:

```bash
3:34PM INF deploying gateway fqdn
3:34PM INF gateway fqdn deployed
```

## Get

```bash
tf-grid get gateway-fqdn <gateway>
```

gateway is the name used when deploying gateway-fqdn using tf-grid.

Example:

```bash
tf-grid get gateway gatewaytest
```

You should see an output like this:

```bash
3:35PM INF fqdn: example.com
```

## Cancel

```bash
tf-grid cancel <deployment-name>
```

deployment-name is the name of the deployment specified in while deploying using tf-grid.

Example:

```bash
tf-grid cancel gatewaytest
```

You should see an output like this:

```bash
3:37PM INF canceling contracts for project gatewaytest
3:37PM INF gatewaytest canceled
```
