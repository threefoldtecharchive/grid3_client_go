# Workflows

## lint workflow

Mainly to run the formatters, golangci-lint and staticcheck

## test workflow

- Runs all tests
- Uses GoReleaser to test building the client.

## release workflow

- Uses go-releaser to publish a release for a created tag
