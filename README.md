# **grid3_go:**

- grid3_go is a go client created to interact with threefold grid. It should manage CRUD operations for deployments on the grid.

## Grid3_go flow

1. the deployment manager will be initialized with identity/network information
2. deployment manager will expose the `deploy` which the user will call it and give it old/new deployments
3. `deploy` method will do the following
   - internally call `calculateChanges` method which
     1. loads old deployments using their ids from teh grid
     2. determine which things needs to be created, updated and deleted
   - will take the sutable action for each operation to create, update and delete
   - waits on them and report the state
4. For applying the changes we have subi and node pkgs which creates contracts/deployments on the grid

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
