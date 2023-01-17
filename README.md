# grid3_client_go

grid3_go is a go client created to interact with threefold grid. It should manage CRUD operations for deployments on the grid.

## Requirements

- [Go](https://golang.org/doc/install) >= 1.18

## Run tests

To run the tests, export MNEMONICS and NETWORK

```bash
export MNEMONICS="<mnemonics words>"
export NETWORK="<network>" # dev or test
```

run the following command

```bash
make test
```
