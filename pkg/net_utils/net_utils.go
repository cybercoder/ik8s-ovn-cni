package net_utils

import (
	"crypto/md5"
	"fmt"
	"log"
	"net"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// CreateStableVeth creates a veth pair, keeps host side in current namespace
// and moves peer side to the target netns (container).
func CreateStableVeth(hostIf, ifName, netnsPath string) (*string, *string, error) {
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

	if err := netlink.LinkSetUp(peerLink); err != nil {
		return nil, nil, fmt.Errorf("failed to bring up peer: %w", err)
	}
	containerMAC := peerLink.Attrs().HardwareAddr.String()

	log.Printf("✅ Created veth pair: host=%s (%s) ↔ container=%s (%s)", hostIf, hostMAC, ifName, containerMAC)

	return &hostMAC, &containerMAC, nil
}

func GenerateMAC(unique string) (string, error) {
	hash := md5.Sum([]byte(unique)) // 16 bytes
	mac := net.HardwareAddr{
		0x02,    // Locally administered, unicast
		hash[0], // Use hash bytes for uniqueness
		hash[1],
		hash[2],
		hash[3],
		hash[4],
	}
	return mac.String(), nil
}

func BringInterfaceUp(nic string) error {
	link, err := netlink.LinkByName(nic)
	if err != nil {
		return fmt.Errorf("cannot find host interface %s: %w", nic, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring host interface up: %w", err)
	}
	return nil
}
