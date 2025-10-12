package net_utils

import (
	"fmt"
	"log"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func CreateStableVeth(name, ifName, netnsPath string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hostIf := fmt.Sprintf("veth-%s", name)
	peerIf := "tmppeer0"

	ns, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return err
	}
	defer ns.Close()

	origNS, _ := netns.Get()
	defer netns.Set(origNS)

	// Create veth in host
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostIf, MTU: 1500},
		PeerName:  peerIf,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return err
	}

	// Move peer to container netns
	peerLink, _ := netlink.LinkByName(peerIf)
	if err := netlink.LinkSetNsFd(peerLink, int(ns)); err != nil {
		return err
	}

	// Configure peer inside container netns
	netns.Set(ns)
	peerLink, _ = netlink.LinkByName(peerIf)
	netlink.LinkSetName(peerLink, ifName) // final container name
	netlink.LinkSetUp(peerLink)
	netns.Set(origNS)

	// Bring up host side
	hostLink, _ := netlink.LinkByName(hostIf)
	netlink.LinkSetUp(hostLink)
	log.Printf("✅ Created veth: host=%s, container=%s", name, ifName)
	return nil
}
