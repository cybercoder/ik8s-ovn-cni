package net_utils

import (
	"fmt"

	"github.com/samber/lo"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func GetVethList(netnsPath string) ([]netlink.Link, error) {
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
		return l.Type() == "veth"
	})

	if len(veths) == 0 {
		return nil, fmt.Errorf("no tap link in ns %s: %v", ns.String(), err)
	}
	return veths, nil
}
