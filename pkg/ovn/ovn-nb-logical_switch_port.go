package ovn

import (
	"context"
	"fmt"

	"github.com/cybercoder/ik8s-ovn-cni/pkg/ovn/ovnnb"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

// CreateLogicalPort creates a new logical port and attaches it to a logical switch
func (c *Client) CreateLogicalPort(ctx context.Context, lsName, portName, mac, ip string) error {
	// Build addresses field
	address := "dynamic"
	if mac != "" && ip != "" {
		address = fmt.Sprintf("%s %s", mac, ip)
	}

	lsp := map[string]any{
		"name":      portName,
		"addresses": ovsdb.OvsSet{GoSet: []any{address}},
	}

	// Insert Logical_Switch_Port
	insertOp := ovsdb.Operation{
		Op:       "insert",
		Table:    "Logical_Switch_Port",
		Row:      lsp,
		UUIDName: "lsp",
	}

	// Mutate Logical_Switch to include this port
	mutateOp := ovsdb.Operation{
		Op:    "mutate",
		Table: "Logical_Switch",
		Where: []ovsdb.Condition{
			{
				Column:   "name",
				Function: ovsdb.ConditionEqual,
				Value:    lsName,
			},
		},
		Mutations: []ovsdb.Mutation{
			{
				Column:  "ports",
				Mutator: ovsdb.MutateOperationInsert,
				Value:   ovsdb.OvsSet{GoSet: []any{ovsdb.UUID{GoUUID: "lsp"}}},
			},
		},
	}

	ops := []ovsdb.Operation{insertOp, mutateOp}

	reply, err := c.nbClient.Transact(ctx, ops...)
	if err != nil {
		return fmt.Errorf("transaction error: %w", err)
	}

	for _, r := range reply {
		if r.Error != "" {
			return fmt.Errorf("OVN NB error: %s", r.Error)
		}
	}

	return nil
}

// DeleteLogicalPort removes a logical port from OVN NB
func (c *Client) DeleteLogicalPort(ctx context.Context, portName string) error {
	delOp := ovsdb.Operation{
		Op:    "delete",
		Table: "Logical_Switch_Port",
		Where: []ovsdb.Condition{
			{
				Column:   "name",
				Function: ovsdb.ConditionEqual,
				Value:    portName,
			},
		},
	}

	_, err := c.nbClient.Transact(ctx, []ovsdb.Operation{delOp}...)
	if err != nil {
		return fmt.Errorf("failed to delete port %s: %w", portName, err)
	}

	return nil
}

// ListLogicalPorts returns all ports on a given logical switch
func (c *Client) ListLogicalPorts(ctx context.Context, lsName string) ([]string, error) {
	// Get Logical_Switch by name
	lsObj := []ovnnb.LogicalSwitch{}
	err := c.nbClient.Where(func(ls *ovnnb.LogicalSwitch) bool {
		return ls.Name == lsName
	}).List(ctx, &lsObj)
	if err != nil {
		return nil, fmt.Errorf("failed to list logical switch: %w", err)
	}

	if len(lsObj) == 0 {
		return nil, fmt.Errorf("logical switch %s not found", lsName)
	}

	return lsObj[0].Ports, nil
}
