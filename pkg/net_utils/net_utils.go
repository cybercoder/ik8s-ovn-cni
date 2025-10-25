package net_utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// CreateStableVeth creates a veth pair, keeps host side in current namespace
// and moves peer side to the target netns (container).
func CreateStableVeth(hostIf, ifName, netnsPath, macAddress, ipAddress string) (*string, *string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	peerIf := fmt.Sprintf("tmp-%s", hostIf)

	ns, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open target netns: %w", err)
	}
	defer ns.Close()

	origNS, err := netns.Get()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get current netns: %w", err)
	}
	defer origNS.Close()
	defer func() {
		if err := netns.Set(origNS); err != nil {
			log.Printf("failed to restore netns: %v", err)
		}
	}()

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostIf, MTU: 1500},
		PeerName:  peerIf,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, nil, fmt.Errorf("failed to create veth pair: %w", err)
	}

	hostLink, err := netlink.LinkByName(hostIf)
	if err != nil {
		return nil, nil, fmt.Errorf("host link lookup failed: %w", err)
	}
	if err := netlink.LinkSetUp(hostLink); err != nil {
		return nil, nil, fmt.Errorf("failed to bring up host link: %w", err)
	}
	hostMAC := hostLink.Attrs().HardwareAddr.String()

	peerLink, err := netlink.LinkByName(peerIf)
	if err != nil {
		return nil, nil, fmt.Errorf("peer link lookup failed: %w", err)
	}
	if err := netlink.LinkSetNsFd(peerLink, int(ns)); err != nil {
		return nil, nil, fmt.Errorf("failed to move peer to target netns: %w", err)
	}

	if err := netns.Set(ns); err != nil {
		return nil, nil, fmt.Errorf("failed to switch to target netns: %w", err)
	}

	peerLink, err = netlink.LinkByName(peerIf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find peer in container ns: %w", err)
	}

	if err := netlink.LinkSetName(peerLink, ifName); err != nil {
		return nil, nil, fmt.Errorf("failed to rename peer: %w", err)
	}

	if ipAddress != "" {
		ip, ipNet, err := net.ParseCIDR(ipAddress)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse IP address %s: %v", ipAddress, err)
		}
		peerLink, err = netlink.LinkByName(ifName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find renamed peer in container ns: %w", err)
		}
		if err := netlink.AddrAdd(peerLink, &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   ip,
				Mask: ipNet.Mask,
			},
		}); err != nil {
			return nil, nil, fmt.Errorf("failed to add IP address %s to interface %s: %w", ipAddress, ifName, err)
		}
		log.Printf("✅ Assigned IP address: %s to interface %s", ipAddress, ifName)
	}

	if macAddress != "" {
		hwAddr, err := net.ParseMAC(macAddress)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse MAC address %s: %w", macAddress, err)
		}
		if err := netlink.LinkSetHardwareAddr(peerLink, hwAddr); err != nil {
			return nil, nil, fmt.Errorf("failed to set MAC address %s: %w", macAddress, err)
		}
		log.Printf("✅ Set custom MAC address: %s on interface %s", macAddress, peerIf)
	}

	if err := netlink.LinkSetUp(peerLink); err != nil {
		return nil, nil, fmt.Errorf("failed to bring up peer: %w", err)
	}
	containerMAC := peerLink.Attrs().HardwareAddr.String()

	log.Printf("✅ Created veth pair: host=%s (%s) ↔ container=%s (%s)", hostIf, hostMAC, ifName, containerMAC)

	return &hostMAC, &containerMAC, nil
}

func WaitForNetns(netnsPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(netnsPath); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for netns %s", netnsPath)
}

func RequestAssignmentFromIPAM(reqBody IpAssignmentRequestBody) (*IpAssignmentResponseBody, error) {
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post("http://172.16.35.20:8000/apis/ovn.ik8s.ir/v1alpha1/assignip", "application/json", bytes.NewBuffer(jsonData))
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
