package ovn

import (
	"context"
	"log"

	"github.com/cybercoder/ik8s-ovn-cni/pkg/ovn/ovnnb"
	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/model"
)

type Client struct {
	nbClient client.Client
}

func CreateClient(nbEndpoint string) *Client {
	// Define database model
	dbModel, err := model.NewClientDBModel("OVN_Northbound", map[string]model.Model{
		"Logical_Switch_Port": &ovnnb.LogicalSwitchPort{},
		// Add other table mappings
	})
	if err != nil {
		log.Fatalf("%v", err)
	}

	// Create client with connection options
	ovsClient, err := client.NewOVSDBClient(dbModel, client.WithEndpoint(nbEndpoint))
	if err != nil {
		log.Fatalf("%v", err)
	}

	// Establish connection
	ctx := context.Background()
	err = ovsClient.Connect(ctx)
	if err != nil {
		log.Fatalf("%v", err)
	}

	// Start monitoring for cache updates
	_, err = ovsClient.MonitorAll(ctx)
	if err != nil {
		log.Fatalf("%v", err)
	}

	return &Client{nbClient: ovsClient}
}

func (c *Client) Close() {
	c.nbClient.Disconnect()
}
