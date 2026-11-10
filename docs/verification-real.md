# 真机验证记录（上游 dstat 0.8.0 vs Go 版）

- 验证主机：`192.168.0.112`（key 免密登录，root）
- 远端工作目录：`/data/godstat`（所有操作限制在此目录内，未改动其它路径）
- 验证时间：2026-11-10 13:49 ~ 14:00（CST）
- 结论：**27 个用例全部结构一致（PASS_COUNT=27，FAIL_COUNT=0）**，无阻塞项。
  真机暴露并修复了 2 个实现缺陷（页面大小硬编码、`sysfs_dev` 语义错误），修正后复测通过。

---

## 1. 环境确认

实际执行：

```bash
uname -m; uname -r; getconf PAGESIZE; getconf CLK_TCK
/usr/bin/python2.7 -V; /usr/bin/python3.7 -V; /data/devtools/python13/bin/python3 -V
which dstat; head -3 /usr/bin/dstat
```

原始输出：

```
ARCH=aarch64
KERNEL=4.19.90-25.46.v2101.ky10.aarch64
USER=root
PAGESIZE=65536
CLK_TCK=100
Python 2.7.18
Python 3.7.9
Python 3.13.0
/usr/bin/dstat
#!/usr/bin/env pmpython
#
# Copyright (C) 2018 Red Hat.
```

| 项目 | 结果 | 影响 |
|---|---|---|
| 架构 | `aarch64` | 需 `GOARCH=arm64` 交叉编译 |
| 内核 | `4.19.90-25.46.v2101.ky10.aarch64` | `/proc` 布局为常规 Linux 4.x |
| 页面大小 | **65536**（64K 页） | 见 §7.1，Go 版原先硬编码 4096 导致 `-g` 首行差 16 倍 |
| CLK_TCK | 100 | 与实现中 `clockTicks()` 常量一致，`--float` 的 tick 换算无需调整 |
| python2.7 | `/usr/bin/python2.7`（2.7.18，含 six） | 上游 0.8.0 可直接以 py2 运行，作为权威对照 |
| python3 | `/usr/bin/python3.7`（3.7.9）、`/data/devtools/python13/bin/python3`(3.13.0) | 3.7 用于跑比对脚本 |
| 系统 `dstat` | `/usr/bin/dstat` 是 **pcp 包装脚本**（`#!/usr/bin/env pmpython`，导入 `pcp.pmapi` 失败） | 不可用；改用 `/data/godstat/upstream/dstat`（上游源码副本） |

上游源码获取：仓库内 `_reference/dstat/dstat`（0.8.0，2834 行）上传到 `/data/godstat/upstream/dstat`，作为唯一权威对照；另有一份 `upstream/dstat_py2fix`（仅把 `loop = (update - 1 + op.delay) / op.delay` 改成 `//`），原因见 §8.1。

---

## 2. 构建与部署

```bash
# Windows 本机（Go 1.27.1）
GOOS=linux GOARCH=arm64 go build -o dstat_arm64 .
scp dstat_arm64 192.168.0.112:/data/godstat/dstat_go
```

远端产物：`-rwxr-xr-x 1 root root 3295353 /data/godstat/dstat_go`（3.15 MB，静态链接）。

---

## 3. 对照方法

**执行方式**：同一台机器、同一时间点并发启动上游与 Go 版（`timeout 25 ... &` 两条并行），命令参数完全相同，各自输出重定向到 `/data/godstat/logs/{up,go}.<用例>`。数值必然因采样时刻错开而有偏差，因此不做逐字节比对。

**判定维度**（脚本 `logs/../shape.py`，远端 `/data/godstat/shape.py`）：

| 维度 | 判定 |
|---|---|
| 列顺序 / 表头 / 总宽度 | `key = (行长度, 所有 '|' 的下标元组, 数值个数)`，三者任一不同即 `SHAPE_FAIL`（硬性） |
| 单位 | 提取行内字母与 `%` 序列比较，不同则给 `UNIT` 提示（软性，多为 k/M 单位边界，不判失败） |
| 量级 / 趋势 | 逐个数值比，偏差 >50% 给 `VALUE` 提示（软性，采样时刻不同导致） |
| 时间 / epoch | 归一化为 `TIME` / `EPOCH` 后再比较 |
| 首行提示 | 剔除 `You did not select any stats` 行后比较 |

CSV 用 `csvshape.py`：前 2 行（版本头、作者行）与表头行必须逐字一致，其余行只比列数与非数值字段，数值允许偏差；`Cmdline` 中的 `up.`/`go.` 归一化。

**为什么主对照统一 `delay=1`**：上游 0.8.0 带 `from __future__ import division`，`delay>1` 时首帧 `loop = (0-1+2)/2 = 0.5`，`if loop == 0` 不成立 → `showheader` 未定义，上游自身崩溃（见 §8.1）。故 `--noupdate`（必须 `delay>1` 才有意义）改用 `dstat_py2fix` 对照，其余全部 `delay=1`。

---

## 4. 用例清单与结果

汇总来自 `/data/godstat/logs/summary.txt` 末尾：`PASS_COUNT=27`、`FAIL_COUNT=0`。

| # | 用例 | 命令 | 结果 |
|---|---|---|---|
| 1 | 默认 5 组 | `dstat 1 3` | SHAPE_OK |
| 2 | cpu | `dstat -c 1 3` | SHAPE_OK |
| 3 | disk | `dstat -d 1 3` | SHAPE_OK（VALUE/UNIT 提示见 §6） |
| 4 | net | `dstat -n 1 3` | SHAPE_OK |
| 5 | mem | `dstat -m 1 3` | SHAPE_OK |
| 6 | swap | `dstat -s 1 3` | SHAPE_OK |
| 7 | sys | `dstat -y 1 3` | SHAPE_OK |
| 8 | page | `dstat -g 1 3` | SHAPE_OK |
| 9 | int | `dstat -i 1 3` | SHAPE_OK（1 处 VALUE 提示） |
| 10 | load | `dstat -l 1 3` | SHAPE_OK |
| 11 | proc | `dstat -p 1 3` | SHAPE_OK |
| 12 | io | `dstat -r 1 3` | SHAPE_OK |
| 13 | time | `dstat -t 1 3` | SHAPE_OK（除时间戳外逐字一致） |
| 14 | 组合短参数 | `dstat -cdn 1 3` | SHAPE_OK |
| 15 | 组合短参数 | `dstat -cdm 1 3` | SHAPE_OK（1 处 VALUE 提示） |
| 16 | all | `dstat -a 1 3` | SHAPE_OK |
| 17 | vmstat | `dstat -v 1 3` | SHAPE_OK |
| 18 | 浮点 | `dstat --float -cdm 1 3` | SHAPE_OK（1 处 VALUE 提示） |
| 19 | 整数 | `dstat --integer -cdm 1 3` | SHAPE_OK |
| 20 | 无表头 | `dstat --noheaders -cdn 1 3` | SHAPE_OK（UNIT/VALUE 提示） |
| 21 | bits | `dstat --bits -dn 1 3` | SHAPE_OK |
| 22 | 全设备 | `dstat -d -f 1 3` | SHAPE_OK |
| 23 | net total | `dstat -n -N total 1 3` | SHAPE_OK |
| 24 | noupdate | `dstat --noupdate -cd 2 3`（上游用最小修正副本） | SHAPE_OK |
| 25 | CSV | `dstat -o ... -cdn 1 3` | CSV_OK（up=6 行 / go=9 行，原因见 §8.2） |
| 26 | CSV | `dstat -o ... -a 1 3` | CSV_OK（同上） |
| 27 | CSV | `dstat -o ... -t 1 3` | CSV_OK（同上） |

---

## 5. 原始输出片段

### 5.1 默认 5 组（`dstat 1 3`）

```
=== up.default ===                          === go.default ===
usr sys idl wai stl| read  writ| recv  send|  in   out | int   csw
 24   2  74   0   0|  22M 4059k|   0     0 |  96B 2586B|  49k  132k
 18   2  80   0   0|  32k  716k| 216k  218k|   0     0 |  79k  149k
 14   2  85   0   0| 248k  908k|  58k   60k|   0     0 |  60k  113k
                                            usr sys idl wai stl| read  writ| recv  send|  in   out | int   csw
                                             24   2  74   0   0|  22M 4059k|   0     0 |  96B 2586B|  49k  132k
                                             18   2  80   0   0|  32k  720k| 217k  220k|   0     0 |  77k  145k
                                             12   2  86   0   0| 248k  900k|  58k   60k|   0     0 |  61k  116k
```

首行（loop 0，开机以来均值）逐字相同：`24 2 74 0 0| 22M 4059k| 0 0 | 96B 2586B| 49k 132k`，证明累计态解析与首帧公式一致。

### 5.2 组合短参数（`-cdm`）

```
--total-cpu-usage-- -dsk/total- ------memory-usage-----
usr sys idl wai stl| read  writ| used  free  buff  cach
 24   2  74   0   0|  22M 4059k|63.2G 42.4G 1069M  142G
 13   3  84   0   0|  32k   12M|63.3G 42.3G 1069M  142G      <- up
 12   2  85   0   0| 424k   12M|63.3G 42.3G 1069M  142G      <- go
 14   3  83   0   0|8192B 4744k|63.6G 42.0G 1069M  142G
```

### 5.3 磁盘全设备（`-d -f`）：3 块盘逐盘分列，`:` 分隔

```
--dsk/sda-- --dsk/sdb-- --dsk/sdc--
 read  writ: read  writ: read  writ
  22M 3862k:  84k  197k:   4B    0
  32k  724k:   0   356k:   0     0
4096B  932k:4096B   76k:   0     0
```

（这是修复 `sysfs_dev` 前失败、修复后对齐的用例：标题为 `--dsk/sda--` 而非 `--dsk/INTEL-S...--`。）

### 5.4 中断（`-i`）：前两行为累计值，完全一致

```
----interrupts---
 487   489   491
   7     2     2
   0     0     8      <- up
   0     0     0
```

### 5.5 `--bits`（单位由字节换算为 bit）

```
-dsk/total- -net/total-
 read  writ| recv  send
 188M   33M|   0     0
  33k 5603k| 539k  566k
  27M 5308k| 487k  522k
```

### 5.6 `--noupdate`（`delay=2`，仅输出一次表头，数据行间隔 2 秒）

```
--total-cpu-usage-- -dsk/total-
usr sys idl wai stl| read  writ
 24   2  74   0   0|  22M 4059k
 15   2  82   0   0|  18k   10M
 17   2  81   0   0|6144B 9458k
```

### 5.7 `-t`（除时间戳外逐字一致）

```
----system----
     time
10-11 13:59:29
10-11 13:59:30
10-11 13:59:31
```

### 5.8 CSV（`-o`）——表头逐字一致

`/data/godstat/logs/go.cdn.csv`：

```
"Dstat 0.8.0 CSV output"
"Author:","Dag Wieers <dag@wieers.com>",,,,"URL:","http://dag.wieers.com/home-made/dstat/"
"Host:","localhost.localdomain",,,,"User:","root"
"Cmdline:","dstat -o /data/godstat/logs/go.cdn.csv -cdn 1 3",,,,"Date:","10 Nov 2026 13:59:56 CST"
"total cpu usage",,,,,"dsk/total",,"net/total",
"usr","sys","idl","wai","stl","read","writ","recv","send"
23.927,1.995,74.016,0.062,0,23553823.898,4156245.338,0,0
17.076,1.643,81.171,0.110,0,4096,1040384,76034,80208
12.435,1.056,86.493,0.016,0,4096,737280,64496,72570
```

上游同名文件到第 6 行（`"usr","sys",...`）即止，原因见 §8.2。

---

## 6. 逐项对齐结论与偏差解释

**已对齐（逐项确认）**：所有用例的列数、列顺序、`|` 分隔位置、行宽、表头文案、单位后缀集合、首行累计语义、趋势（峰谷方向）。

`VALUE`/`UNIT` 提示共 7 处，全部为采样时刻错开导致，非实现差异：

| 用例 | 现象 | 解释 |
|---|---|---|
| `-d` 第 1 数据行 | `24k` vs `4096B` | 4K 页单次块 IO；上游采样窗口内恰好 6 次、Go 1 次 |
| `-d` / `--noheaders` 第 2 行 | `9764k` vs `10M`、`9664k` vs `10M` | 同一数值恰好跨过 1024 进位边界，`UNIT` 提示为字母序列不同，量级一致 |
| `-cdm` / `--float` | `32k` vs `424k`、`0.1` vs `0.0` | 两次采样起点相差毫秒级，磁盘读/软中断抖动 |
| `-i` | `0` vs `1` | 单次中断计数，绝对差 1 |
| `-c` 各用例 usr/idl | `13/84` vs `12/85` | 同一秒内的 CPU 时间片分配不同 |

判定口径：以上均不影响「列顺序、表头、单位、量级、变化趋势」五项，故计为通过。

---

## 7. 真机暴露并修复的实现缺陷

### 7.1 页面大小硬编码 4096（真机 64K 页）

`-g` 首行（loop 0，值为「开机以来 / uptime」）上游 `96B`、Go 版 `6B`，恰好 16 倍差。根因：上游用 `resource.getpagesize()`，Go 版写成常量 4096，而本机 `PAGESIZE=65536`。

修复（[main.go](../main.go#L960)）：

```go
func pageSize() int { return os.Getpagesize() }
```

`page` / `page24` 插件统一使用 `float64(E.pagesize)`。修复后 `-g` SHAPE_OK，首行数值同量级。

### 7.2 `sysfs_dev` 语义错误

修复前 `-d -f` 表头为 `--dsk/INTEL-S...--`（误读 `/sys/block/*/device/model`），上游为 `--dsk/sda--`。

上游 `sysfs_dev()` 只做三件事，已照抄（[plugins.go](../plugins.go#L94)）：

| 输入 | 输出 |
|---|---|
| `ide/hostH/busB/targetT/lunL/disc` | `hd{a + B*2 + L}` |
| `cciss/(cXdY)` | `cXdY` |
| `placeholder...` | `sdX` |
| 其它 | 原样返回 |

修复后 `-d -f` 的 3 块盘标题、`:` 分隔、列宽全部对齐。

### 7.3 其它真机校验项

- `CLK_TCK=100` 与 `clockTicks()` 常量一致（[main.go](../main.go#L958)），`--float` tick 换算无偏差。
- `HOSTNAME` / 用户名 / CSV 的 `Host:`、`User:`、`Date:` 行与上游格式一致。

---

## 8. 上游自身缺陷（如实记录，未在 Go 中复刻）

### 8.1 `delay > 1` 必崩：`NameError: showheader`

```bash
/usr/bin/python2.7 upstream/dstat -cd 2 1
```

```
Traceback (most recent call last):
  File "upstream/dstat", line 2825, in <module>
    main()
  File "upstream/dstat", line 2684, in main
    scheduler.run()
  File "/usr/lib64/python2.7/sched.py", line 117, in run
    action(*argument)
  File "upstream/dstat", line 2769, in perform
    if showheader:
NameError: global name 'showheader' is not defined
```

根因：文件头 `from __future__ import absolute_import, division, ...`，`loop = (update - 1 + op.delay) / op.delay` 在 `delay=2` 时得 `0.5`（py2 真除法），`loop == 0` 分支（初始化 `showheader` 的唯一位置）永不进入。

处理：主对照统一 `delay=1`；`--noupdate` 用 `upstream/dstat_py2fix`（仅把该行 `/` 改为 `//`）对照，Go 侧保持与文档语义一致（`loop` 仍按解析式真除），因此对 `delay>1` Go 版可正常运行而上游崩溃，属**上游缺陷、Go 侧不跟随**。

### 8.2 `-o` CSV 只写表头：`NameError: types`

```bash
/usr/bin/python2.7 upstream/dstat -o logs/probe.csv -c 1 1
```

```
  File "upstream/dstat", line 2784, in perform
    oline = oline + o.showcsv() + o.showcsvend(totlist, vislist)
  File "upstream/dstat", line 544, in showcsv
    if isinstance(self.val[name], types.ListType) or isinstance(self.val[name], types.TupleType):
NameError: global name 'types' is not defined
CSV_LINES=6
```

源码从未 `import types`，`showcsv()` 一进入数据行就抛异常，文件停留在 6 行表头。因此 CSV 三个用例只能比对前 6 行（两侧逐字一致），Go 版多出的 3 行数据按格式规范校验（列数 9、浮点原值、无单位），符合上游 `writecsv()` 的设计意图。

---

## 9. 未在真机覆盖 / 已知限制

| 项 | 状态 |
|---|---|
| tty 颜色、`\r` 刷新、行尾复位 | 真机为管道输出（非 tty），已在 WSL1 的 pty 中 8/8 验证；本轮未在真机 pty 重跑 |
| 外部插件 `plugins/dstat_*.py` | Go 不支持，`--外部名` 报 `option --xxx not recognized`（同上游对未知长选项的处理） |
| `-i -I <网卡名>` | 上游 `int24` 缺陷（把方法当字典），无法用固定字符串对齐，Go 报 `Module dstat_int24 failed to load. (No counter objects to monitor)` |
| `--disk24-old` | 现代内核 `/proc/stat` 无 `disk_io` 行，两侧均报 `No suitable block devices found to monitor` |
| `--profile` | Go 侧提示不支持（无对应 profiler 接口） |
| `-V` 的 `Python` 行 | Go 打印 `Python (go port)`，无对应解释器信息 |

---

## 10. 复现步骤

```bash
# 1) 环境
ssh 192.168.0.112
cd /data/godstat && mkdir -p upstream logs

# 2) 上传：dstat_go（arm64）、upstream/dstat、upstream/dstat_py2fix、
#    compare_real.sh、shape.py、csvshape.py（均为验证用临时脚本，不入库）

# 3) 全量对照（24 文本 + 3 CSV，约 100 秒）
bash /data/godstat/compare_real.sh
cat /data/godstat/logs/summary.txt | tail -3
# PASS_COUNT=27
# FAIL_COUNT=0

# 4) 单例手工对照
/usr/bin/python2.7 /data/godstat/upstream/dstat -cdm 1 3
/data/godstat/dstat_go -cdm 1 3
```

原始输出全部保留在远端 `/data/godstat/logs/`（`up.*` / `go.*` / `summary.txt`）。

---

## 11. 打包与仓库验证（deb / rpm）

### 11.1 打包产物

`make packages`（或直接调用 `packaging/build-packages.sh`）用 nfpm 交叉构建，产出 4 架构 × 2 格式：

```
dstat-go-1.0.0-1.x86_64.rpm   dstat-go_1.0.0-1_amd64.deb
dstat-go-1.0.0-1.aarch64.rpm  dstat-go_1.0.0-1_arm64.deb
dstat-go-1.0.0-1.i686.rpm     dstat-go_1.0.0-1_i386.deb
dstat-go-1.0.0-1.armv7hl.rpm  dstat-go_1.0.0-1_armhf.deb
```

包内文件一致（deb 侧 `dpkg-deb -c`）：

```
-rwxr-xr-x ./usr/bin/dstat-go
-rw-r--r-- ./usr/share/man/man1/dstat-go.1
-rw-r--r-- ./usr/share/doc/dstat-go/{README.md,README.zh-CN.md,LICENSE}
-rw-r--r-- ./usr/share/doc/dstat-go/docs/{dstat-spec.md,verification-real.md}
-rw-r--r-- ./usr/share/doc/dstat-go/changelog.Debian.gz
```

元数据核对（`rpm -qip` / `dpkg-deb -I`）：`Version 1.0.0-1`、`Architecture` 与架构映射一致
（RPM `x86_64/aarch64/i686/armv7hl`，DEB `amd64/arm64/i386/armhf`）、`License GPL-2.0`、
`URL/Homepage https://github.com/yupor5/dstat`，changelog 已转换进 rpm 与 `changelog.Debian.gz`。

### 11.2 RPM 仓库（麒麟 V10 SP1 aarch64）

```
# 服务端
bash packaging/mkrepo-rpm.sh /data/godstat/repo /data/godstat/pkgs
createrepo_c --update --quiet /data/godstat/repo/rpm
python3 -m http.server 8099 --bind 127.0.0.1   # 模拟 gh-pages 的 HTTP 访问

# 客户端
sed 's#https://yupor5.github.io/dstat/rpm/#http://127.0.0.1:8099/rpm/#' dstat-go.repo \
  > /etc/yum.repos.d/dstat-go.repo
dnf -y install dstat-go
```

结果：

```
安装: dstat-go  aarch64  1.0.0-1  dstat-go  972 k
已安装: dstat-go-1.0.0-1.aarch64
$ which dstat-go            -> /usr/bin/dstat-go
$ dstat-go 1 2
--total-cpu-usage-- -dsk/total- -net/total- ---paging-- ---system--
usr sys idl wai stl| read  writ| recv  send|  in   out | int   csw
 24   2  74   0   0|  22M 4059k|   0     0 |  96B 2584B|  49k  132k
  5   2  92   0   0|  37k 2148k| 277k  300k|   0     0 |  60k  112k
$ ls -l /usr/bin/dstat      -> /usr/libexec/pcp/bin/pcp-dstat（未被覆盖）
```

升级路径（同一仓库内同时提供 1.0.0 与 1.0.1）：

```
$ rpm -q dstat-go           -> dstat-go-1.0.0-1.aarch64
$ dnf -y upgrade dstat-go
升级: dstat-go  aarch64  1.0.1-1  dstat-go  975 k
已升级: dstat-go-1.0.1-1.aarch64
$ dstat-go 1 2              -> 正常输出
```

### 11.3 DEB 仓库（WSL1 Ubuntu 22.04 amd64）

```
bash packaging/mkrepo-deb.sh /tmp/debrepo dist/packages
# 生成 deb/pool/*.deb、deb/dists/stable/main/binary-{amd64,arm64,i386,armhf}/Packages(.gz)、Release

echo "deb [trusted=yes] file:///tmp/debrepo/deb stable main" \
  | sudo tee /etc/apt/sources.list.d/dstat-go.list
sudo apt-get update -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/dstat-go.list \
  -o Dir::Etc::sourceparts=-
sudo apt-get install -y dstat-go
```

结果：

```
Get:4 file:/tmp/debrepo/deb stable/main amd64 Packages [572 B]
Get:1 file:/tmp/debrepo/deb stable/main amd64 dstat-go amd64 1.0.0-1 [1068 kB]
Setting up dstat-go (1.0.0-1) ...
$ dpkg -l dstat-go           -> ii  dstat-go  1.0.0-1  amd64  ...
$ dstat-go -c -m -l 1 2
--total-cpu-usage-- ------memory-usage----- ---load-avg---
usr sys idl wai stl| used  free  buff  cach| 1m   5m  15m
  8  10  82   0   0|23.9G 7859M 33.2M  184M|0.52 0.58 0.59
  7   7  86   0   0|24.2G 7579M 33.2M  184M|0.52 0.58 0.59
```

结构校验：`dpkg-deb -I` 四个包、`dpkg-deb -c` 文件清单均正常；`dpkg-deb -f` 输出
`Package/Version/Architecture` 与架构映射一致。

升级路径：

```
$ dpkg-query -W -f='${Package} ${Version}\n' dstat-go   -> dstat-go 1.0.0-1
$ sudo apt-get install --only-upgrade -y dstat-go
Unpacking dstat-go (1.0.1-1) over (1.0.0-1) ...
$ dpkg-query -W -f='${Package} ${Version}\n' dstat-go   -> dstat-go 1.0.1-1
```


### 11.4 WSL1 缺数据源崩溃：复现与修复

WSL1（内核 4.4.0-19041-Microsoft）的 `/proc` 覆盖不全：没有 `/proc/diskstats`、`/proc/vmstat`，
`/proc/uptime` 恒为 `0.00 1.00`。修复前默认 `-cdngy` 直接崩溃：

```
Module dstat_disk24_old failed to load. (No suitable block devices found to monitor)
Module dstat_net failed to load. (No objects discovered, no stats available)
panic: runtime error: index out of range [9] with length 9
main.(*renderer).cprint(...)  dstat/format.go:301
```

根因三层，修复三处（均不改变正常环境下的输出）：

| 根因 | 修复 |
|---|---|
| `/proc/uptime` 为 0，首轮 `elapsed=0`，速率类插件算出 `0/0=NaN` | `perform()` 中 `elapsed<=0` 时按 1 秒处理 |
| `dchg/fchg` 对 NaN 无限缩放，返回 `c=9`，`units[9]` 越界（上游此处同样 IndexError） | `cprint()` 把 NaN/±Inf 按 0 渲染，并把单位下标夹到 `len(units)-1` |
| CSV 路径 `int64(NaN)` 输出 `-9223372036854775808` | `printCsv()` 同样把 NaN/±Inf 写为 0 |

插件加载失败的处理与上游一致（上游 `open()` 在文件不可读时抛异常，插件被跳过）：
`--debug` 下可见完整回退链，例如 `-g`：

```
Module dstat_page failed to load, trying another. (Cannot open file /proc/vmstat)
Module dstat_page24
--paging--
  in   out
   0     0
```

修复后 WSL1 验证（`dstat-go 1 2`、`-g`、`-y`、`-p` 均正常退出，无 panic；`-d`/`-n` 输出
`None of the stats you selected are available.`，与上游缺数据源一致）；CSV 输出为正常数值。

真机回归（麒麟 aarch64，同机并发跑修复前 `/usr/bin/dstat-go` 与修复后二进制，3 次采样）：
归一化数值后 6/6 行完全一致（表头、列宽、`|` 位置、单位位置相同），差异仅为并发采样时刻
造成的数值跳动。


### 11.5 GitHub Pages 仓库

`release.yml` 在 `v*` 标签触发后：`packages` job 产出 deb/rpm 并上传 artifact，
`repo` job 拉取已有 gh-pages（保留旧版本）→ `createrepo_c` 刷新 RPM 元数据 →
`dpkg-scanpackages` + `apt-ftparchive` 生成 DEB 索引 → 推送 gh-pages。
站点：`https://yupor5.github.io/dstat/`（`/rpm/`、`/deb/`、`dstat-go.repo`、`dstat-go.list`）。
仓库未签名，客户端分别使用 `gpgcheck=0` 与 `[trusted=yes]`。

