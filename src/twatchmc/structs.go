/*
The MIT License (MIT)

Copyright (c) 2015 tSURooT <tsu.root@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package main

import (
	"regexp"
	"strconv"
	"time"
)

type DeathCause struct {
	ID      int // ほぼ同一の死因なら正規表現のパターンか違ってもIDは同一とする(奈落落下など)
	Type    int // (0:自然死,1:他殺,2:珍しい死因)
	Pattern *regexp.Regexp
	Message string // "$1は$2に爆破された" のように記述する。
}
type PlayerData struct {
	Name         string  // プレイヤーネーム
	DeathCount   int     // 死亡数
	KillCount    int     // Kill数(これはPvPをカウントする仕様とする)
	DeathHistory []Death // 死亡履歴
}

// 死亡回数を増やす、この時 通知イベントの発生を検査する
func (this *PlayerData) DeathCountUp(d Death) (string, bool) {
	this.DeathCount += 1
	this.DeathHistory = append(this.DeathHistory, d)
	// 死因履歴を調べる
	odd_count := 0
	killed_count := 0
	for _, v := range this.DeathHistory {
		if v.Type == 2 {
			odd_count += 1
		} else if v.Type == 1 && v.KilledByOtherPlayer {
			killed_count += 1
		}
	}
	if d.Type == 2 {
		// 3回または10, 30, 50…の時に通知
		if odd_count == 3 || (odd_count%20 == 10) {
			return (this.Name + "は 通算" + strconv.Itoa(odd_count) + "回 珍しい死因で死亡した。"), true
		}
	} else if d.Type == 1 {
		// 7, 16回または10, 30, 50…の時に通知
		if killed_count == 7 || killed_count == 16 || (killed_count%20 == 10) {
			return (this.Name + "は 通算" + strconv.Itoa(killed_count) + "回 他のプレイヤーに殺された。"), true
		}
	}
	//通算死亡回数の検査
	if this.DeathCount%20 == 10 || this.DeathCount == 16 || this.DeathCount == 64 {
		return (this.Name + "は 通算" + strconv.Itoa(this.DeathCount) + "回 死亡した。"), true
	}
	return "", false
}

// Kill回数を増やす、この時 通知イベントの発生を検査する
func (this *PlayerData) KillCountUp() (string, bool) {
	this.KillCount += 1
	if this.KillCount == 7 || (this.KillCount%20 == 10) {
		return (this.Name + "は 通算" + strconv.Itoa(this.KillCount) + "回 他のプレイヤーを殺した。"), true
	}
	return "", false
}

type Death struct {
	ID                  int
	Type                int
	Timestamp           time.Time // 死亡時の
	KilledBy            string    // 何によって死亡させられたか(Mob名またはプレイヤー名)
	KilledByOtherPlayer bool      // 他プレイヤーからの攻撃で死亡した場合
}
