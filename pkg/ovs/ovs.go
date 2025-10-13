package ovs

import (
	"context"
	"fmt"
	"log"

	"github.com/cybercoder/ik8s-ovn-cni/pkg/net_utils"
	ovsModel "github.com/cybercoder/ik8s-ovn-cni/pkg/ovs/models"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

func (c *Client) AddPort(bridgeName, portName, ifaceType string) error {
	ctx := context.Background()
	interfaceUUID := net_utils.GenerateUUID()
	portUUID := net_utils.GenerateUUID()
	// 1️⃣ Find the bridge
	bridge := &ovsModel.Bridge{Name: bridgeName}
	if err := c.ovsClient.Get(ctx, bridge); err != nil {
		return fmt.Errorf("failed to get bridge %q: %v", bridgeName, err)
	}

	// 2️⃣ Create Interface row
	iface := &ovsModel.Interface{
		UUID: interfaceUUID,
		Name: portName,
		Type: ifaceType, // "system" for veth, "internal" if OVS creates it
	}
	if _, err := c.ovsClient.Create(iface); err != nil {
		return fmt.Errorf("failed to create interface: %v", err)
	}

	// 3️⃣ Create Port row, referencing Interface
	port := &ovsModel.Port{
		UUID:       portUUID,
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
