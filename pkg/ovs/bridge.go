package ovs

const OvsBridgeTable = "Bridge"

type Bridge struct {
	UUID        string            `ovsdb:"_uuid"`
	Name        string            `ovsdb:"name"`
	Ports       []string          `ovsdb:"ports"` // UUIDs of ports
	ExternalIDs map[string]string `ovsdb:"external_ids"`
}
