# **grid3_go:**
- grid3_go is a go client created to interact with threefold grid. It should manage CRUD operations for deployments on the grid.  

## **grid3_go provides the following functionality:**
- Managing deployments.
- Calculating needed changes based on current and desired states.
- Retreiving current state of the deployed infrastructure.

<br/>

## **grid3_go has the following components:**
### **GridClient should:**
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
<br/>

### **DeploymentManager should:**
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

<br/>

### **Loader:**
- Retreives workload current state from the grid.
- Convert from grid types to grid3_go types.

<br/>

### **NodeClient:**
- Uses gridproxy to get information about nodes, farms, and/or twins.
- Uses rmb client (from gridproxy) to interact with nodes.

<br/>

### **Subi:**
- Exposes an interface to interact with the chain.
- Allows mocking substrate-client for testing.