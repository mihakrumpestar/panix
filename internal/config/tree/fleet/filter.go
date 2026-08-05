package fleet

import (
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/pkg/errors"
)

var ErrNoFlakesAfterFilter = errors.New("no flakes in fleet after filtering")

func (f *Fleet) Filter(flags flags.Flags) error {
	f.Flakes.DeleteFunc(func(name string, flake *flake.Flake) bool {
		if flake == nil || flake.Disabled {
			return true
		}

		flake.Filter(flags)

		return flake.Installables.Len() == 0
	})

	if f.Flakes.Len() == 0 {
		return ErrNoFlakesAfterFilter
	}

	return nil
}
