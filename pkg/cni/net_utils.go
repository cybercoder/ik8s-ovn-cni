package cni

import (
	"fmt"
	"os"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// setupVethPair creates a veth pair and moves one end into the given netns.
func setupVethPair(containerIfName, hostIfName, netnsPath string) (netlink.Link, netlink.Link, error) {
	// Create veth pair
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostIfName},
		PeerName:  containerIfName,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, nil, fmt.Errorf("failed to add veth: %w", err)
	}

	host, err := netlink.LinkByName(hostIfName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup host veth: %w", err)
	}

	peer, err := netlink.LinkByName(containerIfName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup peer veth: %w", err)
	}

	// Move container end to its network namespace
	fd, err := getNetnsFd(netnsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open netns: %w", err)
	}
	defer fd.Close()

	if err := netlink.LinkSetNsFd(peer, int(fd.Fd())); err != nil {
		return nil, nil, fmt.Errorf("failed to move veth into netns: %w", err)
	}

	return host, peer, nil
}

// getNetnsFd opens the network namespace file descriptor
func getNetnsFd(netnsPath string) (*os.File, error) {
	return os.Open(netnsPath)
}

// withNetNS runs a function inside a target network namespace
func withNetNS(netnsPath string, fn func() error) error {
	fd, err := getNetnsFd(netnsPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	ns, err := netns.Get()
	if err != nil {
		return err
	}
	defer ns.Close()

	targetNS, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return err
	}
	defer targetNS.Close()

	if err := netns.Set(targetNS); err != nil {
		return err
	}
	defer netns.Set(ns)

	return fn()
}
