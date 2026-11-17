package tracker

import "testing"

func TestNextVolumeFadeStepMovesTowardTargetAndStops(t *testing.T) {
	tests := []struct {
		name      string
		current   float64
		target    float64
		increment float64
		want      float64
		active    bool
	}{
		{name: "fade in", current: 0, target: 0.8, increment: 0.4, want: 0.2, active: true},
		{name: "finish fade in", current: 0.6, target: 0.8, increment: 0.4, want: 0.8, active: false},
		{name: "fade out", current: 1, target: 0.2, increment: 0.1, want: 0.95, active: true},
		{name: "zero volume", current: 0.5, target: 0, increment: 0, want: 0, active: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, active := nextVolumeFadeStep(tt.current, tt.target, tt.increment)
			if got != tt.want || active != tt.active {
				t.Fatalf("nextVolumeFadeStep() = (%v, %v), want (%v, %v)", got, active, tt.want, tt.active)
			}
		})
	}
}

func TestStartVolumeFadeDoesNotScheduleWithoutPlayingPlayer(t *testing.T) {
	m := &Model{paused: true, volume: 0.8}
	if cmd := m.StartVolumeFade(); cmd != nil {
		t.Fatal("a paused model must not schedule volume fade ticks")
	}
}
