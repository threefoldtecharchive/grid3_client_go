package server

type VMParams struct {
	Name       string
	SSHKey     string
	CPU        int
	Memory     int
	RootFS     int
	Disk       int
	Flist      string
	Entrypoint string
	IPV4       bool
	IPV6       bool
	Ygg        bool
}

type K8sParams struct {
	Name          string
	SSHKey        string
	MasterCPU     int
	MasterMemory  int
	MasterDisk    int
	WorkersNumber int
	WorkerCPU     int
	WorkerMemory  int
	WorkerDisk    int
	IPV4          bool
	IPV6          bool
	Ygg           bool
}

type GatewayBase struct {
	Name     string
	Backends []string
	Node     uint32
	TLS      bool
}

type GatewayNameParams struct {
	GatewayBase
}

type GatewayFQDNParams struct {
	GatewayBase
	FQDN string
}
