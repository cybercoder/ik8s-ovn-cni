package main

import (
	"context"
	"fmt"
	"log"
	"os"

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
	hostIf := fmt.Sprintf("veth-%s", vmName)
	if len(hostIf) > 15 {
		hostIf = hostIf[:15]
	}
	// 2. Create veth pair

	// hostMAC, containerMac, err := net_utils.CreateStableVeth(hostIf, args.IfName, args.Netns)
	// if err != nil {
	// 	log.Printf("Error creating veth pair: %v", err)
	// 	// return err
	// }

	// // 3. Add port to ovs
	// err = oclient.AddPort("br-int", hostIf, "system", *hostMAC)
	// if err != nil {
	// 	log.Printf("Error adding port to ovs: %v", err)
	// 	// return err
	// }
	// realmac, _ := net_utils.GenerateMAC(hostIf)
	err = oclient.AddManagedTapPort("br-int", hostIf)
	if err != nil {
		log.Printf("Error on Add managed tap port to br-int: %v", err)
	}
	// realmac, err := oclient.WaitForPortMAC(hostIf, 30*time.Second)
	realmac, err := net_utils.GetInterfaceMAC(hostIf)
	if err != nil {
		log.Printf("Error on getting mac address for %s: %v", hostIf, err)
	}

	// 4. Add port to ovn logical switch
	log.Printf("mac address %s", realmac)
	// err = ovnClient.CreateLogicalPort("public", hostIf, *containerMac)
	err = ovnClient.CreateLogicalPort("public", hostIf, realmac)
	if err != nil {
		log.Printf("Error creating logical port on logical switch public: %v", err)
		// return err
	}
	err = net_utils.BringInterfaceUp(hostIf)
	if err != nil {
		log.Printf("%v", err)
	}
	// ✅ Build minimal CNI result
	result := &types100.Result{

		CNIVersion: version.Current(),
		Interfaces: []*types100.Interface{
			{
				Name:    args.IfName,
				Mac:     realmac,
				Sandbox: args.Netns,
			},
		},
	}

	// ✅ Print JSON to stdout for CNI runtime
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
