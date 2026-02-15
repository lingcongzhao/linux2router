package models

type IPRule struct {
	Priority int    `json:"priority"`
	Selector string `json:"selector"`
	Action   string `json:"action"`
	Table    string `json:"table"`
	From     string `json:"from"`
	To       string `json:"to"`
	FWMark   string `json:"fwmark"`
	IIF      string `json:"iif"`
	OIF      string `json:"oif"`
	Not      bool   `json:"not"`
}

type IPRuleInput struct {
	Priority int    `json:"priority"`
	From     string `json:"from"`
	To       string `json:"to"`
	FWMark   string `json:"fwmark"`
	IIF      string `json:"iif"`
	OIF      string `json:"oif"`
	Table    string `json:"table"`
	Not      bool   `json:"not"`
}
