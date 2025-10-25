package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

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

	f, err := os.OpenFile("/var/log/ik8s-ovn-cni", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)
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
	if err := net_utils.WaitForNetns(args.Netns, 10*time.Second); err != nil {
		log.Printf("Network namespace not ready: %v", err)
		return err
	}
	hostIf := fmt.Sprintf("veth-%s", vmName)
	if len(hostIf) > 15 {
		hostIf = hostIf[:15]
	}
	// 2. Request MAC and IP address from IPAM.

	reqBody := net_utils.IpAssignmentRequestBody{
		Namespace:          string(k8sArgs.K8S_POD_NAMESPACE),
		Name:               vmName,
		ContainerInterface: "eth0",
		IpFamily:           "IPv4",
	}
	ipamResponse, err := net_utils.RequestAssignmentFromIPAM(reqBody)
	if err != nil {
		log.Printf("error from ipam %v", err)
		return err
	}

	// 3. Create veth pair

	hostMAC, containerMac, err := net_utils.CreateStableVeth(hostIf, args.IfName, args.Netns, ipamResponse.MacAddress, ipamResponse.Address)
	if err != nil {
		log.Printf("Error creating veth pair: %v", err)
		// return err
	}

	// 4. Add port to ovs
	err = oclient.AddPort("br-int", hostIf, "system", *hostMAC)
	if err != nil {
		log.Printf("Error adding port to ovs: %v", err)
		// return err
	}

	// 5. Add port to ovn logical switch
	log.Printf("mac address %s", *hostMAC)
	err = ovnClient.CreateLogicalPort("public", hostIf, *containerMac)
	if err != nil {
		log.Printf("Error creating logical port on logical switch public: %v", err)
		// return err
	}

	// ✅ Build minimal CNI result
	_, ipNet, err := net.ParseCIDR(ipamResponse.Address + "/32")
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
				Mac:     *hostMAC,
				Sandbox: args.Netns,
			},
		},
	}

	// ✅ Print JSON to stdout for CNI runtime
	return types.PrintResult(result, version.Current())
}

func cmdDel(args *skel.CmdArgs) error {
	f, err := os.OpenFile("/var/log/ik8s-ovn-cni", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)

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
	hostIf := fmt.Sprintf("veth-%s", vmName)
	if len(hostIf) > 15 {
		hostIf = hostIf[:15]
	}
	err = ovnClient.DeleteLogicalPort("public", hostIf)
	if err != nil {
		log.Printf("Error on deleting logical switch port %s: %v", hostIf, err)
		return err
	}
	err = ovsClient.DelPort("br-int", hostIf)
	if err != nil {
		log.Printf("Error on deleting port %s from ovs: %v", hostIf, err)
		return err
	}

	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, "ovn-cni")
}
