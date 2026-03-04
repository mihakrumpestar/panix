package executioner

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
)

func (ex *Executioner) ExecuteHooks(hooks []config_attributes.PostBootstrapHookCommand, hookType string) error {
	for i, hook := range hooks {
		switch hook {
		case config_attributes.PostBootstrapHookWaitForOnline:
			activeSSH := ex.machine.MetaInspect.ActiveSSH
			err := activeSSH.WaitForReconnect(ex, fmt.Sprintf("waiting for %s to be online", hookType), fmt.Sprintf("%s did not come online", hookType))
			if err != nil {
				return err
			}
		case config_attributes.PostBootstrapHookWaitForOffline:
			activeSSH := ex.machine.MetaInspect.ActiveSSH
			err := activeSSH.WaitForDisconnect(ex, fmt.Sprintf("waiting for %s to go offline", hookType))
			if err != nil {
				return err
			}
		default:
			err := ex.Exec(
				fmt.Sprintf("%s %d", hookType, i+1),
				fmt.Sprintf("running %s: %s", hookType, hook),
				fmt.Sprintf("%s failed", hookType),
				[]string{string(hook)},
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
