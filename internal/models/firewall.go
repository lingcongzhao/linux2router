package models

type FirewallTable string
type FirewallChain string

const (
	TableFilter FirewallTable = "filter"
	TableNAT    FirewallTable = "nat"
	TableMangle FirewallTable = "mangle"
	TableRaw    FirewallTable = "raw"
)

const (
	ChainInput       FirewallChain = "INPUT"
	ChainOutput      FirewallChain = "OUTPUT"
	ChainForward     FirewallChain = "FORWARD"
	ChainPrerouting  FirewallChain = "PREROUTING"
	ChainPostrouting FirewallChain = "POSTROUTING"
)

type FirewallRule struct {
	Num         int    `json:"num"`
	Target      string `json:"target"`
	Protocol    string `json:"protocol"`
	Opt         string `json:"opt"`
	In          string `json:"in"`
	Out         string `json:"out"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Extra       string `json:"extra"`
	Comment     string `json:"comment"`
	Packets     uint64 `json:"packets"`
	Bytes       uint64 `json:"bytes"`
}

type ChainInfo struct {
	Name    string         `json:"name"`
	Policy  string         `json:"policy"`
	Packets uint64         `json:"packets"`
	Bytes   uint64         `json:"bytes"`
	Rules   []FirewallRule `json:"rules"`
}

type FirewallRuleInput struct {
	Table       string `json:"table"`
	Chain       string `json:"chain"`
	Position    int    `json:"position,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	InInterface string `json:"in_interface,omitempty"`
	OutInterface string `json:"out_interface,omitempty"`
	DPort       string `json:"dport,omitempty"`
	SPort       string `json:"sport,omitempty"`
	Target      string `json:"target"`
	ToDestination string `json:"to_destination,omitempty"`
	ToSource    string `json:"to_source,omitempty"`
	State       string `json:"state,omitempty"`
	Comment     string `json:"comment,omitempty"`
	IPSet       string `json:"ipset,omitempty"`       // IPSet name to match
	IPSetFlags  string `json:"ipset_flags,omitempty"` // src, dst, or src,dst
	Mark        string `json:"mark,omitempty"`        // Mark value for MARK target
}
