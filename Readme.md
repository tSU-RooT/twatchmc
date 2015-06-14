#ReadMe

twatchmcはMinecraftのサーバログをTwitterに自動投稿するプログラムです。  
主にプレイヤーの死亡時のログを自動的に投稿します。  
いわゆるバニラ(MOD)が入っていない状態の主要な死因に対応しています。  
![画面](https://raw.githubusercontent.com/wiki/tSU-RooT/twatchmc/images/record.gif)  
````
(執筆及び最終更新:2015/5/4 作者:tSU-RooT ,<tsu.root@gmail.com>)  
````  
twatchmc(Golang) version:0.7beta(2015/5/3)　　  
## Install  
````
$ go get github.com/tSU-RooT/twatchmc　　
````
## Setting
マインクラフトのフォルダに以下のようにtwatchmc.ymlを置く
````
MINECRAFT_JAR_FILE: minecraft_server.1.7.9.jar # 例
SERVER_NAME: MyMineCraftServer
SHOW_DWELLTIME: true # プレイヤーの滞在時間を日付の変わり目に表示する
OPTION: [-Xmx8128M, -Xms8128M] # JVMへのオプションの指定(各オプションは,[カンマ]で区切る)
````
## How to use　　
1.write twatchmc.yml  
2.Twitter OAuth  
  $ twatchmc -a  
3.Start  
  $ twatchmc  

## License  
MIT License  
