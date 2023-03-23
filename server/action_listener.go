package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/internal/config"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

var Cmds = map[string]func(ctx context.Context, data string) (response Response){
	"vm.deploy":       DeployVM,
	"login":           Login,
	"machines.deploy": MachinesDeploy,
	"machines.get":    MachinesGet,
	"machines.delete": MachinesDelete,
}

type RPCArgs struct {
	RetQueue string `json:"ret_queue"`
	Now      uint64 `json:"now"`
	Cmd      string `json:"cmd"`
	Data     string `json:"data"`
}

type Response struct {
	Result string `json:"result"`
	Err    string `json:"error"`
}

type RedisClient struct {
	Pool *redis.Pool
}

func NewRedisClient() (RedisClient, error) {
	pool, err := newRedisPool()
	if err != nil {
		return RedisClient{}, errors.Wrap(err, "failed to create new redis pool")
	}
	return RedisClient{
		Pool: pool,
	}, nil
}

func newRedisPool() (*redis.Pool, error) {
	return &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", "localhost:6379")
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) > 10*time.Second {
				_, err := c.Do("PING")
				return err
			}

			return nil
		},
		MaxActive:   100,
		IdleTimeout: 1 * time.Minute,
		Wait:        true,
	}, nil
}

// Listen watches a redis queue for incoming actions
func (r *RedisClient) Listen(ctx context.Context) {
	con := r.Pool.Get()
	defer con.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			res, err := redis.ByteSlices(con.Do("BRPOP", "tfgrid.client", 0))
			if err != nil {
				log.Err(err).Msg("failed to read from redis")
				return
			}

			go r.processAction(ctx, res[1])
		}
	}
}

func (r *RedisClient) processAction(ctx context.Context, message []byte) {
	args := RPCArgs{}
	err := json.Unmarshal(message, &args)
	if err != nil {
		log.Err(err).Msg("failed to unmarshal incoming message. message is dropped")
		return
	}

	err = validateArgs(args)
	if err != nil {
		log.Err(err).Msg("failed to validate incoming message. message is dropped")
		return
	}
	cmd, ok := Cmds[args.Cmd]
	if !ok {
		log.Error().Msg("invalid command. message is dropped")
		return
	}

	resopnse := cmd(ctx, args.Data)
	b, err := json.Marshal(resopnse)
	if err != nil {
		log.Err(err).Msg("failed to marshal response")
		return
	}

	con := r.Pool.Get()
	defer con.Close()

	_, err = con.Do("RPUSH", args.RetQueue, b)
	if err != nil {
		log.Err(err).Msg("failed to push response bytes into redis")
	}
}

func validateArgs(args RPCArgs) error {
	// any kind of validation on the incoming message should happen here
	return nil
}

func DeployVM(ctx context.Context, data string) Response {

	return Response{}
}

func Login(ctx context.Context, data string) Response {
	credentials := struct {
		Mnemonics string
		Network   string
	}{}
	err := json.Unmarshal([]byte(data), &credentials)
	if err != nil {
		resp := fmt.Errorf("failed to unmarshal credentials data. %w", err)
		log.Err(resp)
		return Response{
			Err: resp.Error(),
		}
	}
	log.Printf("cred: %+v", credentials)
	path, err := config.GetConfigPath()
	if err != nil {
		resp := fmt.Errorf("failed to get config path. %w", err)
		log.Err(resp)
		return Response{
			Err: resp.Error(),
		}
	}

	cfg := config.Config{}
	cfg.Mnemonics = credentials.Mnemonics
	cfg.Network = credentials.Network

	err = cfg.Save(path)
	if err != nil {
		resp := fmt.Errorf("failed to save user configs. %w", err)
		log.Err(resp)
		return Response{
			Err: resp.Error(),
		}
	}
	return Response{}
}

func MachinesDelete(ctx context.Context, data string) Response {
	projectName := data

	cfg, err := config.GetUserConfig()
	if err != nil {
		return Response{
			Err: fmt.Errorf("failed to unmarshal machines data. %w", err).Error(),
		}
	}

	client, err := deployer.NewTFPluginClient(cfg.Mnemonics, "sr25519", cfg.Network, "", "", "", true, false)
	if err != nil {
		return Response{
			Err: errors.Wrap(err, "failed to get tf plugin client").Error(),
		}
	}

	err = client.CancelByProjectName(projectName)
	if err != nil {
		return Response{
			Err: errors.Wrap(err, "failed to cancel project").Error(),
		}
	}

	return Response{}
}

func MachinesGet(ctx context.Context, data string) Response {
	projectName := data

	cfg, err := config.GetUserConfig()
	if err != nil {
		return Response{
			Err: fmt.Errorf("failed to unmarshal machines data. %w", err).Error(),
		}
	}

	client, err := deployer.NewTFPluginClient(cfg.Mnemonics, "sr25519", cfg.Network, "", "", "", true, false)
	if err != nil {
		return Response{
			Err: errors.Wrap(err, "failed to get tf plugin client").Error(),
		}
	}

	contracts, err := client.ContractsGetter.ListContractsOfProjectName(projectName)
	if err != nil {
		return Response{
			Err: errors.Wrap(err, "failed to list project contracts").Error(),
		}
	}

	deployments := []gridtypes.Deployment{}
	for _, id := range contracts.NodeContracts {
		nodeClient, err := client.NcPool.GetNodeClient(client.SubstrateConn, id.NodeID)
		if err != nil {
			return Response{
				Err: errors.Wrapf(err, "failed to get node %d client", id.NodeID).Error(),
			}
		}
		contractID, err := strconv.Atoi(id.ContractID)
		if err != nil {
			return Response{
				Err: errors.Wrapf(err, "failed to parse contract id (%s)", id.ContractID).Error(),
			}
		}

		dl, err := nodeClient.DeploymentGet(ctx, uint64(contractID))
		if err != nil {
			return Response{
				Err: errors.Wrapf(err, "failed to get deployment with id %d", contractID).Error(),
			}
		}
		deployments = append(deployments, dl)
	}

	deploymentsBytes, err := json.Marshal(deployments)
	if err != nil {
		return Response{
			Err: errors.Wrap(err, "failed to marshal deployments").Error(),
		}
	}

	return Response{
		Result: string(deploymentsBytes),
	}
}

func MachinesDeploy(ctx context.Context, data string) (response Response) {
	machinesModel := MachinesModel{}
	err := json.Unmarshal([]byte(data), &machinesModel)
	if err != nil {
		return Response{
			Err: fmt.Errorf("failed to unmarshal machines data. %w", err).Error(),
		}
	}

	cfg, err := config.GetUserConfig()
	if err != nil {
		return Response{
			Err: fmt.Errorf("failed to unmarshal machines data. %w", err).Error(),
		}
	}

	client, err := deployer.NewTFPluginClient(cfg.Mnemonics, "sr25519", cfg.Network, "", "", "", true, false)
	if err != nil {
		return Response{
			Err: errors.Wrap(err, "failed to get tf plugin client").Error(),
		}
	}

	vms, disks, network, err := extractWorkloads(machinesModel)
	if err != nil {
		return Response{
			Err: errors.Wrap(err, "failed to extract workloads data").Error(),
		}
	}

	resVMs, wgConfgig, err := client.DeployMachines(machinesModel.Name, vms, disks, network)
	if err != nil {
		return Response{
			Err: errors.Wrap(err, "failed to deploy machines").Error(),
		}
	}

	machineRes := getMachinesResult(resVMs, wgConfgig)
	machineResBytes, err := json.Marshal(machineRes)
	if err != nil {
		log.Err(err)
		return Response{Err: err.Error()}
	}
	return Response{Result: string(machineResBytes)}
}

func extractWorkloads(m MachinesModel) ([]workloads.VM, []workloads.Disk, workloads.ZNet, error) {
	vms := []workloads.VM{}
	disks := []workloads.Disk{}
	for _, vm := range m.Machines {
		mounts := []workloads.Mount{}
		for _, disk := range vm.Disks {
			disks = append(disks, workloads.Disk{
				Name:   disk.Name,
				SizeGB: int(disk.Size),
			})
			mounts = append(mounts, workloads.Mount{
				DiskName:   disk.Name,
				MountPoint: disk.Mountpoint,
			})
		}
		vms = append(vms, workloads.VM{
			Name:       vm.Name,
			Flist:      vm.Flist,
			PublicIP:   vm.PublicIP,
			Planetary:  vm.Planetary,
			CPU:        int(vm.CPU),
			Memory:     int(vm.Memory),
			RootfsSize: int(vm.RootFSSize),
			Entrypoint: vm.Entrypoint,
			EnvVars: map[string]string{
				"SSH_KEY": vm.SSHKey,
			},
			NetworkName: m.Network.Name,
			Mounts:      mounts,
		})
	}
	ip, err := gridtypes.ParseIPNet(m.Network.IPRange)
	if err != nil {
		return nil, nil, workloads.ZNet{}, errors.Wrap(err, "failed to parse ip range")
	}
	network := workloads.ZNet{
		Name:        m.Network.Name,
		Description: m.Network.Description,
		IPRange:     ip,
		AddWGAccess: m.Network.AddWireguardAccess,
	}
	return vms, disks, network, nil
}

func getMachinesResult(vms []workloads.VM, wgConfig string) MachinesResult {
	machinesResult := MachinesResult{}
	for _, vm := range vms {
		machinesResult.MachineResult = append(machinesResult.MachineResult, MachineResult{
			Name:      vm.Name,
			PublicIP:  vm.ComputedIP,
			PublicIP6: vm.ComputedIP6,
			YggIP:     vm.YggIP,
		})
	}
	machinesResult.NetworkResult.WireguardConfig = wgConfig
	return machinesResult
}
