package server

type NetworkParams struct {
	Name               string `json:"name"`
	IPRange            string `json:"ip_range"`
	AddWireguardAccess bool   `json:"add_wireguard_access"`
	Description        string `json:"description"`
}

type MachinesModel struct {
	Name        string        `json:"name"`
	Network     NetworkParams `json:"network"`
	Machines    []Machine     `json:"machines"`
	Metadata    string        `json:"metadata"`
	Description string        `json:"description"`
}

type Machine struct {
	Name       string `json:"name"`
	NodeID     uint32 `json:"node_id"`
	Disks      []Disk `json:"disks"`
	PublicIP   bool   `json:"public_ip"`
	Planetary  bool   `json:"planetary"`
	CPU        uint32 `json:"cpu"`
	Memory     uint64 `json:"memory"`
	RootFSSize uint64 `json:"rootfs_size"`
	Flist      string `json:"flist"`
	Entrypoint string `json:"entrypoint"`
	SSHKey     string `json:"ssh_key"`
}

type Disk struct {
	Name       string `json:"name"`
	Size       uint32 `json:"size"`
	Mountpoint string `json:"mountpoint"`
}

type MachinesResult struct {
	NetworkResult NetworkResult   `json:"network_result"`
	MachineResult []MachineResult `json:"machine_result"`
}

type NetworkResult struct {
	WireguardConfig string `json:"wireguard_config"`
}

type MachineResult struct {
	Name      string `json:"name"`
	PublicIP  string `json:"public_ip"`
	PublicIP6 string `json:"public_ip6"`
	YggIP     string `json:"ygg_ip"`
}
