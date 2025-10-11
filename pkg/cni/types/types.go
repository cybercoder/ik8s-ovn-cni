package types

import "github.com/containernetworking/cni/pkg/types"

type CniKubeArgs struct {
	types.CommonArgs
	K8S_POD_NAME               types.UnmarshallableString
	K8S_POD_NAMESPACE          types.UnmarshallableString
	K8S_POD_INFRA_CONTAINER_ID types.UnmarshallableString
	K8S_POD_UID                types.UnmarshallableString
}

// NetConf models the JSON CNI config that Multus will pass in stdin.
type NetConf struct {
	types.NetConf
	Bridge        string         `json:"bridge"`        // e.g. "br-int"
	LogicalSwitch string         `json:"logicalSwitch"` // e.g. "ls-vm-net"
	OVNNB         string         `json:"ovnNb"`         // e.g. "tcp:192.168.12.177:6641"
	IPAM          map[string]any `json:"ipam,omitempty"`
}
