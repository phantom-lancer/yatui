package mainpage

import (
	"testing"

	"github.com/phantom-lancer/yatui/config"
)

func TestDiscordKeepaliveIsNotScheduledWhenDisabled(t *testing.T) {
	previous := config.Current.Discord
	t.Cleanup(func() { config.Current.Discord = previous })
	config.Current.Discord = &config.Discord{Enabled: false}

	m := &Model{}
	if cmd := m.startDiscordTick(); cmd != nil {
		t.Fatal("disabled Discord RPC must not schedule a timer")
	}
	if m.discordTickRunning {
		t.Fatal("disabled Discord RPC marked its timer as running")
	}
}

func TestStoppingDiscordKeepaliveInvalidatesPendingTick(t *testing.T) {
	previous := config.Current.Discord
	t.Cleanup(func() { config.Current.Discord = previous })
	config.Current.Discord = &config.Discord{Enabled: false}

	m := &Model{discordTickRunning: true, discordTickGeneration: 4}
	m.stopDiscordTick()

	if m.discordTickRunning {
		t.Fatal("Discord keepalive still marked as running")
	}
	if m.discordTickGeneration != 5 {
		t.Fatalf("generation = %d, want 5", m.discordTickGeneration)
	}
}

func TestDisabledPendingDiscordTickCanBeRestarted(t *testing.T) {
	previous := config.Current.Discord
	t.Cleanup(func() { config.Current.Discord = previous })
	config.Current.Discord = &config.Discord{Enabled: false}

	m := &Model{discordTickRunning: true, discordTickGeneration: 9}
	_, cmd := m.Update(discordTickMsg{generation: 9})
	if cmd != nil || m.discordTickRunning {
		t.Fatal("a disabled tick should stop and clear its timer")
	}

	config.Current.Discord.Enabled = true
	if cmd := m.startDiscordTick(); cmd == nil || !m.discordTickRunning {
		t.Fatal("enabling Discord RPC must schedule a fresh keepalive")
	}
}
