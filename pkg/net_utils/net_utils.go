package net_utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// CreateStableVeth creates a veth pair, keeps host side in current namespace
// and moves peer side to the target netns (container).
func CreateStableVeth(hostIf, ifName, netnsPath, macAddress string) (*string, *string, error) {
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

	if macAddress != "" {
		if err := netlink.LinkSetHardwareAddr(peerLink, []byte(macAddress)); err != nil {
			return nil, nil, fmt.Errorf("failed to set mac address for peer link: %w", err)
		}
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
	resp, err := http.Post("http://192.168.160.6:8000", "application/json", bytes.NewBuffer(jsonData))
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
