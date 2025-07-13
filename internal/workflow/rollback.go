package workflow

import (
	"fmt"
	"sync"

	"github.com/mihakrumpestar/panix/internal/config"
)

func (w *WorkflowExecutor) executeMachineRollback(flakeName, configName, machineName string, machine *config.Machine) error {
	return fmt.Errorf("rollback not implemented for machine %s", machineName)
}

func (w *WorkflowExecutor) rollbackMachines(machines []machineInfo) {
	var wg sync.WaitGroup
	for _, mi := range machines {
		wg.Add(1)
		go func(mi machineInfo) {
			defer wg.Done()
			err := w.executeMachineRollback(mi.flakeName, mi.configName, mi.machineName, mi.machine)
			if err != nil {
				fmt.Printf("Warning: rollback failed for machine %s: %v\n", mi.machineName, err)
			}
		}(mi)
	}
	wg.Wait()
}
