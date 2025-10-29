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

func PrepareLink(generatedName, netnsPath, ifName, ipAddress, macAddress string) (*string, error) {
	origNS, _ := netns.Get()
	defer netns.Set(origNS)

	hostIfName := "v-" + generatedName
	peerTempName := "t-" + generatedName

	targetNS, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get target netns: %v", err)
	}
	defer targetNS.Close()

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: hostIfName,
			MTU:  1500,
		},
		PeerName: peerTempName,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, fmt.Errorf("failed to create veth interface: %v", err)
	}

	peer, err := netlink.LinkByName(peerTempName)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer veth: %v", err)
	}

	if err := netlink.LinkSetNsFd(peer, int(targetNS)); err != nil {
		return nil, fmt.Errorf("failed to move peer veth to target ns: %v", err)
	}

	err = netns.Set(targetNS)
	if err != nil {
		return nil, fmt.Errorf("failed to enter target netns: %v", err)
	}
	defer netns.Set(netns.None())

	peer, _ = netlink.LinkByName(peerTempName)
	hw, _ := net.ParseMAC(macAddress)
	if err := netlink.LinkSetHardwareAddr(peer, hw); err != nil {
		return nil, fmt.Errorf("failed to set MAC: %v", err)
	}

	ip, ipNet, err := net.ParseCIDR(ipAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid IP: %v", err)
	}
	if err := netlink.AddrAdd(peer, &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ip,
			Mask: ipNet.Mask,
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to set ip address for veth interface: %v", err)
	}
	netlink.RouteAdd(&netlink.Route{
		Dst: &net.IPNet{
			IP:   net.ParseIP("192.168.12.1"),
			Mask: net.CIDRMask(32, 32), // /32 mask for single host
		},
		LinkIndex: peer.Attrs().Index,
		Scope:     253,
	})
	if err := netlink.LinkSetName(peer, ifName); err != nil {
		return nil, fmt.Errorf("failed to set veth peer interface name: %v", err)
	}
	if err := netlink.LinkSetUp(peer); err != nil {
		return nil, fmt.Errorf("failed to set veth peer interface up: %v", err)
	}

	netns.Set(origNS)
	if err := netlink.LinkSetUp(veth); err != nil {
		return nil, fmt.Errorf("failed to set veth interface up: %v", err)
	}
	hostLink, err := netlink.LinkByName(hostIfName)
	if err != nil {
		return nil, fmt.Errorf("host link lookup failed: %w", err)
	}
	hostMAC := hostLink.Attrs().HardwareAddr.String()
	return &hostMAC, nil
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
