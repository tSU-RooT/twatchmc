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

// DeathCause :Minecraftにおける死因
type DeathCause struct {
	ID      int // ほぼ同一の死因なら正規表現のパターンか違ってもIDは同一とする(奈落落下など)
	Type    int // (0:自然死,1:他殺,2:珍しい死因)
	Pattern *regexp.Regexp
	Message string // "$1は$2に爆破された" のように記述する。
}

// PlayerData :プレイヤーについて保持する情報
type PlayerData struct {
	Name         string         // プレイヤーネーム
	DeathCount   int            // 死亡数
	KillCount    int            // Kill数(これはPvPをカウントする仕様とする)
	DeathHistory []Death        // 死亡履歴
	KilledTable  map[string]int // Killしたプレイヤーとその回数の対応付け
}

// DeathCountUp :死亡回数を増やす、この時 通知イベントの発生を検査する
func (pd *PlayerData) DeathCountUp(d Death) (string, bool) {
	pd.DeathCount++
	pd.DeathHistory = append(pd.DeathHistory, d)
	// 死因履歴を調べる
	oddCount := 0
	KilledCount := 0
	for _, v := range pd.DeathHistory {
		if v.Type == 2 {
			oddCount++
		} else if v.Type == 1 && v.KilledByOtherPlayer {
			KilledCount++
		}
	}
	if d.Type == 2 {
		// 3回または10, 30, 50…の時に通知
		if oddCount == 3 || (oddCount%20 == 10) {
			return (pd.Name + "は 通算" + strconv.Itoa(oddCount) + "回 珍しい死因で死亡した。"), true
		}
	} else if d.Type == 1 {
		// 7, 16回または10, 30, 50…の時に通知
		if KilledCount == 7 || KilledCount == 16 || (KilledCount%20 == 10) {
			return (pd.Name + "は 通算" + strconv.Itoa(KilledCount) + "回 他のプレイヤーに殺された。"), true
		}
	}
	//通算死亡回数の検査
	if pd.DeathCount%20 == 10 || pd.DeathCount == 16 || pd.DeathCount == 64 {
		return (pd.Name + "は 通算" + strconv.Itoa(pd.DeathCount) + "回 死亡した。"), true
	}
	return "", false
}

// KillCountUp :Kill回数を増やす、この時 通知イベントの発生を検査する
func (pd *PlayerData) KillCountUp() (string, bool) {
	pd.KillCount++
	if pd.KillCount == 7 || (pd.KillCount%20 == 10) {
		return (pd.Name + "は 通算" + strconv.Itoa(pd.KillCount) + "回 他のプレイヤーを殺した。"), true
	}
	return "", false
}

// Death :プレイヤーの死を記録する構造体
type Death struct {
	ID                  int
	Type                int
	Timestamp           time.Time // 死亡時の
	KilledBy            string    // 何によって死亡させられたか(Mob名またはプレイヤー名)
	KilledByOtherPlayer bool      // 他プレイヤーからの攻撃で死亡した場合
}

// PlayerDwellTime :プレイヤー滞在時間
type PlayerDwellTime struct {
	Name      string
	TotalTime time.Duration
	LastLogin time.Time
}

// DwellTimeData :滞在時間データ
type DwellTimeData struct {
	Timestamp time.Time
	Contents  []PlayerDwellTime
}

// Config :設定
type Config struct {
	MinecraftJarFileName string   `yaml:"MINECRAFT_JAR_FILE"`
	ServerName           string   `yaml:"SERVER_NAME"`
	Option               []string `yaml:"OPTION"`
	DwellTime            bool     `yaml:"SHOW_DWELLTIME"`
	Detection            string   `yaml:"DETECTION"`
}

// SortFunc :構造体のソートを行う
func SortFunc(l int, lessFunc func(i, j int) bool, swapFunc func(i, j int)) error {
	for n := 0; n < l-1; n++ {
		for m := l - 1; m > n; m-- {
			if !lessFunc(m-1, m) {
				swapFunc(m-1, m)
			}
		}
	}
	return nil
}
