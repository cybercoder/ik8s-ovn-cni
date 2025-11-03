package net_utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func RequestAssignmentFromIPAM(reqBody IpAssignmentRequestBody) (*IpAssignmentResponseBody, error) {
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post("http://172.16.35.15:8000/apis/ovn.ik8s.ir/v1alpha1/assignip", "application/json", bytes.NewBuffer(jsonData))
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

func MoveIf2NS(generatedName, ifName, netnsPath string) (*string, error) {
	origNS, _ := netns.Get()
	defer netns.Set(origNS)

	// Get netns handle first
	targetNS, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get target netns: %w", err)
	}

	// Wait for OVS to actually create the interface
	link, err := waitLink(generatedName, 3*time.Second)
	if err != nil {
		return nil, err
	}

	// Move into target namespace
	if err := netlink.LinkSetNsFd(link, int(targetNS)); err != nil {
		return nil, fmt.Errorf("failed to move interface: %w", err)
	}

	// Switch context → work inside pod namespace
	if err := netns.Set(targetNS); err != nil {
		return nil, fmt.Errorf("failed entering target netns: %w", err)
	}
	// Bring lo UP
	lo, err := waitLink("lo", 3*time.Second)
	if err != nil {
		return nil, err
	}
	ip, ipNet, err := net.ParseCIDR("127.0.0.1/8")
	netlink.AddrAdd(lo, &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ip,
			Mask: ipNet.Mask,
		},
	})
	if err := netlink.LinkSetUp(lo); err != nil {
		return nil, fmt.Errorf("failed setting link up: %w", err)
	}
	// Wait again because rename appears *after* move
	link, err = waitLink(generatedName, 3*time.Second)
	if err != nil {
		return nil, err
	}

	// Rename
	if err := netlink.LinkSetName(link, ifName); err != nil {
		return nil, fmt.Errorf("failed renaming link: %w", err)
	}

	// Wait for rename
	link, err = waitLink(ifName, 2*time.Second)
	if err != nil {
		return nil, err
	}

	mac := link.Attrs().HardwareAddr.String()
	return &mac, nil

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

func waitLink(name string, timeout time.Duration) (netlink.Link, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		link, err := netlink.LinkByName(name)
		if err == nil {
			return link, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("timeout waiting for link %s", name)
}
