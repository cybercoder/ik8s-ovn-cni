package main

import (
	"log"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	cniTypes "github.com/cybercoder/ik8s-ovn-cni/pkg/cni/types"
)

func cmdAdd(args *skel.CmdArgs) error {
	f, err := os.OpenFile("/var/log/ik8s-ovn-cni", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)

	k8sArgs := cniTypes.CniKubeArgs{}
	if err := types.LoadArgs(args.Args, k8sArgs); err != nil {
		log.Printf("error loading args: %v", err)
		return err
	}
	// 1. find kubevirt vm name using kube api
	log.Println(k8sArgs)
	return nil
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
