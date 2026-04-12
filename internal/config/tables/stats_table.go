package tables

import "github.com/mihakrumpestar/panix/internal/config/attributes"

type StatsTable struct {
	Rows            []MachineRow       `json:"rows"`
	MachineXpaths   []attributes.Xpath `json:"machine_xpaths"`
	SelectedMachine int                `json:"selected_machine"`

	CacheHash         uint64 `json:"-"`
	CacheTableContent string `json:"-"`
}

type MachineRow struct {
	Xpath        string `json:"xpath"`
	FlakeName    string `json:"flake_name"`
	ConfigName   string `json:"config_name"`
	MachineName  string `json:"machine_name"`
	Status       string `json:"status"`
	Phase        string `json:"phase"`
	Architecture string `json:"architecture,omitempty"`
	Generation   string `json:"generation,omitempty"`
	Date         string `json:"date,omitempty"`
	Nixos        string `json:"nixos,omitempty"`
	Kernel       string `json:"kernel,omitempty"`
}
