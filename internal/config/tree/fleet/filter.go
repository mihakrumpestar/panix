package fleet

import (
	"errors"

	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
)

var ErrNoFlakesAfterFilter = errors.New("no flakes in fleet after filtering")

func (f *Fleet) Filter() error {
	f.Flakes.DeleteFunc(func(name string, flake *flake.Flake) bool {
		if flake == nil || flake.Disabled {
			return true
		}

		flake.Filter()

		return flake.Configurations.Len() == 0
	})

	if f.Flakes.Len() == 0 {
		return ErrNoFlakesAfterFilter
	}

	return nil
}
