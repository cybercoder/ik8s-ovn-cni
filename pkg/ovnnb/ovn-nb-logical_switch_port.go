package ovnnb

import (
	"context"
	"fmt"
	"log"

	models "github.com/cybercoder/ik8s-ovn-cni/pkg/ovnnb/models"
	"github.com/google/uuid"
	"github.com/ovn-kubernetes/libovsdb/model"
	"github.com/ovn-kubernetes/libovsdb/ovsdb"
)

// CreateLogicalPort creates a new logical port and attaches it to a logical switch
func (c *Client) CreateLogicalPort(lsName, lspName string) error {
	ctx := context.Background()
	lspUUID := uuid.New().String()
	ls := &models.LogicalSwitch{Name: lsName}
	if err := c.nbClient.Get(ctx, ls); err != nil {
		return fmt.Errorf("failed to get logical switch %s: %v", ls.Name, err)
	}
	lsp := &models.LogicalSwitchPort{
		UUID: lspUUID,
		Name: lspName,
	}
	lspOp, err := c.nbClient.Create(lsp)
	if err != nil {
		return fmt.Errorf("failed to create logical port %s: %v", lsp.Name, err)
	}
	mutations := []model.Mutation{
		{
			Field:   &ls.Ports,
			Mutator: ovsdb.MutateOperationInsert,
			Value:   []string{lsp.UUID},
		},
	}
	mutateOps, err := c.nbClient.Where(ls).Mutate(ls, mutations...)
	if err != nil {
		return fmt.Errorf("failed to prepare mutation: %v", err)
	}
	ops := append(lspOp, mutateOps...)
	reply, err := c.nbClient.Transact(ctx, ops...)
	if err != nil {
		return fmt.Errorf("transaction failed: %v", err)
	}

	for i, r := range reply {
		if r.Error != "" {
			log.Printf("OVSNB error: %d %s (%s)", i, r.Error, r.Details)
		}
	}
	log.Printf("✅ Added logicalport %s to logicalswitch %s ", lspName, lsName)
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
	lsObj := []models.LogicalSwitch{}
	err := c.nbClient.Where(func(ls *models.LogicalSwitch) bool {
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
