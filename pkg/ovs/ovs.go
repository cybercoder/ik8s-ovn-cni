package ovs

import (
	"context"
	"fmt"
	"log"

	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

func (c *Client) AddPort(bridgeName, portName string) error {
	ctx := context.Background()

	// 1. Get the bridge
	br := &Bridge{Name: bridgeName}
	if err := c.ovsClient.Get(ctx, br); err != nil {
		return fmt.Errorf("failed to get bridge %q: %v", bridgeName, err)
	}

	// 2. Define interface and port
	intf := &Interface{
		Name: portName,
		Type: "system",
	}
	port := &Port{
		Name:       portName,
		Interfaces: []string{portName},
	}

	opsIntf, err := c.ovsClient.Create(intf)
	if err != nil {
		return fmt.Errorf("Create(interface) failed: %v", err)
	}
	opsPort, err := c.ovsClient.Create(port)
	if err != nil {
		return fmt.Errorf("Create(port) failed: %v", err)
	}

	// 3. Build mutation to add the port name to bridge.ports
	mut := ovsdb.NewMutation("ports", ovsdb.MutateOperationInsert, []string{portName})
	cond := ovsdb.NewCondition("name", ovsdb.ConditionEqual, bridgeName)

	mutateOp := ovsdb.Operation{
		Op:        ovsdb.OperationMutate,
		Table:     "Bridge",
		Mutations: []ovsdb.Mutation{*mut},  // dereference
		Where:     []ovsdb.Condition{cond}, // dereference
	}

	// 4. Build full operation list
	// opsIntf and opsPort are slices of Operation
	allOps := make([]ovsdb.Operation, 0, len(opsIntf)+len(opsPort)+1)
	allOps = append(allOps, opsIntf...)
	allOps = append(allOps, opsPort...)
	allOps = append(allOps, mutateOp)

	// 5. Transact
	reply, err := c.ovsClient.Transact(ctx, allOps...)
	if err != nil {
		return fmt.Errorf("transaction error: %v", err)
	}
	for _, r := range reply {
		if r.Error != "" {
			return fmt.Errorf("OVSDB op error: %s", r.Error)
		}
	}

	log.Printf("Added port %s to bridge %s", portName, bridgeName)
	return nil
}

func (c *Client) SetInterfaceExternalIDs(ifName string, ids map[string]string) error {
	row := map[string]any{"external_ids": ids}
	op := ovsdb.Operation{
		Op:    "update",
		Table: "Interface",
		Where: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: ifName}},
		Row:   row,
	}
	_, err := c.ovsClient.Transact(context.Background(), []ovsdb.Operation{op}...)
	return err
}
