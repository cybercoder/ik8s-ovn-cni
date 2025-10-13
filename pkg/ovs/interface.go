package ovs

const OvsInterfaceTable = "Interface"

type Interface struct {
	UUID    string            `ovsdb:"_uuid"`
	Name    string            `ovsdb:"name"`
	Type    string            `ovsdb:"type"` // "internal", "dpdk", "vxlan", etc.
	Options map[string]string `ovsdb:"options"`
	MAC     string            `ovsdb:"mac_in_use"`
}
