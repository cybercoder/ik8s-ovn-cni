package net_utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"github.com/samber/lo"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func GetVethList(netnsPath string) ([]netlink.Link, error) {
	ns, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to enter target netns: %v", err)
	}
	defer ns.Close()

	if err := netns.Set(ns); err != nil {
		return nil, fmt.Errorf("failed to enter target netns: %v", err)
	}

	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to get link list in ns %s: %v", ns.String(), err)
	}

	veths := lo.Filter(links, func(l netlink.Link, _ int) bool {
		return l.Type() == "veth"
	})

	if len(veths) == 0 {
		return nil, fmt.Errorf("no tap link in ns %s: %v", ns.String(), err)
	}

	return veths, nil
}

func RequestAssignmentFromIPAM(reqBody IpAssignmentRequestBody) (*IpAssignmentResponseBody, error) {
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(os.Getenv("IPAM_URI")+"/apis/ovn.ik8s.ir/v1alpha1/assignip", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	result := &IpAssignmentResponseBody{}
	err = json.Unmarshal(respBody, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func PrepareLink(netnsPath string, ifIndex int, finalIfName string, ipamResponse IpAssignmentResponseBody) error {
	veths, err := GetVethList(netnsPath)
	if err != nil {
		return err
	}
	ns, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to enter target netns: %v", err)
	}
	defer ns.Close()

	if err := netns.Set(ns); err != nil {
		return fmt.Errorf("failed to enter target netns: %v", err)
	}
	if err := netlink.LinkSetHardwareAddr(veths[ifIndex], net.HardwareAddr(ipamResponse.MacAddress)); err != nil {
		return err
	}
	ip, ipNet, err := net.ParseCIDR(ipamResponse.Address + "/32")
	if err := netlink.AddrAdd(veths[ifIndex], &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ip,
			Mask: ipNet.Mask,
		},
	}); err != nil {
		return err
	}
	if err := netlink.LinkSetName(veths[ifIndex], finalIfName); err != nil {
		return err
	}
	if err := netlink.LinkSetUp(veths[ifIndex]); err != nil {
		return err
	}
	return nil
}
