package cni

import (
	"encoding/json"
	"fmt"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/cybercoder/ik8s-ovn-cni/pkg/config"
	"github.com/cybercoder/ik8s-ovn-cni/pkg/ovn"
	"github.com/cybercoder/ik8s-ovn-cni/pkg/ovs"
)

func CmdAdd(args *skel.CmdArgs, ovnClient *ovn.Client, ovsClient *ovs.Client) error {
	conf := &config.NetConf{}
	if err := json.Unmarshal(args.StdinData, conf); err != nil {
		return err
	}
	hostIfName := fmt.Sprintf("veth%s", args.ContainerID[:5])
	containerIfName := args.IfName

	hostVeth, _, err := setupVethPair(containerIfName, hostIfName, args.Netns)
	if err != nil {
		return fmt.Errorf("failed to create veth: %w", err)
	}

	if err := ovsClient.AddPort("br-int", hostVeth.Attrs().Name); err != nil {
		return fmt.Errorf("failed to add port to br-int: %w", err)
	}

	return nil
}
