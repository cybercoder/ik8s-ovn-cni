package config

import "github.com/containernetworking/cni/pkg/types"

// NetConf models the JSON CNI config that Multus will pass in stdin.
type NetConf struct {
	types.NetConf
	Bridge        string         `json:"bridge"`        // e.g. "br-int"
	LogicalSwitch string         `json:"logicalSwitch"` // e.g. "ls-vm-net"
	OVNNB         string         `json:"ovnNb"`         // e.g. "tcp:192.168.12.177:6641"
	IPAM          map[string]any `json:"ipam,omitempty"`
}
