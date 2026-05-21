package phasehandler

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/executioner"
)

type Handler interface {
	RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error
}

type Skipper interface {
	ShouldSkip(fleetLeaf *fleet.FleetLeaf) bool
}

type HandlerFunc func(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error

func (f HandlerFunc) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	return f(exc, fleetLeaf)
}
