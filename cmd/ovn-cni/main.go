package main

import (
	"log"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/cybercoder/ik8s-ovn-cni/pkg/net_utils"
)

func cmdAdd(args *skel.CmdArgs) error {

	f, err := os.OpenFile("/var/log/ik8s-ovn-cni", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)

	log.Printf("ifName: %s", args.IfName)
	veths, err := net_utils.GetVethList(args.Netns)
	if err != nil {
		log.Printf("%v", err)
	}

	log.Printf("veth 0: %s", veths[0].Attrs().HardwareAddr.String())

	result := &types100.Result{

		CNIVersion: version.Current(),
		Interfaces: []*types100.Interface{
			{
				Name:    args.IfName,
				Mac:     veths[0].Attrs().HardwareAddr.String(),
				Sandbox: args.Netns,
			},
		},
	}

	return types.PrintResult(result, version.Current())
}

func cmdDel(args *skel.CmdArgs) error {
	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, "ovn-cni")
}
