package main

import (
	"context"
	"log"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	cniTypes "github.com/cybercoder/ik8s-ovn-cni/pkg/cni/types"
	"github.com/cybercoder/ik8s-ovn-cni/pkg/k8s"
	"github.com/cybercoder/ik8s-ovn-cni/pkg/net_utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func cmdAdd(args *skel.CmdArgs) error {
	f, err := os.OpenFile("/var/log/ik8s-ovn-cni", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)

	k8sArgs := cniTypes.CniKubeArgs{}
	if err := types.LoadArgs(args.Args, &k8sArgs); err != nil {
		log.Printf("error loading args: %v", err)
		return err
	}
	// 1. find kubevirt vm name using kube api
	k8sClient, err := k8s.CreateClient()
	if err != nil {
		log.Printf("Error creating Kubernetes Client: %v", err)
		return err
	}
	pod, err := k8sClient.CoreV1().Pods(string(k8sArgs.K8S_POD_NAMESPACE)).Get(context.Background(), string(k8sArgs.K8S_POD_NAME), metav1.GetOptions{})
	if err != nil {
		log.Printf("Error getting pod: %v", err)
		return err
	}
	labels := pod.GetLabels()
	log.Printf("the vm name is %s", labels["vm.kubevirt.io/name"])
	vmName := labels["vm.kubevirt.io/name"]

	// 2. Create veth pair
	if len(vmName) > 8 {
		vmName = vmName[:8]
	}
	err = net_utils.CreateStableVeth("veth-"+vmName, args.IfName, args.Netns)
	if err != nil {
		log.Printf("Error creating veth pair: %v", err)
		return err
	}

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
