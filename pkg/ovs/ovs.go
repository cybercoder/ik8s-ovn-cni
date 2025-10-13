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

func (c *Client) AddPort(bridgeName, portName, ifaceType string) error {
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
