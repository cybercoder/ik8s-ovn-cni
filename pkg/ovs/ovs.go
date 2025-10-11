package ovs

import (
	"context"

	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

func (c *Client) AddPort(bridge, port string) error {
	ops := []ovsdb.Operation{{
		Op:    "add-port",
		Table: "Bridge",
		Where: []ovsdb.Condition{{Column: "name", Function: ovsdb.ConditionEqual, Value: bridge}},
		Mutations: []ovsdb.Mutation{{
			Column:  "ports",
			Mutator: ovsdb.MutateOperationInsert,
			Value:   ovsdb.OvsSet{GoSet: []any{port}},
		}},
	}}
	_, err := c.ovsClient.Transact(context.Background(), ops...)
	return err
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
