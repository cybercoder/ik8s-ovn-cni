package net_utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/samber/lo"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func GetVethList(netnsPath string) ([]netlink.Link, error) {
	origNS, err := netns.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get current netns: %w", err)
	}
	defer origNS.Close()
	defer func() {
		if err := netns.Set(origNS); err != nil {
			log.Printf("failed to restore netns: %v", err)
		}
	}()
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
		log.Printf("name: %s type: %s \n", l.Attrs().Name, l.Type())
		return l.Type() == "veth"
	})

	if len(veths) == 0 {
		return nil, fmt.Errorf("no tap link in ns %s: %v", ns.String(), err)
	}

	return veths, nil
}

func RequestAssignmentFromIPAM(reqBody IpAssignmentRequestBody) (*IpAssignmentResponseBody, error) {
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post("http://172.16.35.16:8000/apis/ovn.ik8s.ir/v1alpha1/assignip", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	result := &IpAssignmentResponseBody{}
	err = json.Unmarshal(respBody, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func PrepareLink(namespace, name, netnsPath, ifName, ipAddress, macAddress string) error {
	ns, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to get target netns: %v", err)
	}
	defer ns.Close()

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: GenerateVethIfName(name, namespace, ifName),
			MTU:  1500,
		},
		PeerName:         ifName,
		PeerNamespace:    ns,
		PeerMTU:          1500,
		PeerHardwareAddr: net.HardwareAddr(macAddress),
	}

	ip, ipNet, err := net.ParseCIDR(ipAddress + "/24")
	if err := netlink.AddrAdd(veth, &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ip,
			Mask: ipNet.Mask,
		},
	}); err != nil {
		return fmt.Errorf("failed to ser ip address for veth interface: %v", err)
	}

	if err := netlink.LinkSetUp(veth); err != nil {
		return fmt.Errorf("failed to ser veth interface up: %v", err)
	}
	return nil
}

func GenerateVethIfName(name, namespace, ifName string) string {
	input := fmt.Sprintf("%s/%s/%s", namespace, name, ifName)

	// Create SHA-256 hash
	hash := sha256.Sum256([]byte(input))

	// Take first 13 characters of hex encoding
	hexString := hex.EncodeToString(hash[:])

	if len(hexString) > 13 {
		return hexString[:13]
	}
	return hexString
}
