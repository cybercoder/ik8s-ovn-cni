package ovs

import (
	"context"
	"log"

	"github.com/ovn-kubernetes/libovsdb/client"
	"github.com/ovn-kubernetes/libovsdb/model"
)

type Client struct {
	ovsClient client.Client
}

func CreateOVSclient() *Client {
	dbModel, err := model.NewClientDBModel("Open_vSwitch", map[string]model.Model{
		OvsBridgeTable:    &Bridge{},
		OvsPortTable:      &Port{},
		OvsInterfaceTable: &Interface{},
	})
	if err != nil {
		log.Fatalf("failed to create DB model: %v", err)
	}

	ovsClient, err := client.NewOVSDBClient(
		dbModel,
		client.WithEndpoint("unix:/var/run/openvswitch/db.sock"),
	)
	if err != nil {
		log.Fatalf("failed to create OVS client: %v", err)
	}

	ctx := context.Background()
	if err := ovsClient.Connect(ctx); err != nil {
		log.Fatalf("failed to connect to OVSDB: %v", err)
	}

	return &Client{ovsClient: ovsClient}
}

func (c *Client) Close() {
	c.ovsClient.Disconnect()
}
