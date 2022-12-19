GridClient should:
- Handle interaction with the chain (Create, Update, Delete).
- Handle interaction with the nodes. (Create, Update, Delete).
- Be stateless.

Example:
```go
type GridClient interface{
    Create(deployments []grid.Deployment) (uint64, error)
    Update(deployments []grid.Deployment) error
    Delete(deploymentIDs []uint64) error
    Wait() error
}
```