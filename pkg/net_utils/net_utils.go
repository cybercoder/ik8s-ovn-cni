package net_utils

import (
	"fmt"
	"log"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// CreateStableVeth creates a veth pair, keeps host side in current namespace
// and moves peer side to the target netns (container).
func CreateStableVeth(hostIf, ifName, netnsPath string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	peerIf := "tmp0"

	// Open target namespace
	ns, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to open target netns: %w", err)
	}
	defer ns.Close()

	// Save current namespace to restore later
	origNS, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current netns: %w", err)
	}
	defer origNS.Close()
	defer netns.Set(origNS)

	// 1. Create veth pair in host namespace
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: hostIf,
			MTU:  1500,
		},
		PeerName: peerIf,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("failed to create veth pair: %w", err)
	}

	// 2. Bring up host side
	hostLink, err := netlink.LinkByName(hostIf)
	if err != nil {
		return fmt.Errorf("host link lookup failed: %w", err)
	}
	if err := netlink.LinkSetUp(hostLink); err != nil {
		return fmt.Errorf("failed to bring up host link: %w", err)
	}

	// 3. Move peer side to container netns
	peerLink, err := netlink.LinkByName(peerIf)
	if err != nil {
		return fmt.Errorf("peer link lookup failed: %w", err)
	}
	if err := netlink.LinkSetNsFd(peerLink, int(ns)); err != nil {
		return fmt.Errorf("failed to move peer to target netns: %w", err)
	}

	// 4. Configure container side
	if err := netns.Set(ns); err != nil {
		return fmt.Errorf("failed to switch to target netns: %w", err)
	}

	peerLink, err = netlink.LinkByName(peerIf)
	if err != nil {
		return fmt.Errorf("failed to find peer in container ns: %w", err)
	}

	if err := netlink.LinkSetName(peerLink, ifName); err != nil {
		return fmt.Errorf("failed to rename peer: %w", err)
	}

	if err := netlink.LinkSetUp(peerLink); err != nil {
		return fmt.Errorf("failed to bring up peer: %w", err)
	}

	// Back to original namespace
	if err := netns.Set(origNS); err != nil {
		return fmt.Errorf("failed to return to original netns: %w", err)
	}

	log.Printf("✅ Created veth pair: host=%s ↔ container=%s (netns=%s)",
		hostIf, ifName, netnsPath)

	return nil
}
