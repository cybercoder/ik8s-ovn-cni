package main

import (
	"context"
	"log"
	"net"
	"os"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
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

	log.Printf("ifName: %s", args.IfName)

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
	_, netMask, _ := net.ParseCIDR("172.16.22.0/24")
	veths, err := net_utils.GetVethList(args.Netns)
	if err != nil {
		log.Printf("%v", err)
	}
	reqBody := net_utils.IpAssignmentRequestBody{
		Namespace:          string(k8sArgs.K8S_POD_NAMESPACE),
		Name:               vmName,
		ContainerInterface: veths[0].Attrs().Name,
		IpFamily:           "IPv4",
	}
	ipamResponse, err := net_utils.RequestAssignmentFromIPAM(reqBody)
	if err != nil {
		log.Printf("error from ipam %v", err)
		return err
	}

	if err := net_utils.PrepareLink(args.Netns, 0, args.IfName, *ipamResponse); err != nil {
		log.Printf("%v", err)
	}

	result := &types100.Result{

		CNIVersion: version.Current(),
		Interfaces: []*types100.Interface{
			{
				Mtu:     1500,
				Name:    "eth0",
				Mac:     veths[0].Attrs().HardwareAddr.String(),
				Sandbox: args.Netns,
			},
		},
		IPs: []*types100.IPConfig{
			{
				Interface: types100.Int(0),
				Address:   net.IPNet{IP: net.ParseIP(ipamResponse.Address).To4(), Mask: net.IPMask(netMask.Mask.String())},
			},
		},
	}

	return types.PrintResult(result, version.Current())
}

func cmdDel(args *skel.CmdArgs) error {
	return nil
}

func main() {
	funcs := skel.CNIFuncs{
		Add: cmdAdd,
		Del: cmdDel,
	}
	runtime.LockOSThread()
	skel.PluginMainFuncs(funcs, version.All, "ovn-cni")
}
