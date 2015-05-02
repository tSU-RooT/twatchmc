package main

import (
	"testing"
	"time"
)

func TestDeathCountUp(t *testing.T) {
	player := PlayerData{Name: "User", DeathCount: 0, KillCount: 0,
		DeathHistory: make([]Death, 0), KilledTable: make(map[string]int, 0)}
	death1 := Death{ID: 10, Type: 0, Timestamp: time.Now(), KilledBy: "", KilledByOtherPlayer: false}
	event, exist := player.DeathCountUp(death1)
	if event != "" || exist {
		t.Fatal("Return Message should be empty")
	}
	for i := 0; i < 9; i++ {
		event, exist = player.DeathCountUp(death1)
	}
	if !(event == (player.Name+"は 通算10回 死亡した。") && exist) {
		t.Fatal("Return Message should be \"" + player.Name + "は 通算10回 死亡した。\"")
	}
}
