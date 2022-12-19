DeploymentManager should do the following:
- Calculate needed changes between different provided states.
- Use the GridClient to apply needed changes.
- Retreive current state of specified deployments.

```go
type DeploymentManager interface{
    CalculateChanges(current NodeDeploymentIDMap, new NodeDeploymentMap) ([]create, []update, []delete, error)
    ApplyChanges(current NodeDeploymentIDMap, new NodeDeplyomentMap) (current NodeDeploymentIDMap,error)
    GetCurrentDeployments(current NodeDeploymentIDMap) (NodeDeploymentMap, error)
}
```