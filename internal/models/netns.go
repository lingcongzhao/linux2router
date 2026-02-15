package models

// NetworkNamespace represents a Linux network namespace
type NetworkNamespace struct {
	Name       string   `json:"name"`
	ID         string   `json:"id,omitempty"`    // inode number from /var/run/netns/
	Interfaces []string `json:"interfaces"`      // interfaces in this namespace
	Persistent bool     `json:"persistent"`      // has config file
}

// NetnsCreateInput represents input for creating a namespace
type NetnsCreateInput struct {
	Name string `json:"name"`
}

// VethPairInput represents input for creating a veth pair
type VethPairInput struct {
	Name1     string `json:"name1"`               // first interface name (stays in default ns)
	Name2     string `json:"name2"`               // second interface name
	Namespace string `json:"namespace"`           // namespace for name2
	Address1  string `json:"address1,omitempty"`  // optional IP for name1 (CIDR)
	Address2  string `json:"address2,omitempty"`  // optional IP for name2 (CIDR)
}

// MoveInterfaceInput represents input for moving an interface to a namespace
type MoveInterfaceInput struct {
	Interface string `json:"interface"`
	Namespace string `json:"namespace"` // empty = default namespace
}

// NetnsInterfaceInfo represents interface info within a namespace
type NetnsInterfaceInfo struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	State     string   `json:"state"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	MTU       int      `json:"mtu,omitempty"`
}
