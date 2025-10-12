package net_utils

import (
	"log"
	"net"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func CreateVethPair(hostIfName, contIfName, netnsPath string) error {
	// Lock thread (required for netns operations)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save current (host) namespace
	hostNs, err := netns.Get()
	if err != nil {
		return err
	}
	defer hostNs.Close()

	// Open target (container/pod) namespace
	targetNs, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return err
	}
	defer targetNs.Close()

	// Create veth pair
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:  hostIfName,
			MTU:   1500,
			Flags: net.FlagUp,
		},
		PeerName: contIfName,
	}

	if err := netlink.LinkAdd(veth); err != nil {
		return err
	}

	// Get host side of veth
	hostLink, err := netlink.LinkByName(hostIfName)
	if err != nil {
		return err
	}

	// Bring up host side
	if err := netlink.LinkSetUp(hostLink); err != nil {
		return err
	}

	// Get container side
	contLink, err := netlink.LinkByName(contIfName)
	if err != nil {
		return err
	}

	// Move container side into target namespace
	if err := netlink.LinkSetNsFd(contLink, int(targetNs)); err != nil {
		return err
	}

	// Now enter the container namespace to configure the interface
	if err := netns.Set(targetNs); err != nil {
		return err
	}
	defer netns.Set(hostNs) // return to host ns

	// Re-fetch the link (now inside netns)
	contLink, err = netlink.LinkByName(contIfName)
	if err != nil {
		return err
	}

	// Rename it to the desired name (e.g., eth1)
	if err := netlink.LinkSetName(contLink, contIfName); err != nil {
		return err
	}

	// Bring it up
	if err := netlink.LinkSetUp(contLink); err != nil {
		return err
	}

	log.Printf("✅ Created veth: host=%s, container=%s", hostIfName, contIfName)
	return nil
}
