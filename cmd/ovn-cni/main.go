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
	"github.com/cybercoder/ik8s-ovn-cni/pkg/ovnnb"
	"github.com/cybercoder/ik8s-ovn-cni/pkg/ovs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func cmdAdd(args *skel.CmdArgs) error {
	log.Printf("ifName: %s", args.IfName)
	oclient, err := ovs.CreateOVSclient()
	if err != nil {
		return err
	}
	ovnClient, err := ovnnb.CreateOvnNbClient("tcp:192.168.12.177:6641")
	if err != nil {
		return err
	}
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

	reqBody := net_utils.IpAssignmentRequestBody{
		Namespace:          string(k8sArgs.K8S_POD_NAMESPACE),
		Name:               vmName,
		ContainerInterface: args.IfName,
		IpFamily:           "IPv4",
	}
	ipamResponse, err := net_utils.RequestAssignmentFromIPAM(reqBody)
	if err != nil {
		log.Printf("error from ipam %v", err)
		return err
	}

	if err := oclient.AddPort("br-int", args.IfName, "internal", ipamResponse.MacAddress); err != nil {
		log.Printf("Error adding port to ovs: %v", err)
		return err
	}

	if err := ovnClient.CreateLogicalPort("public", args.IfName, ipamResponse.MacAddress); err != nil {
		log.Printf("Error creating logical port on logical switch public: %v", err)
		return err
	}

	if err := net_utils.MoveIf2NS(args.IfName, args.Netns); err != nil {
		log.Printf("Error moving ovs generated port to target ns %s : %v", args.Netns, err)
		return err
	}
	_, ipNet, err := net.ParseCIDR(ipamResponse.Address + "/24")
	log.Printf("IpamRespond Address: %s, %s", ipamResponse.Address, ipNet.String())
	result := &types100.Result{
		IPs: []*types100.IPConfig{
			{
				Interface: types100.Int(0),
				Address:   *ipNet,
			},
		},
		CNIVersion: version.Current(),
		Interfaces: []*types100.Interface{
			{
				Mtu:     1500,
				Name:    args.IfName,
				Mac:     ipamResponse.MacAddress,
				Sandbox: args.Netns,
			},
		},
	}

	return types.PrintResult(result, version.Current())
}

func cmdDel(args *skel.CmdArgs) error {
	ovnClient, err := ovnnb.CreateOvnNbClient("tcp:192.168.12.177:6641")
	if err != nil {
		log.Printf("error on creating ovn client: %v", err)
		return err
	}
	ovsClient, err := ovs.CreateOVSclient()
	if err != nil {
		log.Printf("error on creating ovs client: %v", err)
		return err
	}

	err = ovnClient.DeleteLogicalPort("public", args.IfName)
	if err != nil {
		log.Printf("Error on deleting logical switch port %s: %v", args.IfName, err)
		return err
	}
	err = ovsClient.DelPort("br-int", args.IfName)
	if err != nil {
		log.Printf("Error on deleting port %s from ovs: %v", args.IfName, err)
		return err
	}

	return nil
}

func main() {
	runtime.LockOSThread()
	f, err := os.OpenFile("/var/log/ik8s-ovn-cni", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)
	// os.Stdout = f
	//os.Stderr = f

	funcs := skel.CNIFuncs{
		Add: cmdAdd,
		Del: cmdDel,
	}
	skel.PluginMainFuncs(funcs, version.All, "ovn-cni")
}
