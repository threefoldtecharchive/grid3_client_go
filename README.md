# **grid3_go:**

grid3_go is a go client created to interact with threefold grid. It should manage CRUD operations for deployments on the grid.

- ## **Grid3_go flow:**

  1. the deployment manager will be initialized with identity/network information.
  2. deployment manager will expose the `deploy` which the user will call it and give it old/new deployments
  3. `deploy` method will do the following
     - internally call `calculateChanges` method which
       1. loads old deployments using their ids from teh grid
       2. determine which things needs to be created, updated and deleted
     - will take the sutable action for each operation to create, update and delete
     - waits on them and report the state
  4. For applying the changes we have subi and node pkgs which creates contracts/deployments on the grid
  5. if a user wishes to apply any changes, they should provide their new state, and their current deployment ids.
  6. the deployment manager should be responsible of reverting the applied changes if some error happens midway.

- ### **How the deployment manager calculates changes:**

- the deployment manager receives informantion about old and new deployments, then decides which operations need to be performed.

  1. deployments that need to be created are present in the new deployments, and not in the old deployments.
  2. deployments that need to be deleted are present in the old deployments, and not in the new deployments.
  3. deployments that need to be updated are present in both old and new deployments, but they must have different hashes.

- ### **Creating a new Deployment:**

  1. deployments must first be signed and validated.
  2. a deployment contract then must be created.
  3. Then, the deployment should be deployed on the node.
  4. If some error happens while trying to deploy on the node, the contract should be canceled to avoid leaking a contract.
  5. after deployment creating, the function should only return after waiting on all workloads to be StateOK.

- ### **Deleting a deployment:**

  1. Deleting a deployment is only done on the chain, by canceling the deployment contract.

- ### **Updating a deployment:**

  1. a deployment should be updated if and only if hashes differ or a workload name was changed.
  2. if a deployment should be updated, its version should be incremented.
  3. if a workload should be updated, its version should match the deployment version.
  4. if a workload shouldn't be updated, its version statys the same.
  5. the deployment contract should then, be updated.
  6. The deployment on the node should be updated.
  7. after deployment update, the function should only return after waiting on all workloads to be StateOK.

- ### **Generating a versionless deployemnt:**

  1. a user should first generate the appropriate deployment builder using the `builder` package.
  2. the `builder` package then generates the grid.Deployment objects.

- ### **Reverting a deployment:**

  1. before applying any change, the deployment manager should first retreive the current state from the nodes `oldState`.
  2. at first, `currentState` of deployment ids is provided by the user (as `oldDeloymentIDs`).
  3. every contract deletion or creation, should directly be reflected in the `currentState`.
  4. if some error happens while applying some change, the deployment manager should revert to its old state using the `currentState` as the `oldDeploymentIDs` and the `oldState` as the `newDeployemnts`.

    Example:

       - oldDeploymentIDs :1, 2, 3 - newDeployments: 3, 4, 5
       - desired state: 3 update, 4 create, 5 create
       - deploymentManager.deploy(oldDeploymentIDs[1,2,3], newDeployments[3,4,5])
       - error happens: 1 deleted, 2 not affected, 3 updated, 4 created, 5 not created
       - currentState: 2, 3, 4
       - deploymentManager.deploy(currentState[2,3,4], oldDeployments[1,2,3])

- ### **Retrieving current state:**

  1. grid3_go users should mainly keep track of contract ids
  2. the deployment manager should use the provided contract ids to retrieve current state from nodes (using `client` package).
  
## **grid3_go has the following components:**

- ### **GridClient:**

  - Handles interaction with the chain (Create, Update, Delete).
  - Handles interaction with the nodes. (Create, Update, Delete).
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

- ### **DeploymentManager:**

  - Calculates needed changes between different provided states.
  - Uses the GridClient to apply needed changes.
  - Retreives current state of specified deployments.

```go
type DeploymentManager interface{
    CalculateChanges(current NodeDeploymentIDMap, new NodeDeploymentMap) ([]create, []update, []delete, error)
    ApplyChanges(current NodeDeploymentIDMap, new NodeDeplyomentMap) (current NodeDeploymentIDMap,error)
    GetCurrentDeployments(current NodeDeploymentIDMap) (NodeDeploymentMap, error)
}
```

- ### **Loader:**

  - Retreives workload current state from the grid.
  - Convert from grid types to grid3_go types.

- ### **NodeClient:**

  - Uses gridproxy to get information about nodes, farms, and/or twins.
  - Uses rmb client (from gridproxy) to interact with nodes.

- ### **Subi:**

  - Exposes an interface to interact with the chain.
  - Allows mocking substrate-client for testing.

- ### **Builder:**

  - builder package is mainly responsible of:

  1. Expose the needed user friendly builder structs.
  2. Generate needed grid deployments from builders to be used by the deployer.
