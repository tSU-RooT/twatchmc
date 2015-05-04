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
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ChimeraCoder/anaconda"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Global Variable
var (
	config     Config
	playerData map[string]*PlayerData
	syncPd     = new(sync.Mutex)
	dwellTime  map[string]*PlayerDwellTime
	syncDt     = new(sync.Mutex)
	mute       = false
	ver        = flag.Bool("v", false, "Show twatchmc(Golang) version and others")
	lic        = flag.Bool("l", false, "Show FLOSS Licenses")
	auth       = flag.Bool("a", false, "Authorization(Twitter Account)")
)

func main() {
	flag.Parse()
	if *ver {
		fmt.Println("twatchmc(Golang) version:0.6beta(2015/5/1) Copyright tSU-RooT")
		fmt.Println("twatchmc is free software licensed under the MIT license.")
		fmt.Println("You can get source code from https://github.com/tSU-RooT/twatchmc_go")
		return
	}
	if *lic {
		showLicenses()
		return
	}
	// Set Client Keys
	anaconda.SetConsumerKey(ComsumerKey)
	anaconda.SetConsumerSecret(ComsumerSercret)
	// Check key
	home := os.Getenv("HOME")
	var file *os.File
	var err error
	if *auth == false {
		file, err = os.Open(home + "/.twatchmc/.key")
	}
	if (*auth) || err != nil {
		authURL, tempCre, err := anaconda.AuthorizationURL("")
		if err != nil {
			fmt.Println("URLが取得できませんでした。")
			return
		}
		fmt.Println("認証を開始します。以下のURLにアクセスして認証した後、PINコードを入力してください。")
		fmt.Println(authURL)
		stdinReader := bufio.NewReader(os.Stdin)
		str, _ := stdinReader.ReadString('\n')
		str = strings.TrimRight(str, "\n")
		_, values, err := anaconda.GetCredentials(tempCre, str)
		if err != nil {
			fmt.Println("認証に失敗しました。")
			return
		}
		oauthToken := values.Get("oauth_token")
		oauthTokenSecret := values.Get("oauth_token_secret")
		// Save
		if _, err = os.Stat(home + "/.twatchmc/"); err != nil {
			err = os.Mkdir(home+"/.twatchmc/", 0700)
			if err != nil {
				fmt.Println(home + "/.twatchmc/ の作成に失敗しました。")
				return
			}
		}
		newFile, err := os.Create(home + "/.twatchmc/.key")
		if err != nil {
			fmt.Println(home + "/.twatchmc/.key の作成に失敗しました。")
			return
		}
		fileWriter := bufio.NewWriter(newFile)
		fileWriter.WriteString(oauthToken + "\n")
		fileWriter.WriteString(oauthTokenSecret + "\n")
		fileWriter.Flush()
		fmt.Println("認証は終了しました。")
		// End
		newFile.Close()
		return
	}
	fileReader := bufio.NewReader(file)
	// Read oauthToken
	var str string
	str, _ = fileReader.ReadString('\n')
	oauthToken := strings.TrimRight(str, "\n")
	str, _ = fileReader.ReadString('\n')
	oauthTokenSecret := strings.TrimRight(str, "\n")
	file.Close()
	// Set Client
	client := anaconda.NewTwitterApi(oauthToken, oauthTokenSecret)
	readConfig()
	if config.MinecraftJarFileName == "" {
		fmt.Println("twatchmc.yml に MINECRAFT_JAR_FILEを設定してください。")
		return
	} else if _, err = os.Stat(config.MinecraftJarFileName); err != nil {
		fmt.Println(config.MinecraftJarFileName, "が見つかりません。")
		return
	}
	// Pipe Process
	infoCh := make(chan string, 10)
	postCh := make(chan string, 10)
	go postProcess(postCh, client)
	if config.DwellTime {
		go timeProcess(postCh)
	}
	go analyzeProcess(infoCh, postCh)
	go func() {
		// 15分ごとに保存する。
		for {
			time.Sleep(time.Minute * 15)
			serializeData()
		}
	}()
	pipeProcess(infoCh)
	// 終了
	client.Close()
	serializeData()
	os.Exit(0)
}
func readConfig() {
	buf, err := ioutil.ReadFile("twatchmc.yml")
	if err != nil {
		return
	}
	err = yaml.Unmarshal(buf, &config)
}
func analyzeProcess(inCh chan string, postCh chan string) {
	causes := setupDeathCauses()
	playerSpeak := regexp.MustCompile("^<(.+)> (.+)$")
	playerIn := regexp.MustCompile("^(.+) joined the game$")
	playerOut := regexp.MustCompile("^(.+) left the game$")
	banPlayer := regexp.MustCompile("^Banned player (.+)$")
	var str string
	var submatch []string
	playerCount := 0
	playerNameList := make([]string, 0, 5)

	deserializeData()
	for {
		str = <-inCh
		if playerIn.MatchString(str) {
			submatch = playerIn.FindStringSubmatch(str)
			// プレイヤーネームを取得
			name := submatch[1]
			// リストを検査する
			alreadyLogin := false
			for _, n := range playerNameList {
				if n == name {
					// 二重ログインなのでスキップする
					alreadyLogin = true
				}
			}
			if alreadyLogin == false {
				playerCount++
				playerNameList = append(playerNameList, name)
				// プレイヤーデータは共有資源なのでsync.Mutexでロックをかける
				syncPd.Lock()
				_, ok := playerData[name]
				if ok == false {
					playerData[name] = &PlayerData{Name: name, DeathCount: 0, KillCount: 0, DeathHistory: make([]Death, 0), KilledTable: make(map[string]int, 0)}
				}
				syncPd.Unlock()
				syncDt.Lock()
				d, ok := dwellTime[name]
				if ok {
					d.LastLogin = time.Now()
				} else {
					dwellTime[name] = &PlayerDwellTime{Name: name, TotalTime: 0, LastLogin: time.Now()}
				}
				syncDt.Unlock()
				postCh <- (name + "が入場しました(" + strconv.Itoa(playerCount) + "人がオンライン)")
			}
		} else if playerOut.MatchString(str) {
			submatch = playerOut.FindStringSubmatch(str)
			// プレイヤーネームを取得
			name := submatch[1]
			// リストを検査する
			for i, n := range playerNameList {
				if n == name {
					playerCount--
					// 削除する
					playerNameList = append(playerNameList[:i], playerNameList[i+1:]...)
					// 滞在時間の記録
					syncDt.Lock()
					dwellTime[name].TotalTime += time.Now().Sub(dwellTime[name].LastLogin)
					dwellTime[name].LastLogin = time.Time{} // ログアウト中はゼロ時にセット
					syncDt.Unlock()
					break
				}
			}
		} else if playerSpeak.MatchString(str) {
			// プレイヤーの発言内容を検査
			submatch = playerSpeak.FindStringSubmatch(str)
			con := submatch[2]
			if con == "MUTE" {
				mute = true
				fmt.Println("twatchmc is Muted by Player")
			} else if con == "UNMUTE" {
				mute = false
				fmt.Println("twatchmc is unMuted by Player")
			} else if con == "DUMP" {
				serializeData()
				fmt.Println("PlayerData DUMPED")
			}
		} else if banPlayer.MatchString(str) {
			// プレイヤーのBAN
			submatch = banPlayer.FindStringSubmatch(str)
			// プレイヤーネームを取得
			name := submatch[1]
			postCh <- (name + "がサーバーからBANされました。")
		} else {
			// ログイン、ログアウト、ゲーム内チャット以外の場合の処理
			// 正規表現で順に探す
			for _, c := range causes {
				if c.Pattern.MatchString(str) {
					// プレイヤーの死亡と一致した場合
					submatch = c.Pattern.FindStringSubmatch(str)
					mes := c.Message
					// $1, $2の置換
					for i, s := range submatch {
						if i == 0 {
							continue
						}
						// 前後の空白文字や"〜.name" , "entity.〜"などのプロパティにかかわる物があった場合トリミングする(For Mods)
						ss := strings.Split(strings.Replace(strings.Trim(s, " "), ".name", "", -1), ".")
						submatch[i] = ss[len(ss)-1]
						mes = strings.Replace(mes, "$"+strconv.Itoa(i), submatch[i], -1)
					}
					name1 := submatch[1]
					death := Death{ID: c.ID, Type: c.Type, Timestamp: time.Now(), KilledBy: "", KilledByOtherPlayer: false}
					syncPd.Lock()
					p1, ok := playerData[name1]
					if ok {
						// DeathCount, KillCountなどを更新する。
						if c.Type == 0 || c.Type == 2 {
							// postChにメッセージを投げる
							postCh <- mes
							if event, exist := p1.DeathCountUp(death); exist {
								postCh <- event
							}
						} else if c.Type == 1 && len(submatch) >= 3 {
							name2 := submatch[2]
							death.KilledBy = name2
							p2, ok := playerData[name2]
							death.KilledByOtherPlayer = ok
							if ok {
								if _, exist := p2.KilledTable[name1]; exist {
									p2.KilledTable[name1]++
								} else {
									p2.KilledTable[name1] = 1
								}
								// メッセージを付け加える
								mes += "\n(" + name2 + " -> " + name1 + " " + strconv.Itoa(p2.KilledTable[name1]) + "回目)"
							}
							// postChにメッセージを投げる
							postCh <- mes
							if event, exist := p1.DeathCountUp(death); exist {
								// イベントがあったらpostChに投げる
								postCh <- event
							}
							if ok {
								if event, exist := p2.KillCountUp(); exist {
									// 同様にイベントがあったらpostChに投げる
									postCh <- event
								}
							}
						}
					}
					syncPd.Unlock()
					break
				}
			}
		}
	}
}
func timeProcess(postCh chan string) {
	pastTime := time.Now()
	for {
		now := time.Now()
		if pastTime.Day() != now.Day() {
			// 起動中に日付をまたいだとみなす
			list := make([]PlayerDwellTime, 0, 5)
			syncDt.Lock()
			var sum time.Duration
			for _, d := range dwellTime {
				if !d.LastLogin.IsZero() {
					// プレイヤーがログイン中なら現在時刻までを加算、その後ログイン時刻を計算上現在にセットする
					d.TotalTime += now.Sub(d.LastLogin)
					d.LastLogin = now
				}
				list = append(list, *(d))
				sum += d.TotalTime
				d.TotalTime = 0
			}
			// プレイヤーの総ログイン時間が2時間を超えているなら
			if (sum / time.Minute) >= 120 {
				SortFunc(len(list),
					func(i, j int) bool { return list[i].TotalTime < list[j].TotalTime },
					func(i, j int) { list[i], list[j] = list[j], list[i] },
				)
				// プレイヤーログイン時間の統計のお知らせをする
				mes := fmt.Sprintf("%d年%d月%d日のログイン時間\n",
					pastTime.Year(), pastTime.Month(), pastTime.Day())
				for i := len(list) - 1; i >= 0; i-- {
					h := list[i].TotalTime / time.Hour
					m := (list[i].TotalTime % time.Hour) / time.Minute
					t := fmt.Sprintf("%s %02d:%02d\n", list[i].Name, h, m)
					if len(mes)+len(t) <= 140 && list[i].TotalTime > 0 {
						mes += t
					} else {
						break
					}
				}
				postCh <- mes
			}
			syncDt.Unlock()
		}
		pastTime = time.Now()
		time.Sleep(time.Minute)
	}
}
func setupDeathCauses() (result []DeathCause) {
	result = make([]DeathCause, 45, 45)
	result[0] = DeathCause{ID: 1, Pattern: regexp.MustCompile("^(.+) was slain by (.+) using (.+)$"), Message: "$1は$2の$3で殺された。", Type: 1}
	result[1] = DeathCause{ID: 1, Pattern: regexp.MustCompile("^(.+) was slain by (.+)$"), Message: "$1は$2に殺害された！", Type: 1}
	result[2] = DeathCause{ID: 2, Pattern: regexp.MustCompile("^(.+) was fireballed by (.+)$"), Message: "$1は$2に火だるまにされてしまった。", Type: 1}
	result[3] = DeathCause{ID: 3, Pattern: regexp.MustCompile("^(.+) was killed by (.+) using magic$"), Message: "$1は$2に魔法で殺された。", Type: 1}
	result[4] = DeathCause{ID: 4, Pattern: regexp.MustCompile("^(.+) got finished off by (.+) using (.+)$"), Message: "$1は$2の$3で殺害された！！", Type: 1}
	result[5] = DeathCause{ID: 5, Pattern: regexp.MustCompile("^(.+) was pummeled by (.+)$"), Message: "$1は$2によってぺしゃんこにされた！", Type: 1}
	result[6] = DeathCause{ID: 6, Pattern: regexp.MustCompile("^(.+) was shot by arrow$"), Message: "$1は矢に射抜かれてしんでしまった！", Type: 0}
	result[7] = DeathCause{ID: 6, Pattern: regexp.MustCompile("^(.+) was shot by (.+) using (.+)$"), Message: "$1は$2の$3に射抜かれた！！", Type: 1}
	result[8] = DeathCause{ID: 6, Pattern: regexp.MustCompile("^(.+) was shot by (.+)$"), Message: "$1は$2に射抜かれた！", Type: 1}
	result[9] = DeathCause{ID: 7, Pattern: regexp.MustCompile("^(.+) drowned$"), Message: "$1は溺れしんでしまった！", Type: 0}
	result[10] = DeathCause{ID: 7, Pattern: regexp.MustCompile("^(.+) drowned whilst trying to escape (.+)$"), Message: "$1は$2から逃れようとして溺れ死んでしまった。", Type: 1}
	result[11] = DeathCause{ID: 8, Pattern: regexp.MustCompile("^(.+) fell out of the world$"), Message: "$1は奈落の底へ落ちてしまった！！！！", Type: 0}
	result[12] = DeathCause{ID: 8, Pattern: regexp.MustCompile("^(.+) fell from a high place and fell out of the world$"), Message: "$1は奈落の底へ落ちてしまった！！！！", Type: 0}
	result[13] = DeathCause{ID: 8, Pattern: regexp.MustCompile("^(.+) was knocked into the void by (.+)$"), Message: "$1は$2に奈落へ落とされた。", Type: 1}
	result[14] = DeathCause{ID: 9, Pattern: regexp.MustCompile("^(.+) fell from a high place$"), Message: "$1は高い所から落ちた。", Type: 0}
	result[15] = DeathCause{ID: 10, Pattern: regexp.MustCompile("^(.+) hit the ground too hard$"), Message: "$1は地面と強く激突してしまった。", Type: 2}
	result[16] = DeathCause{ID: 11, Pattern: regexp.MustCompile("^(.+) fell off a ladder$"), Message: "$1はツタから滑り落ちた……", Type: 2}
	result[17] = DeathCause{ID: 11, Pattern: regexp.MustCompile("^(.+) fell off some vines$"), Message: "$1は梯子から落ちた……", Type: 2}
	result[18] = DeathCause{ID: 11, Pattern: regexp.MustCompile("^(.+) fell out of the water$"), Message: "$1は水から落ちた。", Type: 2}
	result[19] = DeathCause{ID: 11, Pattern: regexp.MustCompile("^(.+) fell into a patch of fire$"), Message: "$1は火の海に落ちた。", Type: 0}
	result[20] = DeathCause{ID: 11, Pattern: regexp.MustCompile("^(.+) fell into a patch of cacti$"), Message: "$1はサボテンの上に落ちた！", Type: 2}
	result[21] = DeathCause{ID: 11, Pattern: regexp.MustCompile("^(.+) was doomed to fall by (.+)$"), Message: "$1は$2によって 命が尽きて落下した。", Type: 1}
	result[22] = DeathCause{ID: 11, Pattern: regexp.MustCompile("^(.+) was shot off some vines by (.+)$"), Message: "$1は$2によってツタから弾き落とされた。", Type: 1}
	result[23] = DeathCause{ID: 11, Pattern: regexp.MustCompile("^(.+) was shot off a ladder by (.+)$"), Message: "$1は$2によって梯子から弾き落とされた。", Type: 1}
	result[24] = DeathCause{ID: 11, Pattern: regexp.MustCompile("^(.+) was blown from a high place by (.+)$"), Message: "$1は$2によって高所から弾き落とされた。", Type: 1}
	result[25] = DeathCause{ID: 12, Pattern: regexp.MustCompile("^(.+) went up in flames$"), Message: "$1は炎に巻かれてしまった！", Type: 0}
	result[26] = DeathCause{ID: 12, Pattern: regexp.MustCompile("^(.+) walked into a fire whilst fighting (.+)$"), Message: "$1は$2と戦いながら火の中へ踏み入れてしまった！", Type: 1}
	result[27] = DeathCause{ID: 12, Pattern: regexp.MustCompile("^(.+) burned to death$"), Message: "$1はこんがりと焼けてしまった！！！", Type: 0}
	result[28] = DeathCause{ID: 12, Pattern: regexp.MustCompile("^(.+) walked into a fire whilst fighting (.+)$"), Message: "$1は$2と戦いながらカリカリに焼けてしまった。", Type: 1}
	result[29] = DeathCause{ID: 12, Pattern: regexp.MustCompile("^(.+) was burnt to a crisp whilst fighting (.+)$"), Message: "$1は$2と戦いながらカリカリに焼けてしまった。", Type: 1}
	result[30] = DeathCause{ID: 13, Pattern: regexp.MustCompile("^(.+) tried to swim in lava$"), Message: "$1は溶岩遊泳を試みた。", Type: 0}
	result[31] = DeathCause{ID: 13, Pattern: regexp.MustCompile("^(.+) tried to swim in lava while trying to escape (.+)$"), Message: "$1は$2から逃れようと溶岩遊泳を試みた。", Type: 1}
	result[32] = DeathCause{ID: 13, Pattern: regexp.MustCompile("^(.+) tried to swim in lava to escape (.+)$"), Message: "$1は$2から逃れようと溶岩遊泳を試みた。", Type: 1}
	result[33] = DeathCause{ID: 14, Pattern: regexp.MustCompile("^(.+) starved to death$"), Message: "$1はお腹がすいて飢え死にしてしまった！", Type: 0}
	result[34] = DeathCause{ID: 15, Pattern: regexp.MustCompile("^(.+) was killed by magic$"), Message: "$1は魔法で殺された。", Type: 0}
	result[35] = DeathCause{ID: 16, Pattern: regexp.MustCompile("^(.+) was blown up by (.+)$"), Message: "$1は$2に爆破されてしまった！", Type: 1}
	result[36] = DeathCause{ID: 16, Pattern: regexp.MustCompile("^(.+) blew up$"), Message: "$1は爆発に巻き込まれてしまった！", Type: 0}
	result[37] = DeathCause{ID: 17, Pattern: regexp.MustCompile("^(.+) suffocated in a wall$"), Message: "＊$1は壁の中で窒息してしまった＊", Type: 2}
	result[38] = DeathCause{ID: 18, Pattern: regexp.MustCompile("^(.+) died$"), Message: "$1は死んだ。", Type: 2}
	result[39] = DeathCause{ID: 19, Pattern: regexp.MustCompile("^(.+) was squashed by a falling block$"), Message: "$1は落下してきたブロックに押しつぶされた。", Type: 2}
	result[40] = DeathCause{ID: 20, Pattern: regexp.MustCompile("^(.+) was squashed by a falling anvil$"), Message: "$1は落下してきた金床におしつぶされた", Type: 2}
	result[41] = DeathCause{ID: 21, Pattern: regexp.MustCompile("^(.+) was killed while trying to hurt (.+)$"), Message: "$1は$2を傷つけようとして殺されました！！", Type: 1}
	result[42] = DeathCause{ID: 22, Pattern: regexp.MustCompile("^(.+) withered away$"), Message: "$1は枯れ果ててしまった。", Type: 2}
	result[43] = DeathCause{ID: 23, Pattern: regexp.MustCompile("^(.+) was pricked to death$"), Message: "$1はサボテンに刺されて死んでしまった", Type: 2}
	result[44] = DeathCause{ID: 23, Pattern: regexp.MustCompile("^(.+) walked into a cactus whilst trying to escape (.+)$"), Message: "$1は$2から逃げようとしてサボテンにぶつかってしまった。", Type: 1}

	return
}
func serializeData() {
	syncPd.Lock()
	defer syncPd.Unlock()
	var serializeSlice []PlayerData
	for _, val := range playerData {
		serializeSlice = append(serializeSlice, *(val))
	}
	if _, err := os.Stat(".twatchmc/"); err != nil {
		err = os.Mkdir(".twatchmc/", 0700)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	file1, err := os.Create(".twatchmc/player_data.json")
	defer file1.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
	err = json.NewEncoder(file1).Encode(serializeSlice)
	if err != nil {
		fmt.Println(err)
	}
	syncDt.Lock()
	defer syncDt.Unlock()
	dtd := DwellTimeData{Timestamp: time.Now(), Contents: make([]PlayerDwellTime, 0, 5)}
	now := time.Now()
	for _, val := range dwellTime {
		if !val.LastLogin.IsZero() {
			val.TotalTime += now.Sub(val.LastLogin)
			val.LastLogin = now // 保存のたびに現在時刻にセット
		}
		dtd.Contents = append(dtd.Contents, *(val))
	}
	file2, err := os.Create(".twatchmc/dwelltime.json")
	defer file2.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
	err = json.NewEncoder(file2).Encode(dtd)
	if err != nil {
		fmt.Println(err)
	}
}
func deserializeData() {
	syncPd.Lock()
	defer syncPd.Unlock()
	playerData = make(map[string]*PlayerData, 0)
	var deserializeSlice []PlayerData
	file1, err := os.Open(".twatchmc/player_data.json")
	defer file1.Close()
	if err != nil {
		return
	}
	err = json.NewDecoder(file1).Decode(&deserializeSlice)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, v := range deserializeSlice {
		nv := v
		playerData[v.Name] = &nv
	}
	syncDt.Lock()
	defer syncDt.Unlock()
	var dtd DwellTimeData
	file2, err := os.Open(".twatchmc/dwelltime.json")
	defer file2.Close()
	if err != nil {
		return
	}
	err = json.NewDecoder(file2).Decode(&dtd)
	if err != nil {
		fmt.Println(err)
		return
	}
	// 日付が同じなら
	if now := time.Now(); (now.Sub(dtd.Timestamp) <= time.Hour*24) && now.Day() == dtd.Timestamp.Day() {
		for _, v := range dtd.Contents {
			nv := v
			dwellTime[v.Name] = &nv
		}
	}
}

func postProcess(ch chan string, client *anaconda.TwitterApi) {
	var str string
	for {
		str = <-ch
		if !mute {
			_, err := client.PostTweet(str, nil)
			if err != nil {
				fmt.Println("Post Failed!:" + str)
			} else {
				fmt.Println("Posted:" + str)
			}
		} else {
			fmt.Println("Mute:" + str)
		}
	}
}
func pipeProcess(ch chan string) {
	var cmd *exec.Cmd
	if config.Option != nil && len(config.Option) > 0 {
		cmd = exec.Command("java")
		cmd.Args = append(cmd.Args, config.Option...)
		cmd.Args = append(cmd.Args, "-jar", config.MinecraftJarFileName, "nogui")
	} else {
		cmd = exec.Command("java", "-jar", config.MinecraftJarFileName, "nogui")
	}
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	outpipe, err := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("twatchmc is starting process...")
	scanner := bufio.NewScanner(outpipe)
	var reg1 *regexp.Regexp
	if config.Detection != "" {
		reg1, err = regexp.Compile(config.Detection)
		if err != nil {
			reg1 = regexp.MustCompile("^.+Server thread/INFO.*For help, type .*$")
		}
	} else {
		reg1 = regexp.MustCompile("^.+Server thread/INFO.*For help, type .*$")
	}
	reg2 := regexp.MustCompile("^.+Server thread/INFO.*: (.+)$")
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		if reg1.MatchString(line) {
			break
		}
	}
	if config.ServerName != "" {
		ch <- config.ServerName + "が起動しました。"
	}
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		if reg2.MatchString(line) {
			submatch := reg2.FindStringSubmatch(line)
			ch <- submatch[1]
		}
	}
	cmd.Wait()
}
