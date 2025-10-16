package ovs

import (
	"context"
	"fmt"
	"log"

	ovsModel "github.com/cybercoder/ik8s-ovn-cni/pkg/ovs/models"
	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

func (c *Client) AddManagedTapPort(bridgeName, portName, mac string) error {
	ctx := context.Background()

	ifaceUUID := uuid.New()
	portUUID := uuid.New()

	bridge := &ovsModel.Bridge{Name: bridgeName}
	if err := c.ovsClient.Get(ctx, bridge); err != nil {
		return fmt.Errorf("failed to get bridge %q: %v", bridgeName, err)
	}

	iface := &ovsModel.Interface{
		UUID: ifaceUUID.String(),
		Name: portName,
		Type: "managed_tap",
		MAC:  &mac,
		ExternalIDs: map[string]string{
			"iface-id": portName,
			// optional flag: mark as managed for ovn-controller visibility
			"ovn-installed": "true",
		},
	}

	port := &ovsModel.Port{
		UUID:       portUUID.String(),
		Name:       portName,
		Interfaces: []string{iface.UUID},
	}

	ifaceOps, err := c.ovsClient.Create(iface)
	if err != nil {
		return fmt.Errorf("failed to create interface: %v", err)
	}

	portOps, err := c.ovsClient.Create(port)
	if err != nil {
		return fmt.Errorf("failed to create port: %v", err)
	}

	mutations := []model.Mutation{{
		Field:   &bridge.Ports,
		Mutator: ovsdb.MutateOperationInsert,
		Value:   []string{port.UUID},
	}}

	mutateOps, err := c.ovsClient.Where(bridge).Mutate(bridge, mutations...)
	if err != nil {
		return fmt.Errorf("failed to prepare bridge mutation: %v", err)
	}

	ops := append(ifaceOps, append(portOps, mutateOps...)...)
	reply, err := c.ovsClient.Transact(ctx, ops...)
	if err != nil {
		return fmt.Errorf("OVSDB transaction failed: %v", err)
	}

	for i, r := range reply {
		if r.Error != "" {
			log.Printf("OVSDB error %d: %s (%s)", i, r.Error, r.Details)
		}
	}

	log.Printf("✅ Created managed_tap port %q on bridge %q (MAC=%s)", portName, bridgeName, mac)
	return nil
}

func (c *Client) AddPort(bridgeName, portName, ifaceType, hostmac string) error {
	ctx := context.Background()
	ifaceUUID := uuid.New()
	portUUID := uuid.New()
	bridge := &ovsModel.Bridge{Name: bridgeName}
	if err := c.ovsClient.Get(ctx, bridge); err != nil {
		return fmt.Errorf("failed to get bridge %q: %v", bridgeName, err)
	}

	iface := &ovsModel.Interface{
		UUID: ifaceUUID.String(),
		Name: portName,
		Type: ifaceType, // "system" for veth, "internal" if OVS creates it
		MAC:  &hostmac,
		ExternalIDs: map[string]string{
			"iface-id": portName,
		},
	}

	ifaceOp, err := c.ovsClient.Create(iface)
	if err != nil {
		return fmt.Errorf("failed to create interface: %v", err)
	}

	port := &ovsModel.Port{
		UUID:       portUUID.String(),
		Name:       portName,
		Interfaces: []string{iface.UUID},
	}
	portOp, err := c.ovsClient.Create(port)
	if err != nil {
		return fmt.Errorf("failed to create port: %v", err)
	}

	mutations := []model.Mutation{
		{
			Field:   &bridge.Ports,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{port.UUID},
		},
	}
	mutateOps, err := c.ovsClient.Where(bridge).Mutate(bridge, mutations...)
	if err != nil {
		return fmt.Errorf("failed to prepare mutation: %v", err)
	}
	ops := append(ifaceOp, append(portOp, mutateOps...)...)
	reply, err := c.ovsClient.Transact(ctx, ops...)
	if err != nil {
		return fmt.Errorf("transaction failed: %v", err)
	}

	for i, r := range reply {
		if r.Error != "" {
			log.Printf("OVSDB error: %d %s (%s)", i, r.Error, r.Details)
		}
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

func (c *Client) GetPortMAC(portName string) (string, error) {
	ctx := context.Background()
	iface := &ovsModel.Interface{Name: portName}
	if err := c.ovsClient.Get(ctx, iface); err != nil {
		return "", fmt.Errorf("failed to get interface %q: %w", portName, err)
	}
	if iface.MACInUse == nil {
		return "", fmt.Errorf("no mac_in_use found")
	}
	return *iface.MACInUse, nil
}
