# dstat（Go 版）

中文 | [English](README.md)

用 Go 从头实现的 [dstat](https://github.com/dstat-real/dstat) 0.8.0。命令行用法与屏幕输出
以上游为准：参数（含组合短参数）、列顺序、表头、单位、颜色、缩放规则、`--float`/`--integer`/`--bits`、
`--output` CSV、`--pidfile`、`-h`、`-V`、`--list` 均与上游对齐。

产物是单个静态二进制：运行时不需要 Python，也不需要 plugins 目录，读取的仍是上游使用的
Linux `/proc`、`/sys` 接口。

* 上游项目（唯一权威参考）：<https://github.com/dstat-real/dstat>
* 上游作者：Dag Wieers 及贡献者，Copyright 2004-2019
* 本项目为独立重写，与上游无隶属关系
* 许可：**GPL-2.0**（见 [LICENSE](LICENSE)），与上游一致

## 当前状态

* 仅支持 Linux，数据全部来自 `/proc` 与 `/sys`（不抓取 `/dev`，不调用外部命令）。
* 已真机验证：aarch64、银河麒麟 V10 SP1、内核 `4.19.90-25.46.v2101.ky10.aarch64`、64 KiB 页 ——
  **27/27 个对照用例与上游 0.8.0 结构一致**（默认 5 组、常用插件、组合短参数、
  `--float`/`--integer`/`--noheaders`/`--noupdate`、CSV 输出）。详见
  [docs/verification-real.md](docs/verification-real.md)。
* 完整行为规范（参数、插件、公式、格式）：[docs/dstat-spec.md](docs/dstat-spec.md)。

## 环境要求

| | |
|---|---|
| 构建 | Go 1.21 或更高 |
| 运行 | 带 `/proc`、`/sys` 的 Linux；静态链接，不依赖 libc |

## 安装

预编译包通过 GitHub Pages 发布：

| 格式 | 仓库地址 |
|---|---|
| RPM（dnf / yum） | <https://yupor5.github.io/dstat/rpm/> |
| DEB（apt / apt-get） | <https://yupor5.github.io/dstat/deb/> |

包名为 **dstat-go**，安装到 `/usr/bin/dstat-go`，不会覆盖系统自带的
`/usr/bin/dstat`（麒麟上该文件是 Performance Co-Pilot 的包装脚本），可与发行版
`dstat` 包共存。支持架构：RPM 为 `x86_64`、`aarch64`、`i686`、`armv7hl`；DEB 为
`amd64`、`arm64`、`i386`、`armhf`。

### RPM（dnf / yum）

```bash
sudo curl -fsSL https://yupor5.github.io/dstat/dstat-go.repo -o /etc/yum.repos.d/dstat-go.repo
sudo dnf install dstat-go          # 老版本系统用 yum install dstat-go
sudo dnf upgrade dstat-go          # 后续升级
```

`/etc/yum.repos.d/dstat-go.repo` 内容：

```ini
[dstat-go]
name=dstat-go - Go implementation of dstat
baseurl=https://yupor5.github.io/dstat/rpm/
enabled=1
gpgcheck=0
repo_gpgcheck=0
sslverify=1
```

### DEB（apt / apt-get）

```bash
echo "deb [trusted=yes] https://yupor5.github.io/dstat/deb stable main" \
  | sudo tee /etc/apt/sources.list.d/dstat-go.list
sudo apt-get update
sudo apt-get install dstat-go
sudo apt-get install --only-upgrade dstat-go
```

### 仓库签名

两个仓库目前均为**未签名**：RPM 侧使用 `gpgcheck=0`，APT 侧使用 `[trusted=yes]`，
因此无需导入公钥。若需要签名元数据，必须向发布流程提供 GPG 私钥；工作流不会自行
生成或保存任何密钥。

### 直接下载

也可以直接从仓库目录获取 `.deb` / `.rpm` 手动安装：

```bash
sudo rpm -Uvh dstat-go-1.0.0-1.x86_64.rpm
sudo dpkg -i dstat-go_1.0.0-1_amd64.deb
```

## 构建

```bash
git clone https://github.com/yupor5/dstat.git
cd dstat
go build -o dstat .

# 或使用 make
make build          # linux/amd64
make build-all      # 全部发布目标，输出到 dist/
make package        # build-all 并打包 tar.gz
```

交叉编译（Go 标准机制，`CGO_ENABLED=0`）：

```bash
GOOS=linux GOARCH=amd64            go build -trimpath -ldflags "-s -w" -o dstat .
GOOS=linux GOARCH=arm64            go build -trimpath -ldflags "-s -w" -o dstat .
GOOS=linux GOARCH=386              go build -trimpath -ldflags "-s -w" -o dstat .
GOOS=linux GOARCH=arm GOARM=7      go build -trimpath -ldflags "-s -w" -o dstat .
GOOS=linux GOARCH=ppc64le          go build -trimpath -ldflags "-s -w" -o dstat .
GOOS=linux GOARCH=s390x            go build -trimpath -ldflags "-s -w" -o dstat .
```

推送 `v*` 标签时，[.github/workflows/release.yml](.github/workflows/release.yml) 会自动构建
各架构产物：`dstat-<version>-linux-<arch>.tar.gz`，内含二进制、两份 README、`LICENSE` 与 `docs/`。

## 用法

通过安装包安装后命令名为 `dstat-go`；下文示例为简洁写成 `dstat`
（`dstat-go -cdn 2` 与 `dstat -cdn 2` 行为完全一致）。

```
dstat [-afv] [options..] [delay [count]]
```

```bash
dstat                       # 默认 -cdngy，1 秒间隔，持续输出
dstat 5                     # 5 秒间隔
dstat 5 3                   # 5 秒间隔，输出 3 次
dstat -cdn 2                # cpu、disk、net，每 2 秒（组合短参数）
dstat -c -d -n -m -s -y     # 显式选择插件
dstat -a                    # 等价 -cdngy
dstat -v                    # 类 vmstat：proc、mem、page、disk、sys、cpu
dstat --float -cdm          # 强制浮点输出
dstat --integer -cdm        # 强制整数输出
dstat --noheaders -cdn      # 不重复表头
dstat --noupdate -cd 2 3    # 只打印一次表头，之后仅数据行
dstat -o stats.csv 1 10     # 写 CSV（同时保留屏幕输出）
dstat --pidfile /run/dstat.pid
dstat --list                # 列出内置插件
dstat -h                    # 帮助
dstat -V                    # 版本 / 平台 / 终端 / 计数信息
dstat --bits -dn            # 网络与磁盘按 bit 显示
dstat -d -f                 # 每块磁盘单独一组列
dstat -n -N total,eth0      # 限定/排序网卡
dstat -c -C 0,3,total       # 限定 CPU
```

环境变量：`DSTAT_OPTS` 会拼接到命令行前（与上游一致）。

## 已支持参数

| 短参数 | 长参数 | 含义 |
|---|---|---|
| `-c` | `--cpu` | CPU 使用率（`-C 0,3,total` / `-C all`） |
| `-d` | `--disk` | 磁盘 I/O（`-D total,sda`） |
| `-g` | `--page` | 换页活动 |
| `-i` | `--int` | 中断（`-I 5,eth2`） |
| `-l` | `--load` | 负载均值 |
| `-m` | `--mem` | 内存使用（`-M`、`--mods`、`--modules` 为已废弃别名） |
| `-n` | `--net` | 网络 I/O（`-N eth1,total`） |
| `-p` | `--proc` | 进程计数 |
| `-r` | `--io` | 已完成的 I/O 请求数 |
| `-s` | `--swap` | Swap 使用（`-S swap1,total`） |
| `-t` | `--time` | 日期时间列 |
| `-T` | `--epoch` | 纪元秒数列 |
| `-y` | `--sys` | 系统统计（中断、上下文切换） |
| `-a` | `--all` | 等价 `-cdngy` |
| `-v` | `--vmstat` | proc、mem、page、disk、sys、cpu |
| `-f` | `--full` | 展开所有设备/网卡，而非 `total` |
| | `--bits` | 速率按 bit/s 显示 |
| | `--bw`、`--black-on-white` | 浅色终端配色 |
| | `--color`、`--nocolor` | 强制开/关颜色 |
| | `--debug` | 调试输出（可叠加） |
| | `--float`、`--integer` | 强制数字格式（互斥） |
| | `--noheaders` | 不重复列表头 |
| | `--noupdate` | 不重复刷新表头 |
| `-o` | `--output FILE` | 追加 CSV 输出到文件 |
| | `--pidfile FILE` | 写入进程号 |
| | `--profile` | 接受参数，但提示不支持 |
| | `--all-plugins` | 启用全部内置插件 |
| | `--list` | 列出插件 |
| `-h` | `--help` | 帮助（版式与上游一致） |
| `-V` | `--version` | 版本信息 |

## 内置插件

共 34 个内部插件，名称与长参数与上游一致。可用长参数（如 `--cpu24`、`--page24`）或上表中的短参数选择。

| 插件 | 选择方式 | 说明 |
|---|---|---|
| `cpu` | `-c`、`--cpu` | 总 CPU：usr、sys、idl、wai、stl（`--cpu-adv` 增加 hld、guest） |
| `cpu-adv` | `--cpu-adv` | 带硬中断与 guest 时间的 CPU 使用 |
| `cpu-use` | `--cpu-use` | 各 CPU 使用率 |
| `cpu24` | `--cpu24` | 2.4 风格 CPU 行 |
| `disk` | `-d`、`--disk` | 磁盘读写吞吐 |
| `disk24` | `--disk24` | 2.4 风格磁盘 I/O |
| `disk24-old` | `--disk24-old` | 旧版 `/proc/stat` 中的 `disk_io` |
| `page` | `-g`、`--page` | 换页 in/out（`--page24` 为缺页统计） |
| `page24` | `--page24` | 2.4 风格换页 |
| `int` | `-i`、`--int` | 每秒硬中断 |
| `int24` | `--int24` | 2.4 风格中断 |
| `io` | `-r`、`--io` | 已完成块 I/O 请求数 |
| `load` | `-l`、`--load` | 1/5/15 分钟负载 |
| `mem` | `-m`、`--mem` | used、free、buff、cach、avail |
| `mem-adv` | `--mem-adv` | 更细的内存计数 |
| `net` | `-n`、`--net` | 网络收发 |
| `proc` | `-p`、`--proc` | runnable、uninterruptible、new |
| `swap` | `-s`、`--swap` | Swap 使用/空闲 |
| `swap-old` | `--swap-old` | 旧版 swap 计数 |
| `sys` | `-y`、`--sys` | 中断与上下文切换 |
| `time` | `-t`、`--time` | 当前日期时间 |
| `epoch` | `-T`、`--epoch` | 纪元秒数 |
| `aio` | `--aio` | 异步 I/O 计数 |
| `fs` | `--filesystem`、`--fs` | 打开文件数、inode |
| `ipc` | `--ipc` | 消息队列、信号量、共享内存 |
| `lock` | `--lock` | 文件锁 |
| `raw` | `--raw` | 原始套接字计数 |
| `socket` | `--socket` | 按状态/协议的套接字计数 |
| `tcp` | `--tcp` | TCP 套接字状态 |
| `udp` | `--udp` | UDP 套接字 |
| `unix` | `--unix` | UNIX 域套接字 |
| `vm` | `--vm` | 虚拟内存统计 |
| `vm-adv` | `--vm-adv` | 更细的虚拟内存统计 |
| `zones` | `--zones` | Zone CPU/内存（上游仅 Solaris 有效） |

## 输出格式说明

* 第一行数据（`loop 0`）是**开机以来的平均值**（上游语义），不是 1 秒增量，因此网络速率为 `0`。
* 表头默认重复；`--noheaders` 只关闭重复部分，首个表头块仍会打印。
* 超宽行按终端宽度拆分，列之间用 `|` 分隔，与上游一致。
* `--output` CSV：文件先写 4 行前言（`Dstat 0.8.0 CSV output`、作者、主机/用户、命令/日期），
  随后是每个插件的表头行与每次采样的数据行；追加写入已有文件时会插入空行分隔。
* 输出非终端时自动关闭颜色、表头和刷新，与上游相同。

## 平台支持

| 系统 | 状态 |
|---|---|
| Linux（任何提供 `/proc` 的架构） | 支持；发布 amd64、arm64、386、arm(v7)、ppc64le、s390x |
| macOS / BSD / Windows | 不支持（无 `/proc`，上游同样只面向 Linux） |

## 已知限制

| 项 | 行为 |
|---|---|
| 外部 Python 插件（`plugins/dstat_*.py`） | 未实现。指定时输出 `option --<name> not recognized`，与上游对未知长参数的处理一致。 |
| `--profile` | 接受参数并提示不支持；Go 无对应的 Python profiler。 |
| `-V` 细节 | 解释器版本行显示 `Python (go port)`；其余各行与上游一致。 |
| `-i -I <网卡名>` | 上游 `dstat_int24` 存在缺陷（把方法当字典用）会崩溃；本实现输出 `No counter objects to monitor`。 |
| `--disk24-old` | 与上游一致：内核 `/proc/stat` 无 `disk_io` 时输出 `No suitable block devices found to monitor`。 |
| `-o` CSV 与上游 0.8.0 的差异 | 上游写数据行时因 `NameError: types` 崩溃；本实现按上述格式完整写出 CSV。 |
| 版本号 | 有意报告 `Dstat 0.8.0`，保证 `-V` 输出可与上游对照。 |

## 验证情况

两层验证，均为「同一时刻并发启动上游与本实现，按结构而非精确数值比对」（采样时刻不同必然有偏差）：

1. WSL1 Ubuntu 22.04 + Python 3.10 夹具对照：80 个纯文本用例、6 个 CSV、8 个伪终端、10 个 CLI 用例。
2. 真机（aarch64，64 KiB 页）：24 个纯文本 + 3 个 CSV 用例，全部结构一致 ——
   列数/顺序、`|` 位置、行宽、表头、单位、量级与趋势均对齐。

报告：[docs/verification-real.md](docs/verification-real.md)（真机）、
[docs/dstat-spec.md](docs/dstat-spec.md) 第 12 节（夹具）。

## 许可

GNU General Public License v2.0，见 [LICENSE](LICENSE)。该许可继承自本项目所复刻的上游项目。
