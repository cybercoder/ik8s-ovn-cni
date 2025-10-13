package ovs

const OvsPortTable = "Port"

type Port struct {
	UUID       string   `ovsdb:"_uuid"`
	Name       string   `ovsdb:"name"`
	Interfaces []string `ovsdb:"interfaces"` // UUIDs of interfaces

	ExternalIDs map[string]string `ovsdb:"external_ids"`
}

type PortConfig struct {
	InterfaceType    string
	InterfaceOptions map[string]string
	ExternalIDs      map[string]string // Additional metadata
}
