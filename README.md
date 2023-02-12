# grid3_client_go

[![Codacy Badge](https://app.codacy.com/project/badge/Grade/cd6e18aac6be404ab89ec160b4b36671)](https://www.codacy.com/gh/threefoldtech/grid3-go/dashboard?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=threefoldtech/grid3-go&amp;utm_campaign=Badge_Grade)

grid3_go is a go client created to interact with threefold grid. It should manage CRUD operations for deployments on the grid.

## Requirements

- [Go](https://golang.org/doc/install) >= 1.19

## Examples

Refer to [integration examples](https://github.com/threefoldtech/grid3-go/tree/development/integration_tests) directory for examples.

## Run tests

To run the tests, export MNEMONICS and NETWORK

```bash
export MNEMONICS="<mnemonics words>"
export NETWORK="<network>" # dev, qa or test
```

Run the following command

### running unit tests

```bash
make test
```

### running integration tests

```bash
make integration
```
