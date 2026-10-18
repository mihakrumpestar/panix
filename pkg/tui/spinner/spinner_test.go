package spinner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClampInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		interval time.Duration
		want     time.Duration
	}{
		{"negative", -time.Second, MinInterval},
		{"nanoseconds", 120 * time.Nanosecond, MinInterval},
		{"below minimum", 5 * time.Millisecond, MinInterval},
		{"minimum", MinInterval, MinInterval},
		{"inside bounds", 120 * time.Millisecond, 120 * time.Millisecond},
		{"maximum", MaxInterval, MaxInterval},
		{"above maximum", 2 * time.Minute, MaxInterval},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ClampInterval(tt.interval))
		})
	}
}

func TestSpinnerApplyThemeClampsInterval(t *testing.T) {
	t.Parallel()

	spinner := New(nil, 120*time.Nanosecond)

	assert.Equal(t, MinInterval, spinner.Interval(), "the scheduler must never see a hot-loop interval")

	spinner.ApplyTheme(nil, -time.Second)

	assert.Equal(t, MinInterval, spinner.Interval())
}
