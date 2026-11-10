# dstat 0.8.0 行为规范（Go 重写基准）

> 权威来源：<https://github.com/dstat-real/dstat>（`dstat` 主脚本 2834 行 + `plugins/` 100 个外部插件）
> 参考副本：`_reference/dstat`（只读，未做任何修改）
> 本文所有结论均来自逐段阅读上游源码，并用「夹具 /proc + 上游脚本」实测校验（见第 9 节）。

---

## 1. 范围

- 目标：用 Go 实现与上游 0.8.0 命令行用法、输出形态逐行一致的工具。
- 数据源：Linux `/proc`、`/sys`。Windows 原生不是目标平台。
- 本文覆盖：CLI 参数、内部插件、输出格式化、默认行为、采样模型、数据源公式、验证方法。
- 不在本文范围：外部 Python 插件（`plugins/dstat_*.py`，依赖 MySQL/MongoDB/SNMP 等外部服务），仅给出清单与归类。

---

## 2. 版本与运行环境

| 项 | 上游值 |
|---|---|
| 版本 | `VERSION = '0.8.0'`（[dstat](file:///e:/golang/source/dstat/_reference/dstat/dstat#L36)） |
| 语言 | Python 2/3（源码为 Python 2 写法，Python 3.10 下需兼容补丁） |
| 入口 | 单文件脚本 `dstat`，内部插件与引擎同文件 |
| 外部插件路径 | `~/.dstat/`、`<argv[0] 目录>/plugins/`、`/usr/share/dstat/`、`/usr/local/share/dstat/` |
| 插件发现 | `glob(dstat_*.py)`，文件名 `dstat_x_y.py` → 插件名 `x-y` |

关键全局量（[main](file:///e:/golang/source/dstat/_reference/dstat/dstat#L2511-L2527)）：

- `cpunr` = `sysconf(SC_NPROCESSORS_ONLN)`
- `hz` = `sysconf(SC_CLK_TCK)`
- `pagesize` = `getpagesize()`（通常 4096）
- `maxint` = `(sys.maxint+1)*2`（Python 3 下退化为 `+inf`）

---

## 3. 命令行接口

### 3.1 语法

```
dstat [-afv] [options..] [delay [count]]
```

- `delay`：默认 `1`，必须是**整数**，`<= 0` 报错。
- `count`：默认 `-1`（无限），整数。
- 只取位置参数前两个，多余参数被忽略。
- `delay`/`count` 解析用 `int()`，传 `1.5` 会解析失败。

### 3.2 短参数清单

getopt 短选项串（[dstat:L93](file:///e:/golang/source/dstat/_reference/dstat/dstat#L93-L97)）：

```
acdfghilmno:prstTvyC:D:I:M:N:S:V
```

| 短参数 | 带参 | 语义 | 生效方式 |
|---|---|---|---|
| `-c` | 否 | 启用 `cpu` 插件 | `plugins += ['cpu']` |
| `-C` | 是 | CPU 列表，逗号分隔；`all` = 全部 CPU | 仅设置 `cpulist`，**不启用插件** |
| `-d` | 否 | 启用 `disk` 插件 | `plugins += ['disk']` |
| `-D` | 是 | 磁盘列表（`total,hda`） | 仅设置 `disklist` |
| `-g` | 否 | 启用 `page` 插件 | `plugins += ['page']` |
| `-i` | 否 | 启用 `int` 插件 | `plugins += ['int']` |
| `-I` | 是 | 中断列表（`5,eth2`） | 仅设置 `intlist` |
| `-l` | 否 | 启用 `load` 插件 | `plugins += ['load']` |
| `-m` | 否 | 启用 `mem` 插件 | `plugins += ['mem']` |
| `-n` | 否 | 启用 `net` 插件 | `plugins += ['net']` |
| `-N` | 是 | 网卡列表（`eth1,total`） | 仅设置 `netlist` |
| `-p` | 否 | 启用 `proc` 插件 | `plugins += ['proc']` |
| `-r` | 否 | 启用 `io` 插件 | `plugins += ['io']` |
| `-s` | 否 | 启用 `swap` 插件 | `plugins += ['swap']` |
| `-S` | 是 | swap 设备列表 | 仅设置 `swaplist` |
| `-t` | 否 | 启用 `time` 插件 | `plugins += ['time']` |
| `-T` | 否 | 启用 `epoch` 插件 | `plugins += ['epoch']` |
| `-y` | 否 | 启用 `sys` 插件 | `plugins += ['sys']` |
| `-a` | 否 | 等价 `-cdngy` | `plugins += ['cpu','disk','net','page','sys']` |
| `-v` | 否 | vmstat 风格 | `plugins += ['proc','mem','page','disk','sys','cpu']` |
| `-f` | 否 | `full`：展开 `-C/-D/-I/-N/-S` 列表 | `full = True` |
| `-o` | 是 | CSV 输出文件 | `output = arg` |
| `-h` | 否 | 打印用法+帮助，退出 0 | |
| `-V` | 否 | 打印版本+插件清单，退出 0 | |
| `-M` | 是 | **已废弃**，等价于逐个 `--plugin` | 先打印 WARNING 到 stderr |

### 3.3 长参数清单

硬编码长选项（`allplugins` 会被追加到列表末尾，[dstat:L93-L97](file:///e:/golang/source/dstat/_reference/dstat/dstat#L93-L97)）：

```
all, all-plugins, bits, bw, black-on-white, color, debug, filesystem, float,
full, help, integer, list, mods, modules, nocolor, noheaders, noupdate,
output=, pidfile=, profile, version, vmstat  + 全部插件名
```

| 长参数 | 带参 | 语义 |
|---|---|---|
| `--all` | | 同 `-a` |
| `--all-plugins` | | 启用**全部**内部+外部插件（排序后去重） |
| `--bits` | | 字节类数值按 bit 显示（仅在 `scale==1024` 且 `type=='b'` 生效） |
| `--bw` / `--black-on-white` | | 白底配色方案 |
| `--color` | | 强制彩色，并强制 `update=True` |
| `--debug` | | 可重复累加（1→ms，2→loop:step，3→traceback） |
| `--filesystem` | | 启用 `fs` 插件（`--fs` 也由插件名提供） |
| `--float` | | 数值强制浮点格式 |
| `--full` | | 同 `-f` |
| `--integer` | | 数值强制整数格式 |
| `--list` | | 打印插件清单，退出 0 |
| `--mods=` / `--modules=` | 是 | 已废弃，逗号分隔插件名 |
| `--nocolor` | | 关闭颜色 |
| `--noheaders` | | 关闭**重复**表头（首次表头仍会打印） |
| `--noupdate` | | 关闭中间刷新（delay>1 时只输出整段结果行） |
| `--output=` | 是 | CSV 文件 |
| `--pidfile=` | 是 | 写 pid 文件，退出时删除 |
| `--profile` | | cProfile 统计（Go 实现可不提供，或以等价方式） |
| `--version` | | 同 `-V` |
| `--vmstat` | | 同 `-v` |
| `--<plugin>` | | 启用指定插件（内部或外部），如 `--cpu-adv`、`--tcp` |

补充语义：

- Python `getopt` 支持**长选项前缀缩写**：`--cpu-a` 唯一匹配 `--cpu-adv` 即生效；有歧义时（如 `--all-`）报错。
- 内部插件名自动成为合法长选项（`--cpu`、`--int`、`--page`、`--proc`、`--sys`、`--disk`…），因为 `listplugins()` 把它们加入了长选项表。
- 未识别的 `--xxx` 不会走到 getopt 报错分支，而是 `plugins.append(opt[2:])`，加载失败时报 `Module dstat_xxx failed to load.`（仅当它是回退链最后一环）。

### 3.4 互斥、依赖与组合

| 规则 | 说明 |
|---|---|
| `--float` + `--integer` | 互斥，报错并退出 1 |
| `-C/-D/-I/-N/-S` 单独使用 | **不启用**对应插件，只设置列表；无其他选择时回落默认 5 组 |
| 组合短参数 | 支持 `-cdn`、`-pmgdsc`、`-dfn` 等（getopt 标准行为） |
| 重复启用 | 同一插件出现多次会生成多个实例（上游未去重） |
| 插件顺序 | 严格按命令行出现顺序；`-a` 内部顺序固定 `cpu,disk,net,page,sys`；`-v` 为 `proc,mem,page,disk,sys,cpu` |
| `-v` 隐含 `-D total` | 帮助文本如此声明，但代码只加了插件，`disklist` 未设置（`disk` 默认即 `total`） |
| 插件回退 | `cpu→cpu24`、`disk→disk24→disk24-old`、`int→int24`、`page→page24`、`swap→swap-old`，逐个尝试，全部失败才报错 |

### 3.5 错误与退出码

| 场景 | stderr/stdout | 退出码 |
|---|---|---|
| 未知短参数 | `dstat: option -w not recognized, try dstat -h for a list of all the options` | 1 |
| delay/count 非整数 | `dstat: incorrect argument, try dstat -h for the correct syntax` | 1 |
| `delay <= 0` | `dstat: delay must be an integer, greater than zero` | 1 |
| `--float` + `--integer` | `dstat: option --float and --integer are mutual exclusive, you can only force one` | 1 |
| 所有选中插件都不可用 | `None of the stats you selected are available.`（stderr） | **8** |
| 单个模块失败（非最后一环） | `Module dstat_x failed to load, trying another. (原因)`（仅 `--debug`） | — |
| 单个模块失败（最后一环） | `Module dstat_x failed to load. (原因)`（stderr） | — |
| 正常结束 / Ctrl-C | — | 0 |

### 3.6 环境变量

| 变量 | 作用 |
|---|---|
| `DSTAT_OPTS` | 预置参数，拼在 `argv[1:]` **之前**（`Options(os.getenv('DSTAT_OPTS','').split() + sys.argv[1:])`） |
| `DSTAT_TIMEFMT` | `time` 插件时间格式，默认 `%d-%m %H:%M:%S`（strftime 语法） |
| `TERM` | 终端名，影响配色能力探测（curses） |
| `COLUMNS` / `LINES` | 无法 ioctl 时的终端尺寸回退 |
| `XTERM_SHELL` | 为 `bash` 且 `TERM` 匹配 `screen*/xterm*` 时改写终端标题 |

---

## 4. 插件清单

### 4.1 内部插件（34 个）

`name` 决定表头标题，`nick` 决定列名，`width` 是**单列宽度**，`scale` 是单位换算基准（1000 或 1024 会启用单位后缀）。

| 参数 | 插件 | 标题 name | nick（列名） | type | width | scale | 主要数据源 |
|---|---|---|---|---|---|---|---|
| `-c` | cpu | `total cpu usage` / `cpuN usage` | usr sys idl wai stl | p | 3 | 34 | `/proc/stat` cpu 行 |
| `--cpu-adv` | cpu-adv | 同上 | usr sys idl wai hiq siq stl | p | 3 | 34 | `/proc/stat` |
| `--cpu-use` | cpu-use | `per cpu usage` | （无，单值/CPU） | p | 3 | 34 | `/proc/stat` |
| `--cpu24` | cpu24 | `cpu usage` / `cpuN` | usr sys idl | p | 3 | 34 | `/proc/stat`（2.4 格式） |
| `-d` | disk | `dsk/<dev>` | read writ | b | 5 | 1024 | `/proc/diskstats` |
| `--disk24` | disk24 | `dsk/<dev>` | read writ | b | 5 | 1024 | `/proc/partitions` |
| `--disk24-old` | disk24-old | `dsk/<dev>` | read writ | b | 5 | 1024 | `/proc/stat` |
| `-g` | page | `paging` | in out | d | 5 | 1024 | `/proc/vmstat` |
| `--page24` | page24 | `paging` | in out | d | 5 | 1024 | `/proc/stat` `swap` 行 |
| `-y` | sys | `system` | int csw | d | 5 | 1000 | `/proc/stat` |
| `-t` | time | `system` | time | s | 14（=格式长度） | 0 | 采样时刻 |
| `-T` | epoch | `epoch` | epoch | f | 10 | 0 | 采样起始时间戳 |
| `-l` | load | `load avg` | 1m 5m 15m | f | 4 | 0.5 | `/proc/loadavg` |
| `-m` | mem | `memory usage` | used free buff cach | f | 5 | 1024 | `/proc/meminfo` |
| `--mem-adv` | mem-adv | `advanced memory usage` | total used free buff cach dirty shmem recl | f | 5 | 1024 | `/proc/meminfo` |
| `-s` | swap | `swap` | used free | d | 5 | 1024 | `/proc/swaps`（旧：`/proc/meminfo`） |
| `--swap-old` | swap-old | `swap` | used free | d | 5 | 1024 | `/proc/meminfo` |
| `-p` | proc | `procs` | run blk new | f | 3 | 10 | `/proc/stat` |
| `-r` | io | `io/<dev>` | read writ | f | 5 | 1000 | `/proc/diskstats` |
| `-n` | net | `net/<iface>` | recv send | b | 5 | 1024 | `/proc/net/dev` |
| `-i` | int | `interrupts` | 按发现的中断号 | d | 5 | 1000 | `/proc/stat` + `/proc/interrupts` |
| `--int24` | int24 | `interrupts` | 按发现的中断名 | d | 5 | 1000 | `/proc/interrupts` |
| `--fs` / `--filesystem` | fs | `filesystem` | files inodes | d | 6 | 1000 | `/proc/sys/fs/file-nr`、`inode-nr` |
| `--ipc` | ipc | `sysv ipc` | msg sem shm | d | 3 | 10 | `/proc/sysvipc/*` |
| `--lock` | lock | `file locks` | pos lck rea wri | f | 3 | 10 | `/proc/locks` |
| `--socket` | socket | `sockets` | tot tcp udp raw frg | d | 4 | 1000 | `/proc/net/sockstat` |
| `--tcp` | tcp | `tcp sockets` | lis act syn tim clo | d | 4 | 1000 | `/proc/net/tcp{,6}` |
| `--udp` | udp | `udp` | lis act | d | 4 | 1000 | `/proc/net/udp{,6}` |
| `--unix` | unix | `unix sockets` | dgm str lis act | d | 4 | 1000 | `/proc/net/unix` |
| `--raw` | raw | `raw` | raw | d | 4 | 1000 | `/proc/net/raw` |
| `--vm` | vm | `virtual memory` | majpf minpf alloc free | d | 5 | 1000 | `/proc/vmstat` |
| `--vm-adv` | vm-adv | `advanced virtual memory` | steal scanK scanD pgoru astll | d | 5 | 1000 | `/proc/vmstat` |
| `--zones` | zones | `zones memory` | d32F d32H normF normH | d | 5 | 1000 | `/proc/zoneinfo` |
| `--aio` | aio | `async` | #aio | d | 5 | 1000 | `/proc/sys/fs/aio-nr` |

基类默认值（[dstat:L309-L322](file:///e:/golang/source/dstat/_reference/dstat/dstat#L309-L322)）：`type='f'`、`width=5`、`scale=1024`、`cols=0`。

`cols` 表示该插件每个 `var` 拥有 `cols` 个子列（cpu=5/7/3，disk/io/net=2）。

### 4.2 外部插件（100 个，Go 版暂不实现）

按功能归类（完整清单见 `_reference/dstat/plugins/`，或上游 `dstat --list`）：

| 类别 | 插件 |
|---|---|
| 磁盘细分 | disk-avgqu, disk-avgrq, disk-svctm, disk-tps, disk-util, disk-wait, freespace, md-status, zfs-arc, zfs-l2arc, zfs-zil, lustre, gpfs, gpfs-ops |
| 进程 Top | top-bio, top-bio-adv, top-childwait, top-cpu, top-cpu-adv, top-cputime, top-cputime-avg, top-int, top-io, top-io-adv, top-latency, top-latency-avg, top-mem, top-oom, proc-count |
| 数据库 | mysql*, innodb-*, mongodb-*, memcache-hits, redis |
| 网络/服务 | nfs3*, nfsd3*, nfsd4-ops, nfsstat4, net-packets, rpc, rpcd, squid, postfix, qmail, sendmail, ntp, snmp-* |
| 虚拟化/硬件 | vmk-*, vm-*, vz-*, cpufreq, thermal, fan, battery, battery-remain, power, wifi, ib, fuse, dbus, utmp, condor-queue, jvm-*, helloworld, test, snooze, dstat, dstat-cpu, dstat-ctxt, dstat-mem |

### 4.3 插件装配逻辑（必须照抄）

1. 按 `op.plugins` 顺序遍历。
2. 每个插件名对应一个回退链 `mods`（见 3.4）。
3. 依次尝试 `dstat_<mod>`（`-`→`_`）：
   - 成功：`check()`（`vars`/`discover` 非空、`colwidth()>0`）→ `prepare()`。
   - 失败：`debug` 时打印 `failed to load, trying another`，继续下一环。
4. 链条全部失败：打印 `Module dstat_x failed to load. (原因)` 到 stderr。
5. 最终 `totlist` 为空 → `die(8, 'None of the stats you selected are available.')`。

`open()` 的文件语义（易错点）：`self.open(a, b, ...)` 逐个尝试打开，**只要有一个成功就视为成功**；
全部失败才抛 `Cannot open file <最后一个文件名>`。因此 `--tcp`/`--udp` 同时打开 `tcp`+`tcp6`/`udp`+`udp6`，
缺 tcp6/udp6 不影响加载；但两者都缺时，报错信息里出现的是 **tcp6/udp6**（列表末项），不是 tcp/udp。

### 4.4 `prepare()` 的三种数据模型

```
struct 不为空        → Plugin API v2（本文内部插件都不用）
cols <= 0            → v1 标量：val/set1/set2[name] = 0
cols > 0             → v1 矢量：val/set1/set2[name] = [0]*cols，且额外建 'total' 键
```

---

## 5. 输出格式规范

### 5.1 行结构

一行输出 =

```
[换行/清行/恢复序列]  [表头(仅需时)]  [插件1输出][分隔符][插件2输出]…  [输入色复位]
```

- 每个插件输出 = `show()` + `showend()`。
- `showend()`：不是最后一个可见插件 → `|`；列表被裁剪（`totlist != vislist`）→ `>`；否则空。

### 5.2 标题与副标题

单 name 插件（`name` 是字符串）：

- 标题宽度 = `statwidth()` = `len(nick)*colwidth() + len(nick)-1`
- 标题 = `name[0:width]` 居中、**空格替换为 `-`**（[dstat:L426-L439](file:///e:/golang/source/dstat/_reference/dstat/dstat#L426-L439)）
- 副标题 = 每个 nick 截断到 `width` 居中，列间 1 空格

多 name 插件（cpu 列表、disk 列表等，`name` 是列表）：

- 每个 name 居中于 `colwidth()`，空格替换 `-`；name 之间：有颜色 → `frame + '-'`，无颜色 → `' '`
- 副标题：每个 name 段内打印全部 nick，段间 `':'`

`colwidth()`：`name` 为字符串 → `width`；否则 `len(nick)*width + len(nick)-1`。

### 5.3 数值格式化算法（核心，必须逐字复刻）

`cprint(var, ctype, width, scale)`（[dstat:L2088-L2173](file:///e:/golang/source/dstat/_reference/dstat/dstat#L2088-L2173)）：

```
base = 1000; 若 scale == 1024 → base = 1024
unit = False
若 scale ∈ {1000,1024} 且 width >= len(str(base))(=4):
    unit = True; width -= 1
若 type != 's' 且 var < 0 → 输出 '-' 右对齐(width)（unit 时再补 1 空格），结束
units:
    base != 1024 → (' ', 'k','M','G','T','P','E','Z','Y')
    base == 1024 且 --bits 且 type=='b' → ('b','k','M',...)，base=scale=1000，var *= 8
    否则 → ('B','k','M','G','T','P','E','Z','Y')
选择转换函数:
    --integer 且 type ∈ b,d,p,f → dchg
    --float   且 type ∈ b,d,p,f → fchg
    type ∈ b,d,p → dchg
    type == 'f'  → fchg
    type == 's'  → 原样字符串
    type == 't'  → tchg（时间，内部插件不用）
对齐: type=='s' → ljust(width)；否则 rjust(width)
unit 时追加: 若 c != -1 且 round(var) != 0 → units[c]，否则 1 个空格
```

`dchg(var,width,base)`：循环 `ret=str(int(round(var)))`；长度 > width 则 `var/=base; c+=1` 再试。

`fchg(var,width,base)`：

```
var == 0 → '0'
ret = str(int(round(var, width)))        # round 到 width 位后取整（截断）
若 len(ret) <= width:
    i = width - len(ret) - 1
    当 i > 0 时尝试 ret = '%.{i}f' % var，若 len(ret) <= width 且 ret != str(int(round(var,width))) 则采用
    i 用尽仍不满足 → ret = str(int(round(var)))
否则 var/=base; c+=1 重试
```

`tchg(var,width)`：`%2dh%02d`（分/时），太长退化 `%2dh`、`%2dd`、`%2dw`。

### 5.4 单位与进制（实测结论）

| 插件类型 | base | 首单位 | 触发进位 |
|---|---|---|---|
| `b`（disk/net，scale 1024） | 1024 | `B` | 1024 |
| `b` + `--bits` | 1000 | `b` | 1000（值 ×8） |
| `d`/`f` + scale 1000 | 1000 | 空格 | 1000 |
| `d` + scale 1024（page/swap） | 1024 | `B` | 1024 |
| scale 非 1000/1024（cpu 34、load 0.5、proc/lock/ipc 10） | 1000 | 无单位 | 不启用单位 |

实测（夹具数据，`1 1`）：

```
-dsk/total- -net/total- ---paging-- ---system--
 read  writ| recv  send|  in   out | int   csw 
 450k 1090k|   0     0 |  40B  113B|  72    40 
```

### 5.5 分隔符

| 位置 | 字符 |
|---|---|
| 插件之间 | `|`（末列若裁剪则 `>`） |
| 插件内多 var 之间（矢量值） | `:`（如 `-C total,0`、`-D total,sda`） |
| 插件内标量 var 之间 | ` `（1 空格） |
| 标题 name 之间（无颜色） | ` `（1 空格） |
| CSV | `,` |

实测：

```
--total-cpu-usage-- -----cpu0-usage----
usr sys idl wai stl:usr sys idl wai stl
 11   3  86   0   0: 11   3  86   0   0
```

### 5.6 颜色

`color` 表为 ANSI 序列（[dstat:L1727-L1754](file:///e:/golang/source/dstat/_reference/dstat/dstat#L1727-L1754)）。深色主题关键项：

| 键 | 值 |
|---|---|
| title | `\033[0;34m`（darkblue） |
| subtitle | `\033[1;34m\033[4m`（blue+underline） |
| frame | `\033[0;34m` |
| default | `\033[0;0m` |
| error | `\033[1;37m\033[41m` |
| input | `\033[1;30m` |
| text_lo / text_hi | gray / darkgray |
| unit_lo / unit_hi | darkgray |
| done_lo / done_hi | white / gray |
| colors_lo | red yellow green blue cyan white darkred darkgreen |
| colors_hi | darkred darkyellow darkgreen darkblue darkcyan gray red green |

- `--bw` 使用另一套（title=darkblue、colors_lo=(darkred,darkmagenta,darkgreen,darkblue,darkcyan,black,red,green)）。
- 无颜色时：`color/theme/ansi` 全置空，`colors_hi=colors_lo=(ansi['default'],)`。
- 每个数值列的颜色由 **进位次数 c** 或 `int(var/scale)%len(colors)` 决定；`var==0` 用 unit 色，`%` 类型且 `>=100` 用 done 色。
- 行尾统一追加 `theme['input']`。

### 5.7 中间刷新（`--noupdate` 关闭）

| 变量 | 定义 |
|---|---|
| `interval` | `op.update ? 1 : op.delay` |
| `update` | 循环推进量（0, interval, 2×interval…） |
| `loop` | `(update-1+delay)/delay`（整数除法） |
| `step` | `((update-1) % delay) + 1`，取值 1..delay |
| `elapsed` | `loop==0` → `ticks()`（`/proc/uptime` 秒）；否则 `step` |

- `step == delay` 时才把 `set2` 写入 `set1`（即每个 delay 周期为一个统计边界）。
- 打印时：
  - `step==1 && update!=0` → 先 `\n` + reset + 清行 + save
  - `loop!=0` → 先 `ansi['restore']`（回到行首覆盖）
  - `step==delay` → `theme['default']=ansi['reset']`（定稿行），否则 `text_lo`（暗色中间行）
- 程序结束若 `op.update` 则补一个 `\n`。
- `--noupdate` 时 `step` 恒为 `delay`，每行后补 `\n`。

### 5.8 表头重复与终端裁剪

- 首行**必定**打印表头（`loop==0 → showheader=True`），`--noheaders` 只关闭后续重复。
- 重复条件（`op.header && rows>=6`）：
  - 可见列表变化
  - `--noupdate`：`loop % (rows-2) == 0`
  - 否则：`step==1 && loop % (rows-1) == 0`
- stdout 为 tty 时按终端列宽裁剪可见插件（不足则丢弃后续插件，并在 stderr 提示 `Terminal width too small, trimming output.`），裁剪用 `>` 结尾。

### 5.9 非 tty 行为（管道/重定向）

```python
if not sys.stdout.isatty():
    color = False; header = False; update = False
```

即：无颜色、不重复表头（首行表头仍打印）、不中间刷新。注意默认 5 组走 `-a` 时还会向 **stdout** 打印一行提示：

```
You did not select any stats, using -cdngy by default.
```

### 5.10 CSV 输出（`-o/--output`）

文件不存在时写入文件头，存在时追加 `\n\n`：

```
"Dstat 0.8.0 CSV output"
"Author:","Dag Wieers <dag@wieers.com>",,,,"URL:","http://dag.wieers.com/home-made/dstat/"
"Host:","<host>",,,,"User:","<user>"
"Cmdline:","dstat <原始参数>",,,,"Date:","10 Nov 2026 11:56:25 CST"
"total cpu usage",,,,          ← 标题行：单 name 时后面补 len(nick)-1 个逗号
"usr","sys","idl","wai","stl"  ← 列名行
```

- 数值行仅在每个 `step==delay` 写入。
- 数值格式：`var != round(var)` → `%.3f`，否则 `%d`（四舍五入取整）。
- 多 name 插件列名为 `"name:nick"`；单 name 时用 `"nick"`。

### 5.11 `-V/--version` 与 `--list`

`version()` 输出：

```
Dstat 0.8.0
Written by Dag Wieers <dag@wieers.com>
Homepage at http://dag.wieers.com/home-made/dstat/

Platform posix/linux
Kernel <uname -r>
Python <python version>

Terminal type: <TERM> (no color support)
Terminal size: 25 lines, 80 columns

Processors: <n>
Pagesize: 4096
Clock ticks per secs: 100

internal:
	aio,cpu,cpu-adv,...
<pluginpath 绝对路径>:
	battery,battery-remain,...
```

`--list` 只输出从 `internal:` 开始的部分（与 `-V` 尾部一致）。列表按终端列宽每行折行。

---

## 6. 默认运行（不带任何插件参数）

1. `Options` 解析后 `plugins` 为空 → 打印 `You did not select any stats, using -cdngy by default.`（**stdout**）
2. `plugins = ['cpu','disk','net','page','sys']`
3. 延迟 1s、无限次，每行即 1 秒窗口
4. 输出形态（实测）：

```
--total-cpu-usage-- -dsk/total- -net/total- ---paging-- ---system--
usr sys idl wai stl| read  writ| recv  send|  in   out | int   csw 
 11   3  86   0   0| 450k 1090k|   0     0 |  40B  113B|  72    40 
```

- **没有合计行**（dstat 不输出汇总行，`count` 次采样逐行输出）。
- 首行（loop 0）是「自开机累计值 / uptime」的均值，不是 1 秒增量；`net` 插件在 loop 0 因 `if update:` 判断而固定为 0。

---

## 7. 采样与时间模型

```
update = 0
while update <= delay*(count-1) or count == -1:
    perform(update)          # 绝对时刻 inittime + update 调度
    update += interval
```

- `count=2, delay=1` → `update` 取 0、1 → 2 行。
- `count=-1` → 无限。
- `ticks()`：读 `/proc/uptime` 第 1 字段（秒，float）；异常时用 `/proc/stat` 的 `btime` 反推 `time.time()-btime`。
- 采样耗时晚于计划 1s 以上 → 跳过该次并累计 `missed`，输出 ` missed N ticks`。

---

## 8. 数据源与计算公式

### cpu（`-c`，cols=5）

`/proc/stat` 的 `cpu` / `cpuN` 行：

```
idx1 user, idx2 nice, idx3 system, idx4 idle, idx5 iowait, idx6 irq, idx7 softirq, idx8 steal
set2 = (
  user + nice + irq + softirq,   # usr
  system,                        # sys
  idle,                          # idl
  iowait,                        # wai
  steal                          # stl
)
val[i] = 100.0 * (set2[i]-set1[i]) / (sum(set2)-sum(set1))   # 总量为 0 时取 0
```

- `usr` 含 nice；`irq+softirq` 计入 usr。`sum` 为 5 项之和。
- `--cpu-adv` 改为 7 项：`usr=user+nice, sys, idl, wai, hiq=irq, siq=softirq, stl`。
- `--cpu-use`：`val = 100 - 100*(idle_delta/sum_delta)`，cols=7。

### disk（`-d`，cols=2，type=b，scale=1024）

`/proc/diskstats`（≥13 字段，跳过全 0 行）：

```
sectors_read  = 字段6（下标5）
sectors_written = 字段10（下标9）
val = (curr - prev) * 512.0 / elapsed
```

- `total` = **不匹配** `diskfilter` 的设备之和；`diskfilter` 排除分区与 `dm-*`、`md*`、`VxVM*`、`mmcblkXpY0`、`cciss/cXdYpZ`：
  `^([hsv]d[a-z]+\d+|cciss/c\d+d\d+p\d+|dm-\d+|md\d+|mmcblk\d+p\d0|VxVM\d+)$`
- 注意：`dm-0` 会被过滤掉（不计入 total），`loop0` 会计入。
- `-D total,sda` / `-f` 决定 `vars`；`-f` 还要求设备在 `/sys/block/` 中存在。

### net（`-n`，cols=2，type=b，scale=1024）

`/proc/net/dev`，先把 `:` 换成空格：

```
name=0, rx_bytes=1, tx_bytes=9
val = (curr - prev) / elapsed（loop 0 不计算，固定 0）
负值 + maxint+1 修正计数器回绕
total 排除 lo|bond\d+|face|.+\.\d+
```

### paging（`-g`，2 标量，type=d，scale=1024）

`/proc/vmstat` 的 `pswpin` / `pswpout`：

```
val = delta * pagesize / elapsed
```

### system（`-y`，2 标量，type=d，scale=1000）

`/proc/stat` 的 `intr`、`ctxt`，`val = delta / elapsed`。

### mem（`-m`，4 标量，type=f，scale=1024）

`/proc/meminfo`（kB → ×1024）：

```
MemUsed = MemTotal - MemFree - Buffers - Cached - SReclaimable + Shmem
MemFree = MemFree; Buffers = Buffers; Cached = Cached
```

### swap（`-s`，2 标量，type=d，scale=1024）

`/proc/swaps` 每行 `Size`(idx2)、`Used`(idx3)（kB）：

```
used = Used*1024; free = (Size-Used)*1024; total 为各设备求和
```

### proc（`-p`，3 标量，type=f，scale=10）

`/proc/stat`：

```
new   = delta(processes) / elapsed
run   = procs_running - 1     （每个 step 累加，周期末清零）
blk   = procs_blocked         （每个 step 累加，周期末清零）
```

### io（`-r`，cols=2，type=f，scale=1000）

`/proc/diskstats` 字段 4（下标 3，读完成次数）与字段 8（下标 7，写完成次数），`val = delta/elapsed`，过滤规则同 disk。

### load（`-l`，3 标量，type=f，scale=0.5）

`/proc/loadavg` 前 3 个浮点，**不做除以 elapsed**。

### int（`-i`，type=d，scale=1000）

- `discover`：`/proc/stat` 的 `intr` 行，`enumerate(l[2:])` 中计数 `>10` 的下标作为中断号。
- `vars`：去掉 `0,1,2,8,NMI,LOC,MIS,CPU0`；非 `-f` 时取**最后 3 个**。
- `intmap`：`/proc/interrupts` 建立「设备名→中断号」，支持 `-I eth0` 这类写法。
- `val = delta / elapsed`；`total` 取 `intr` 总数。

### time（`-t`）/ epoch（`-T`）

- 取**本次采样起始时刻** `starttime`（不是打印时刻）。
- `time`：`strftime(DSTAT_TIMEFMT or '%d-%m %H:%M:%S')`，宽度=格式化后的长度（默认 14）。
- `epoch`：`starttime` 的浮点取整，宽度 10，scale 0（原样输出）。

### fs / ipc / lock / socket / tcp / udp / unix / raw / vm / vm-adv / zones

| 插件 | 公式 |
|---|---|
| fs | `files = file-nr[0]`；`inodes = inode-nr[0] - inode-nr[1]` |
| ipc | `/proc/sysvipc/{msg,sem,shm}` 行数 - 1 |
| lock | 统计 `POSIX`/`FLOCK`/`READ`/`WRITE` 出现次数 |
| socket | `sockstat` 第 3 列：sockets/TCP/UDP/RAW/FRAG 的 used 值；`other = tot-tcp-udp-raw-frg` |
| tcp | `st=0A→lis, 01→act, 02/03/09→syn, 06→tim, 04/05/07/08/0B→clo`（tcp + tcp6） |
| udp | `07→lis, 01→act` |
| unix | `Type=0002→dgm, 0001→str`；`St=01→lis, 03→act` |
| raw | `/proc/net/raw` 行数 - 1 |
| vm | `fnmatch` 匹配 `pgmajfault / pgfault / pgalloc_* / pgfree`，求和后 delta/elapsed |
| vm-adv | `pgsteal_* / pgscan_kswapd_* / pgscan_direct_* / pageoutrun / allocstall` |
| zones | `Node X, zone Y` 后的 `pages free`（→`Y_free`）与首个 `high`（→`Y_high`） |

---

## 9. 验证环境与方法

### 9.1 本机探测结果

| 项 | 结果 |
|---|---|
| Go | `E:\golang\soft\go1.27.1\bin\go.exe`（1.27.1） |
| Git | `C:\Program Files\Git\cmd\git.exe` |
| WSL | 有，`Ubuntu 22.04.1 LTS`，**WSL1**（kernel 4.4.0-19041，非 WSL2） |
| Python | 3.10.6；`six` 1.16.0 已存在；`imp` 可用 |
| dstat | 未安装（Ubuntu 22.04 仓库不再提供 dstat） |

### 9.2 直接运行上游的障碍

1. **Python 3.10 不兼容**：`collections.Sequence`、`types.ListType/TupleType/StringType` 已移除 → 首次 `show()` 即 `AttributeError`。
2. **WSL1 的 /proc 不完整**：`/proc/uptime` 恒为 `0.00`（导致除零）、缺 `/proc/diskstats`、`/proc/vmstat`、`/proc/swaps`、`/proc/net/sockstat`、`/proc/net/unix`、`/proc/locks`、`/proc/zoneinfo`、`/proc/sys/fs/*`。
3. 无 python2、无 docker。

### 9.3 采用的验证方案（已落地）

**A. py3 兼容垫片运行器**（不改动上游仓库）

`run_dstat.py`：注入 `collections.Sequence/abc`、`types.ListType/TupleType/StringType` 别名后 `exec` 上游脚本；`dstat` 本身零修改。

**B. 夹具 /proc 根**（`DSTAT_PROC_ROOT`）

垫片同时劫持 `open/os.path.exists/glob.glob`，把 `/proc`、`/sys` 重定向到夹具根 `/tmp/procroot`，内含真实内核格式的 `stat / meminfo / vmstat / diskstats / swaps / net/dev / net/* / interrupts / locks / zoneinfo / sys/fs/* / sysvipc/*` 与固定 `uptime=123456.78`。

**C. 对照策略**

| 场景 | 方法 | 用途 |
|---|---|---|
| 数值等价（`delay count` = `1 1`） | 首样本 `elapsed=uptime`、`set1=0` → 结果由累计计数器唯一确定 | 逐字符比对数值、单位、宽度 |
| 零增量格式（`1 2`） | 夹具静态 → 第 2 行全 0 | 比对零值、单位占位、分隔符 |
| 表头/裁剪/颜色 | `script -q -c` 伪造 pty | 比对标题、副标题、ANSI 序列、中间刷新 |
| CSV | `-o` | 比对 CSV 头、标题行、数值精度 |
| 速率语义 | 独立手工计算期望值（不影响上游对照） | 验证 delta/elapsed 口径 |

### 9.4 已知基准输出（夹具，`count=1`）

```
$ dstat 1 1
You did not select any stats, using -cdngy by default.
--total-cpu-usage-- -dsk/total- -net/total- ---paging-- ---system--
usr sys idl wai stl| read  writ| recv  send|  in   out | int   csw 
 11   3  86   0   0| 450k 1090k|   0     0 |  40B  113B|  72    40 

$ dstat -c -C total,0 1 1
--total-cpu-usage-- -----cpu0-usage----
usr sys idl wai stl:usr sys idl wai stl
 11   3  86   0   0: 11   3  86   0   0

$ dstat -m 1 1
------memory-usage-----
 used  free  buff  cach
5809M 5112M  338M 4461M
```

### 9.5 数值自校验样例

| 项 | 推导 |
|---|---|
| disk read `450k` | (sda 98765432 + sdb 12345678 + loop0 8000) 扇区 ×512 / 123456.78 s = 460,833 B/s ÷1024 = 450.0 → `450k` |
| disk writ `1090k` | (234567890+34567890+16000)×512/123456.78 = 1,116,200 B/s ÷1024 = 1090.0 → `1090k` |
| page in `40B` | 1200 页 ×4096 / 123456.78 = 39.8 → `40` + 单位 `B` |
| sys int `72` | 8944980 / 123456.78 = 72.45 → `72` |
| mem used `5809M` | (16318456-5234568-345678-4567890-456789+234567) kB → 5809.0 M |
| load `12.3 8.90 5.67` | `fchg` 的 width=4 特例：`8.9` 先取整为 `9`（1 位）→ 保留 2 位小数 `8.90` |

---

## 10. Go 实现映射（后续阶段）

| 层 | 对应上游 | 说明 |
|---|---|---|
| CLI | `Options` | 自实现 getopt 兼容解析：短参数聚合、长参数唯一前缀匹配、`DSTAT_OPTS` |
| 插件接口 | `dstat` 基类 | `Name/Nick/Vars/Type/Width/Scale/Cols/Discover/Vars()/Extract()` 七要素 |
| 插件装配 | `main()` 装配循环 | 回退链 + `check/prepare` + `totlist/vislist` |
| 渲染 | `cprint/dchg/fchg/header/title/subtitle` | 1:1 移植，含单位、颜色、宽度计算 |
| 采样 | `perform()` | `loop/step/elapsed` 与 `set1` 边界更新规则 |
| CSV | `csvheader/showcsv` | 1:1 移植 |
| 数据源 | `/proc`、`/sys` | 可注入 proc 根（用于夹具回归测试） |

阶段划分（对应任务清单）：

1. 规范文档（本文）
2. Go 骨架 + 默认 5 组（cpu/disk/net/page/sys）→ 与 9.4 基准逐行一致
3. 参数解析（含 `-cdn` 组合、`--float/--integer/--bits/--noheaders/--noupdate/--output`）
4. 常用插件补齐（`-c -d -n -m -s -y -g -i -l -p -r -t`）
5. 其余内部插件 + 表格/CSV/颜色/终端裁剪
6. 夹具回归测试（Go 与上游输出 diff）

---

## 11. 差异与风险清单

| # | 风险 | 说明 / 对策 |
|---|---|---|
| 1 | Python2 整除语义 | 上游 `loop`、`dev()` 依赖整数除法；Go 用整数运算复刻 |
| 2 | `--list`/`-V` 中的外部插件清单 | 依赖文件系统扫描；Go 版可列出内置清单并扫描插件目录 |
| 3 | 外部 Python 插件不可移植 | 明确不支持，`--<外部插件>` 报 `failed to load` 并保持退出码语义 |
| 4 | WSL1 /proc 不完整 | 用夹具回归；真实数据验证需真机 Linux 或 WSL2 |
| 5 | 终端相关行为（颜色/裁剪/中间刷新） | 需 pty 实测比对，不能只做管道输出比对 |
| 6 | 上游已知缺陷（如 `int` 插件边遍历边 `remove`） | 为保持输出一致，**照抄行为**，不做「修正」 |
| 7 | 首行含义 | loop 0 = 开机以来均值（net 恒 0），不要改成 1 秒增量 |
| 8 | `--noheaders` 语义 | 只关闭重复表头，首行表头仍打印 |

---

## 12. 验证结果（Go 实现 vs 上游 0.8.0）

> 真机验证（aarch64 / 64K 页真实 `/proc`，27 个用例）详见 [verification-real.md](verification-real.md)。

验证方式见 9.3：上游脚本由 py3 兼容垫片（注入 `collections.Sequence`、`types.ListType/TupleType/StringType`、
`DSTAT_PROC_ROOT` 路径劫持、退出时 flush）在 WSL1 Ubuntu 22.04 + Python 3.10.6 下运行，
Go 版交叉编译为 linux/amd64，两者读同一份夹具 proc 根（`/tmp/procroot`，`uptime=123456.78`）。

### 12.1 覆盖范围与结果

| 批次 | 用例数 | 结果 | 内容 |
|---|---|---|---|
| 纯文本（非 tty） | 80 | 79 通过 / 1 已知差异 | 默认、`-c -d -n -g -y -m -s -p -r -l -t -T -i`、`-a/-v`、组合短参数 `-cdn/-cdm/-dfn/-ni/-df`、`-C/-D/-N/-S`（含 `total`/`all`/设备名）、`--float/--integer/--bits/--noheaders/--noupdate`、全部内部长选项（`--cpu-adv/--cpu-use/--mem-adv/--swap-old/--fs/--ipc/--lock/--socket/--tcp/--udp/--unix/--raw/--vm/--vm-adv/--zones/--aio/--page24/--cpu24/--int24/--disk24/--disk24-old`）、`-i -I 16,25`、`-d -D total`、`-n -N total,eth0,lo` 等 |
| CSV（`-o`） | 6 | 6 通过 | `-cdn`、`-a`、`-v 2 2`、`--float -cdm`、`-t`、`-T` 的 CSV 文件逐行一致（归一化时间/日期/输出路径） |
| tty（pty，伪终端） | 8 | 8 通过 | 120/60/200 列、`--color`、`--nocolor`、`--noheaders`、`--noupdate`、`--black-on-white`：含 ANSI 颜色、`\r` 清行/保存恢复序列、行尾 `theme['input']`、退出时 `\x1b[0;0m` 复位 |
| CLI 元信息 | 10 | 9 通过 / 1 已知差异 | `-h`（尾部空行）、`--list`（`internal:\n\t` 折行格式）、`-V`、`--badopt`（`option --badopt not recognized` + 退出码 1）、`-x`、`--float --integer` 互斥、`0 1`、`-c abc`、`DSTAT_OPTS="-c"` |

### 12.2 已知差异（有意保留）

| 差异 | 上游行为 | Go 行为 | 原因 |
|---|---|---|---|
| `-i -I <网卡名>`（如 `eth0`） | `dstat_int24` 抛 `TypeError: argument of type 'method' is not iterable`（py3；py2 为 `instancemethod`） | `Module dstat_int24 failed to load. (No counter objects to monitor)` | 上游缺陷：`dstat_int24.__init__` 漏了 `self.intmap = self.intmap()`，`vars()` 里把方法当字典用。异常文本随 Python 版本变化，无法用单一字符串对齐，故不复刻 |
| `-V` 的 `Python ...` 行 | `Python 3.10.6 (main, ...) [GCC 11.3.0]` | `Python (go port)` | 实现语言不同，无对应值；其余行（Platform/Kernel/Terminal/Processors/Pagesize/Clock ticks/插件清单）已对齐 |
| `-o` CSV | 直接运行 0.8.0 会 `NameError: name 'types' is not defined`（源码未 `import types`） | 正常输出 CSV | 上游缺陷；Go 按「注释与文档描述的预期格式」实现，并与垫片修复后的上游输出逐行一致 |
| 外部插件（`plugins/dstat_*.py`） | 扫描 4 个 pluginpath 动态加载 | 不支持，`--外部插件名` 报 `option --xxx not recognized` | Python 插件无法在 Go 中执行；`--list` 仅列内置插件 |
| `--disk24-old`（旧版 `disk_io`） | discover/extract 引用了未定义变量 `line`，`/proc/stat` 含 `disk_io` 时抛 `NameError` | 按旧版 `disk_io` 格式正常解析 | 上游缺陷；现代内核无 `disk_io` 行，两者表现一致（均报 `No suitable block devices found to monitor`） |
| `--profile` | `profile.run()` 输出性能统计 | stderr 提示不支持 | Go 无对应 profiler 接口 |

### 12.3 复现命令

```bash
# Go 侧交叉编译
cd e:/golang/source/dstat
GOOS=linux GOARCH=amd64 go build -o /tmp/dstat_go .

# 夹具 + 对照（脚本在临时目录，不入库）
export DSTAT_PROC_ROOT=/tmp/procroot
compare2.sh c 0 80    # 文本对照
cmpcsv.sh             # CSV + pty 对照
cmpcli.sh             # -h/-V/--list/错误分支对照
```

