package net_utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func RequestAssignmentFromIPAM(reqBody IpAssignmentRequestBody) (*IpAssignmentResponseBody, error) {
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post("http://172.16.35.12:8000/apis/ovn.ik8s.ir/v1alpha1/assignip", "application/json", bytes.NewBuffer(jsonData))
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

func MoveIf2NS(ifName, netnsPath string) error {
	netNs, err := netns.GetFromPath(netnsPath)
	if err != nil {
		return fmt.Errorf("failed to get target netns: %v", err)
	}
	var If netlink.Link
	for range make([]struct{}, 50) { // try ~50 times
		If, err = netlink.LinkByName(ifName)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to veth: %v", err)
	}
	if err := netlink.LinkSetUp(If); err != nil {
		return fmt.Errorf("failed to bring If %s up: %v", If.Attrs().Name, err)
	}
	if err := netlink.LinkSetNsFd(If, int(netNs)); err != nil {
		return fmt.Errorf("failed to move peer veth to target ns: %v", err)
	}
	return nil
}
