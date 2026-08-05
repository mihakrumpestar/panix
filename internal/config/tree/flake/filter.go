package flake

import (
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
)

func (f *Flake) Filter(flags flags.Flags) {
	f.Installables.ForEach(func(_ string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable]) bool {
		if attrMap == nil {
			return true
		}

		attrMap.DeleteFunc(func(name string, installable *installable.Installable) bool {
			if installable == nil || installable.Disabled {
				return true
			}

			installable.Filter(flags)

			return installable.Machines.Len() == 0
		})

		return true
	})

	// Clean up empty inner maps
	f.Installables.DeleteFunc(func(_ string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable]) bool {
		return attrMap == nil || attrMap.Len() == 0
	})
}
