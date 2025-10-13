package ovs

import (
	"context"
	"fmt"
	"log"

	"github.com/cybercoder/ik8s-ovn-cni/pkg/net_utils"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

func (c *Client) AddPortToBridge(bridgeName, portName string, portConfig *PortConfig) error {
	ctx := context.Background()

	interfaceUUID := net_utils.GenerateUUID()
	portUUID := net_utils.GenerateUUID()

	// Create interface and port
	iface := &Interface{
		UUID: interfaceUUID,
		Name: portName,
		Type: portConfig.InterfaceType,
	}

	if len(portConfig.InterfaceOptions) > 0 {
		iface.Options = portConfig.InterfaceOptions
	}

	port := &Port{
		UUID:       portUUID,
		Name:       portName,
		Interfaces: []string{interfaceUUID},
	}

	if len(portConfig.ExternalIDs) > 0 {
		port.ExternalIDs = portConfig.ExternalIDs
	}

	// Single transaction with all operations
	var operations []ovsdb.Operation

	// Create interface
	ifaceOp, err := c.ovsClient.Create(iface)
	if err != nil {
		return err
	}
	operations = append(operations, ifaceOp...)

	// Create port
	portOp, err := c.ovsClient.Create(port)
	if err != nil {
		return err
	}
	operations = append(operations, portOp...)

	// Add port to bridge using mutate with insert
	mutateOp, err := c.ovsClient.Where(&Bridge{Name: bridgeName}).Mutate(&Bridge{}, model.Mutation{
		Field:   "ports",
		Mutator: "insert",
		Value:   []string{portUUID},
	})
	if err != nil {
		return err
	}
	operations = append(operations, mutateOp...)

	// Execute all in one transaction
	results, err := c.ovsClient.Transact(ctx, operations...)
	if err != nil {
		return fmt.Errorf("transaction failed: %v", err)
	}

	for i, result := range results {
		if result.Error != "" {
			return fmt.Errorf("operation %d failed: %s", i, result.Error)
		}
	}

	return nil
}

func (c *Client) AddPort(bridgeName, portName, ifaceType string) error {
	ctx := context.Background()

	// 1️⃣ Find the bridge
	bridge := &Bridge{Name: bridgeName}
	if err := c.ovsClient.Get(ctx, bridge); err != nil {
		return fmt.Errorf("failed to get bridge %q: %v", bridgeName, err)
	}

	// 2️⃣ Create Interface row
	iface := &Interface{
		Name: portName,
		Type: ifaceType, // "system" for veth, "internal" if OVS creates it
	}
	if _, err := c.ovsClient.Create(iface); err != nil {
		return fmt.Errorf("failed to create interface: %v", err)
	}

	// 3️⃣ Create Port row, referencing Interface
	port := &Port{
		Name:       portName,
		Interfaces: []string{iface.Name},
	}
	_, err := c.ovsClient.Create(port)
	if err != nil {
		return fmt.Errorf("failed to create port: %v", err)
	}

	log.Printf("port uuid: %s", port.UUID)
	// 4️⃣ Mutate Bridge to include the new port
	mutations := []model.Mutation{
		{
			Field:   &bridge.Ports,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   port,
		},
	}

	if _, err := c.ovsClient.Where(bridge).Mutate(bridge, mutations...); err != nil {
		return fmt.Errorf("failed to mutate bridge: %v", err)
	}

	log.Printf("✅ Added port %s to bridge %s (type=%s)", portName, bridgeName, ifaceType)
	return nil
}

// br := &Bridge{Name: bridgeName}
//        if err := c.ovsClient.Get(ctx, br); err != nil {
//                return fmt.Errorf("failed to get bridge %q: %v", bridgeName, err)
//        }

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
