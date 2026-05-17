# demosim（demo.py 的 Go 版）

与 `../demo.py` 逻辑对齐的 RTP 压测程序。

## 运行

在 `demosim` 目录：

```powershell
cd D:\src\egame\egame-grpc03\game\tmtg\doc\demosim
go run . -mode BASE -n 10000000
```

或在仓库根目录：

```powershell
go run ./game/tmtg/doc/demosim -config ./game/tmtg/doc/config.json -mode BASE -n 10000000
```

## 参数

| 参数 | 默认 | 说明 |
|------|------|------|
| `-mode` | `BASE` | `BASE` 普通旋转 / `BUY` 购买免费 |
| `-n` | `10000000` | 仿真局数 |
| `-config` | 自动查找 | `config.json` 路径 |

购买模式示例：

```powershell
go run . -mode BUY -n 1000000 -config ../config.json
```

```

===== WILD消除，基础符号不够wild保留不消除 （如7个香蕉+WILD）

GOROOT=D:\soft\Go #gosetup
GOPATH=D:\soft\gopath #gosetup
D:\soft\Go\bin\go.exe test -c -o C:\Users\15186\AppData\Local\JetBrains\GoLand2025.3\tmp\GoLand\___TestRtp_in_egame_grpc_game_tmtg.test.exe egame-grpc/game/tmtg #gosetup
D:\soft\Go\bin\go.exe tool test2json -t C:\Users\15186\AppData\Local\JetBrains\GoLand2025.3\tmp\GoLand\___TestRtp_in_egame_grpc_game_tmtg.test.exe -test.v=test2json -test.paniconexit0 -test.run ^\QTestRtp\E$ #gosetup
=== RUN   TestRtp
Runtime=10000000 baseRtp=41.6193%,baseWinRate=27.3982% freeRtp=32.6908% freeWinRate=39.8958%, freeTriggerRate=0.4965% avgFree=11.4725 Rtp=74.3101% 
totalWin-148620124 freeWin=65381537,baseWin-83238587 ,baseWinTime-2739822 ,freeTime-49648, freeRound-569586 ,freeWinTime-227241, elapsed=14s
Runtime=20000000 baseRtp=41.6814%,baseWinRate=27.3925% freeRtp=32.9208% freeWinRate=39.9328%, freeTriggerRate=0.4955% avgFree=11.4834 Rtp=74.6022% 
totalWin-298408852 freeWin=131683103,baseWin-166725749 ,baseWinTime-5478502 ,freeTime-99094, freeRound-1137935 ,freeWinTime-454409, elapsed=27s
Runtime=30000000 baseRtp=41.6793%,baseWinRate=27.3839% freeRtp=33.0403% freeWinRate=39.9495%, freeTriggerRate=0.4960% avgFree=11.4808 Rtp=74.7195% 
totalWin-448317292 freeWin=198241680,baseWin-250075612 ,baseWinTime-8215176 ,freeTime-148792, freeRound-1708257 ,freeWinTime-682440, elapsed=40s
Runtime=40000000 baseRtp=41.6696%,baseWinRate=27.3809% freeRtp=33.0568% freeWinRate=39.9710%, freeTriggerRate=0.4961% avgFree=11.4872 Rtp=74.7263% 
totalWin-597810540 freeWin=264454075,baseWin-333356465 ,baseWinTime-10952342 ,freeTime-198446, freeRound-2279587 ,freeWinTime-911174, elapsed=54s
Runtime=50000000 baseRtp=41.6704%,baseWinRate=27.3848% freeRtp=33.0537% freeWinRate=39.9765%, freeTriggerRate=0.4960% avgFree=11.4852 Rtp=74.7241% 
totalWin-747241344 freeWin=330537182,baseWin-416704162 ,baseWinTime-13692387 ,freeTime-248014, freeRound-2848492 ,freeWinTime-1138728, elapsed=1m7s
Runtime=60000000 baseRtp=41.6528%,baseWinRate=27.3822% freeRtp=33.0475% freeWinRate=39.9895%, freeTriggerRate=0.4967% avgFree=11.4851 Rtp=74.7002% 
totalWin-896402583 freeWin=396569528,baseWin-499833055 ,baseWinTime-16429323 ,freeTime-297993, freeRound-3422492 ,freeWinTime-1368638, elapsed=1m20s
Runtime=70000000 baseRtp=41.6690%,baseWinRate=27.3844% freeRtp=32.9636% freeWinRate=39.9977%, freeTriggerRate=0.4964% avgFree=11.4840 Rtp=74.6326% 
totalWin-1044856387 freeWin=461491046,baseWin-583365341 ,baseWinTime-19169091 ,freeTime-347482, freeRound-3990477 ,freeWinTime-1596100, elapsed=1m33s
Runtime=80000000 baseRtp=41.6780%,baseWinRate=27.3867% freeRtp=32.9742% freeWinRate=40.0059%, freeTriggerRate=0.4965% avgFree=11.4831 Rtp=74.6522% 
totalWin-1194434691 freeWin=527587098,baseWin-666847593 ,baseWinTime-21909349 ,freeTime-397178, freeRound-4560847 ,freeWinTime-1824609, elapsed=1m47s
Runtime=90000000 baseRtp=41.6783%,baseWinRate=27.3869% freeRtp=33.0043% freeWinRate=40.0044%, freeTriggerRate=0.4970% avgFree=11.4848 Rtp=74.6827% 
totalWin-1344287833 freeWin=594077624,baseWin-750210209 ,baseWinTime-24648233 ,freeTime-447325, freeRound-5137437 ,freeWinTime-2055202, elapsed=2m0s
Runtime=100000000 baseRtp=41.6766%,baseWinRate=27.3876% freeRtp=32.9277% freeWinRate=39.9955%, freeTriggerRate=0.4971% avgFree=11.4840 Rtp=74.6044% 
totalWin-1492087324 freeWin=658554696,baseWin-833532628 ,baseWinTime-27387630 ,freeTime-497133, freeRound-5709087 ,freeWinTime-2283378, elapsed=2m14s
Runtime=100000000 baseRtp=41.6766%,baseWinRate=27.3876% freeRtp=32.9277% freeWinRate=39.9955%, freeTriggerRate=0.4971% avgFree=11.4840 Rtp=74.6044% 
totalWin-1492087324 freeWin=658554696,baseWin-833532628 ,baseWinTime-27387630 ,freeTime-497133, freeRound-5709087 ,freeWinTime-2283378, elapsed=2m14s

运行局数: 100000000，用时: 2m14s，速度: 751880 局/秒

[基础模式统计]
基础模式总游戏局数: 100000000
基础模式总投注(倍数): 2000000000.00
基础模式总奖金: 833532628.00
基础模式RTP: 41.6766%
基础模式触发免费次数: 497133
基础模式触发免费比例: 0.4971%
基础模式平均每局免费次数: 0.0571
基础模式中奖率: 27.3876%
基础模式中奖局数: 27387630

[免费模式统计]
免费模式总游戏局数: 5709087
免费模式总奖金: 658554696.0000
免费模式RTP: 32.9277%
免费模式中奖率: 39.9955%
免费模式中奖局数: 2283378

[免费触发效率]
  总免费游戏次数: 5709087 | 总触发次数: 497133
  平均每次触发获得免费次数: 11.4840

[总计]
总回报率(RTP): 74.6044%
总投注金额: 2000000000.00
总奖金金额: 1492087324.00

--- PASS: TestRtp (133.82s)
PASS

Process finished with the exit code 0

```

```

GOROOT=D:\soft\Go #gosetup
GOPATH=D:\soft\gopath #gosetup
D:\soft\Go\bin\go.exe test -c -o C:\Users\15186\AppData\Local\JetBrains\GoLand2025.3\tmp\GoLand\___TestRtp_in_egame_grpc_game_tmtg.test.exe egame-grpc/game/tmtg #gosetup
D:\soft\Go\bin\go.exe tool test2json -t C:\Users\15186\AppData\Local\JetBrains\GoLand2025.3\tmp\GoLand\___TestRtp_in_egame_grpc_game_tmtg.test.exe -test.v=test2json -test.paniconexit0 -test.run ^\QTestRtp\E$ #gosetup
=== RUN   TestRtp
Runtime=10000000 baseRtp=45.3706%,baseWinRate=27.3741% freeRtp=51.4748% freeWinRate=40.0239%, freeTriggerRate=0.5105% avgFree=11.7187 Rtp=96.8454% 
totalWin-193690870 freeWin=102949668,baseWin-90741202 ,baseWinTime-2737413 ,freeTime-51052, freeRound-598265 ,freeWinTime-239449, elapsed=13s
Runtime=20000000 baseRtp=45.3916%,baseWinRate=27.3830% freeRtp=51.7201% freeWinRate=39.9233%, freeTriggerRate=0.5095% avgFree=11.7100 Rtp=97.1118% 
totalWin-388447136 freeWin=206880552,baseWin-181566584 ,baseWinTime-5476591 ,freeTime-101895, freeRound-1193195 ,freeWinTime-476363, elapsed=27s
Runtime=30000000 baseRtp=45.4103%,baseWinRate=27.3815% freeRtp=51.8469% freeWinRate=39.9391%, freeTriggerRate=0.5101% avgFree=11.7096 Rtp=97.2571% 
totalWin-583542883 freeWin=311081299,baseWin-272461584 ,baseWinTime-8214451 ,freeTime-153042, freeRound-1792054 ,freeWinTime-715730, elapsed=40s
Runtime=40000000 baseRtp=45.4056%,baseWinRate=27.3841% freeRtp=51.9902% freeWinRate=39.9332%, freeTriggerRate=0.5110% avgFree=11.7147 Rtp=97.3958% 
totalWin-779166203 freeWin=415921650,baseWin-363244553 ,baseWinTime-10953646 ,freeTime-204394, freeRound-2394417 ,freeWinTime-956168, elapsed=54s
Runtime=50000000 baseRtp=45.4073%,baseWinRate=27.3817% freeRtp=51.8859% freeWinRate=39.9504%, freeTriggerRate=0.5098% avgFree=11.7142 Rtp=97.2933% 
totalWin-972932708 freeWin=518859298,baseWin-454073410 ,baseWinTime-13690873 ,freeTime-254913, freeRound-2986094 ,freeWinTime-1192957, elapsed=1m7s
Runtime=60000000 baseRtp=45.4173%,baseWinRate=27.3857% freeRtp=51.9612% freeWinRate=39.9590%, freeTriggerRate=0.5101% avgFree=11.7139 Rtp=97.3785% 
totalWin-1168541539 freeWin=623534318,baseWin-545007221 ,baseWinTime-16431393 ,freeTime-306087, freeRound-3585462 ,freeWinTime-1432713, elapsed=1m21s
Runtime=70000000 baseRtp=45.4018%,baseWinRate=27.3847% freeRtp=52.0798% freeWinRate=39.9585%, freeTriggerRate=0.5101% avgFree=11.7167 Rtp=97.4816% 
totalWin-1364742743 freeWin=729117285,baseWin-635625458 ,baseWinTime-19169263 ,freeTime-357088, freeRound-4183880 ,freeWinTime-1671817, elapsed=1m34s
Runtime=80000000 baseRtp=45.4082%,baseWinRate=27.3860% freeRtp=52.0699% freeWinRate=39.9689%, freeTriggerRate=0.5100% avgFree=11.7154 Rtp=97.4781% 
totalWin-1559650346 freeWin=833118446,baseWin-726531900 ,baseWinTime-21908802 ,freeTime-408001, freeRound-4779886 ,freeWinTime-1910466, elapsed=1m48s
Runtime=90000000 baseRtp=45.4038%,baseWinRate=27.3873% freeRtp=52.0966% freeWinRate=39.9823%, freeTriggerRate=0.5104% avgFree=11.7110 Rtp=97.5005% 
totalWin-1755008705 freeWin=937739620,baseWin-817269085 ,baseWinTime-24648597 ,freeTime-459325, freeRound-5379165 ,freeWinTime-2150714, elapsed=2m2s
Runtime=100000000 baseRtp=45.4037%,baseWinRate=27.3864% freeRtp=51.9504% freeWinRate=39.9750%, freeTriggerRate=0.5097% avgFree=11.7113 Rtp=97.3541% 
totalWin-1947082250 freeWin=1039008488,baseWin-908073762 ,baseWinTime-27386414 ,freeTime-509719, freeRound-5969462 ,freeWinTime-2386293, elapsed=2m15s
Runtime=100000000 baseRtp=45.4037%,baseWinRate=27.3864% freeRtp=51.9504% freeWinRate=39.9750%, freeTriggerRate=0.5097% avgFree=11.7113 Rtp=97.3541% 
totalWin-1947082250 freeWin=1039008488,baseWin-908073762 ,baseWinTime-27386414 ,freeTime-509719, freeRound-5969462 ,freeWinTime-2386293, elapsed=2m15s

运行局数: 100000000，用时: 2m15s，速度: 746269 局/秒

[基础模式统计]
基础模式总游戏局数: 100000000
基础模式总投注(倍数): 2000000000.00
基础模式总奖金: 908073762.00
基础模式RTP: 45.4037%
基础模式触发免费次数: 509719
基础模式触发免费比例: 0.5097%
基础模式平均每局免费次数: 0.0597
基础模式中奖率: 27.3864%
基础模式中奖局数: 27386414

[免费模式统计]
免费模式总游戏局数: 5969462
免费模式总奖金: 1039008488.0000
免费模式RTP: 51.9504%
免费模式中奖率: 39.9750%
免费模式中奖局数: 2386293

[免费触发效率]
  总免费游戏次数: 5969462 | 总触发次数: 509719
  平均每次触发获得免费次数: 11.7113

[总计]
总回报率(RTP): 97.3541%
总投注金额: 2000000000.00
总奖金金额: 1947082250.00

--- PASS: TestRtp (134.98s)
PASS

Process finished with the exit code 0

```


```
demosim | mode=BASE | n=100000000 | workers=8 | config=game\tmtg\doc\config.json
progress: 1000/1000 chunks

=============================================================================
║     SWEET WILD 仿真报告 (demosim / demo.py)  mode=BASE  n=100000000       ║
=============================================================================
【整体经济指标】
  > 总返还 (RTP)  :      97.38 %
  > Base 贡献 RTP :      42.98 %
  > Free 贡献 RTP :      54.39 %
  > FG 触发频率   : 1 / 196.7 转
  > Base 中奖率   :    27.3832 %  (27383215/100000000)
  > Free 中奖率   :    39.9683 %  (2380563/5956121)
  > MaxWin 触发数 : 22 (1 / 4545455 转)
-----------------------------------------------------------------------------
Runtime=100000000 baseRtp=42.9833% baseWinRate=27.3832% freeRtp=54.3932% freeWinRate=39.9683% freeTriggerRate=0.5083% avgFree=11.7169 Rtp=97.3765%
totalWin=1947530155 freeWin=1087864425 baseWin=859665730 baseWinTime=27383215 freeTrig=508335 freeRound=5956121 freeWinTime=2380563
=============================================================================

Process finished with the exit code 0


```


```
GOROOT=D:\soft\Go #gosetup
GOPATH=D:\soft\gopath #gosetup
D:\soft\Go\bin\go.exe test -c -o C:\Users\15186\AppData\Local\JetBrains\GoLand2025.3\tmp\GoLand\___TestRtp_in_egame_grpc_game_tmtg.test.exe egame-grpc/game/tmtg #gosetup
D:\soft\Go\bin\go.exe tool test2json -t C:\Users\15186\AppData\Local\JetBrains\GoLand2025.3\tmp\GoLand\___TestRtp_in_egame_grpc_game_tmtg.test.exe -test.v=test2json -test.paniconexit0 -test.run ^\QTestRtp\E$ #gosetup
=== RUN   TestRtp
Runtime=10000000 baseRtp=41.7301%,baseWinRate=27.3892% freeRtp=22.3464% freeWinRate=39.9278%, freeTriggerRate=0.4978% avgFree=11.2204 Rtp=64.0765%
totalWin-128153087 freeWin=44692875,baseWin-83460212 ,baseWinTime-2738916 ,freeTime-49779, freeRound-558540 ,freeWinTime-223013, elapsed=11s
Runtime=20000000 baseRtp=41.6375%,baseWinRate=27.3864% freeRtp=22.3310% freeWinRate=39.9304%, freeTriggerRate=0.4967% avgFree=11.2221 Rtp=63.9685%
totalWin-255873955 freeWin=89324100,baseWin-166549855 ,baseWinTime-5477274 ,freeTime-99339, freeRound-1114795 ,freeWinTime-445142, elapsed=23s
Runtime=30000000 baseRtp=41.6581%,baseWinRate=27.3878% freeRtp=22.3094% freeWinRate=39.9066%, freeTriggerRate=0.4969% avgFree=11.2266 Rtp=63.9675%
totalWin-383805255 freeWin=133856458,baseWin-249948797 ,baseWinTime-8216340 ,freeTime-149069, freeRound-1673545 ,freeWinTime-667855, elapsed=34s
Runtime=40000000 baseRtp=41.6700%,baseWinRate=27.3846% freeRtp=22.2433% freeWinRate=39.9075%, freeTriggerRate=0.4963% avgFree=11.2305 Rtp=63.9133%
totalWin-511306327 freeWin=177946116,baseWin-333360211 ,baseWinTime-10953841 ,freeTime-198508, freeRound-2229350 ,freeWinTime-889677, elapsed=45s
Runtime=50000000 baseRtp=41.6793%,baseWinRate=27.3856% freeRtp=22.1480% freeWinRate=39.9087%, freeTriggerRate=0.4959% avgFree=11.2372 Rtp=63.8273%
totalWin-638272783 freeWin=221479884,baseWin-416792899 ,baseWinTime-13692780 ,freeTime-247956, freeRound-2786320 ,freeWinTime-1111984, elapsed=56s
Runtime=60000000 baseRtp=41.6748%,baseWinRate=27.3873% freeRtp=22.2040% freeWinRate=39.9049%, freeTriggerRate=0.4966% avgFree=11.2413 Rtp=63.8789%
totalWin-766546482 freeWin=266448589,baseWin-500097893 ,baseWinTime-16432371 ,freeTime-297934, freeRound-3349155 ,freeWinTime-1336478, elapsed=1m7s
Runtime=70000000 baseRtp=41.6719%,baseWinRate=27.3851% freeRtp=22.1895% freeWinRate=39.8981%, freeTriggerRate=0.4968% avgFree=11.2399 Rtp=63.8613%
totalWin-894058333 freeWin=310652394,baseWin-583405939 ,baseWinTime-19169539 ,freeTime-347751, freeRound-3908685 ,freeWinTime-1559490, elapsed=1m19s
Runtime=80000000 baseRtp=41.6779%,baseWinRate=27.3853% freeRtp=22.1763% freeWinRate=39.8907%, freeTriggerRate=0.4969% avgFree=11.2403 Rtp=63.8543%
totalWin-1021668146 freeWin=354821298,baseWin-666846848 ,baseWinTime-21908216 ,freeTime-397527, freeRound-4468335 ,freeWinTime-1782449, elapsed=1m30s
Runtime=90000000 baseRtp=41.6757%,baseWinRate=27.3875% freeRtp=22.1374% freeWinRate=39.8906%, freeTriggerRate=0.4967% avgFree=11.2411 Rtp=63.8131%
totalWin-1148636253 freeWin=398472918,baseWin-750163335 ,baseWinTime-24648784 ,freeTime-447061, freeRound-5025475 ,freeWinTime-2004692, elapsed=1m42s
Runtime=100000000 baseRtp=41.6723%,baseWinRate=27.3853% freeRtp=22.1616% freeWinRate=39.8757%, freeTriggerRate=0.4968% avgFree=11.2398 Rtp=63.8339%
totalWin-1276678910 freeWin=443232512,baseWin-833446398 ,baseWinTime-27385265 ,freeTime-496761, freeRound-5583500 ,freeWinTime-2226459, elapsed=1m53s
Runtime=100000000 baseRtp=41.6723%,baseWinRate=27.3853% freeRtp=22.1616% freeWinRate=39.8757%, freeTriggerRate=0.4968% avgFree=11.2398 Rtp=63.8339%
totalWin-1276678910 freeWin=443232512,baseWin-833446398 ,baseWinTime-27385265 ,freeTime-496761, freeRound-5583500 ,freeWinTime-2226459, elapsed=1m53s

运行局数: 100000000，用时: 1m53s，速度: 892857 局/秒

[基础模式统计]
基础模式总游戏局数: 100000000
基础模式总投注(倍数): 2000000000.00
基础模式总奖金: 833446398.00
基础模式RTP: 41.6723%
基础模式触发免费次数: 496761
基础模式触发免费比例: 0.4968%
基础模式平均每局免费次数: 0.0558
基础模式中奖率: 27.3853%
基础模式中奖局数: 27385265

[免费模式统计]
免费模式总游戏局数: 5583500
免费模式总奖金: 443232512.0000
免费模式RTP: 22.1616%
免费模式中奖率: 39.8757%
免费模式中奖局数: 2226459

[免费触发效率]
总免费游戏次数: 5583500 | 总触发次数: 496761
平均每次触发获得免费次数: 11.2398

[总计]
总回报率(RTP): 63.8339%
总投注金额: 2000000000.00
总奖金金额: 1276678910.00

--- PASS: TestRtp (112.94s)
PASS

Process finished with the exit code 0


```