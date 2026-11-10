# dstat (Go)

[中文](README.zh-CN.md) | English

A from-scratch Go implementation of [dstat](https://github.com/dstat-real/dstat) 0.8.0.
The command-line syntax and the on-screen output are written to match upstream: same
options (including combined short flags), same column order, headers, units, colors,
scaling, `--float`/`--integer`/`--bits`, `--output` CSV, `--pidfile`, `-h`, `-V`, `--list`.

The result is a single static binary with no runtime dependencies (no Python, no plugins
directory) that reads the same Linux `/proc` and `/sys` interfaces upstream reads.

* Upstream project (authoritative reference): <https://github.com/dstat-real/dstat>
* Upstream author: Dag Wieers and contributors, Copyright 2004-2019
* This project is an independent reimplementation, not affiliated with upstream
* License: **GPL-2.0** (see [LICENSE](LICENSE)), same as upstream

## Status

* Linux only. All data comes from `/proc` and `/sys` (no `/dev` scraping, no external commands).
* Field-verified on a real machine: aarch64, Kylin V10 SP1, kernel
  `4.19.90-25.46.v2101.ky10.aarch64`, 64 KiB pages — **27/27 comparison cases structurally
  identical to upstream 0.8.0** (defaults, common plugins, combined short options,
  `--float`/`--integer`/`--noheaders`/`--noupdate`, CSV output). Details:
  [docs/verification-real.md](docs/verification-real.md).
* Full behaviour specification (options, plugins, formulas, formats):
  [docs/dstat-spec.md](docs/dstat-spec.md).

## Requirements

| | |
|---|---|
| Build | Go 1.21 or newer |
| Run | Linux kernel with `/proc` and `/sys`; statically linked binary, no libc needed |

## Install

Prebuilt packages are published from GitHub Pages:

| Format | Repository URL |
|---|---|
| RPM (dnf / yum) | <https://yupor5.github.io/dstat/rpm/> |
| DEB (apt / apt-get) | <https://yupor5.github.io/dstat/deb/> |

The package is named **dstat-go** and installs `/usr/bin/dstat-go`. It never
touches `/usr/bin/dstat` (on Kylin that file is a Performance Co-Pilot wrapper
script), so it can be installed next to the distribution `dstat` package.
Supported architectures: RPM `x86_64`, `aarch64`, `i686`, `armv7hl`; DEB
`amd64`, `arm64`, `i386`, `armhf`.

### RPM (dnf / yum)

```bash
sudo curl -fsSL https://yupor5.github.io/dstat/dstat-go.repo -o /etc/yum.repos.d/dstat-go.repo
sudo dnf install dstat-go          # yum install dstat-go on older releases
sudo dnf upgrade dstat-go          # later upgrades
```

`/etc/yum.repos.d/dstat-go.repo`:

```ini
[dstat-go]
name=dstat-go - Go implementation of dstat
baseurl=https://yupor5.github.io/dstat/rpm/
enabled=1
gpgcheck=0
repo_gpgcheck=0
sslverify=1
```

### DEB (apt / apt-get)

```bash
echo "deb [trusted=yes] https://yupor5.github.io/dstat/deb stable main" \
  | sudo tee /etc/apt/sources.list.d/dstat-go.list
sudo apt-get update
sudo apt-get install dstat-go
sudo apt-get install --only-upgrade dstat-go
```

### Repository signing

Both repositories are currently **unsigned**: the RPM repository uses
`gpgcheck=0` and the APT source `[trusted=yes]`, so there is no public key to
import. Signed metadata requires a GPG private key for the release workflow;
the workflow never generates or stores keys on its own.

### Direct download

`.deb` and `.rpm` files can also be fetched from the repository directories or
installed by hand:

```bash
sudo rpm -Uvh dstat-go-1.0.0-1.x86_64.rpm
sudo dpkg -i dstat-go_1.0.0-1_amd64.deb
```

## Build

```bash
git clone https://github.com/yupor5/dstat.git
cd dstat
go build -o dstat .

# or with make
make build          # linux/amd64
make build-all      # all release targets into dist/
make package        # build-all + tar.gz archives
```

Cross-compilation (Go's standard mechanism, `CGO_ENABLED=0`):

```bash
GOOS=linux GOARCH=amd64            go build -trimpath -ldflags "-s -w" -o dstat .
GOOS=linux GOARCH=arm64            go build -trimpath -ldflags "-s -w" -o dstat .
GOOS=linux GOARCH=386              go build -trimpath -ldflags "-s -w" -o dstat .
GOOS=linux GOARCH=arm GOARM=7      go build -trimpath -ldflags "-s -w" -o dstat .
GOOS=linux GOARCH=ppc64le          go build -trimpath -ldflags "-s -w" -o dstat .
GOOS=linux GOARCH=s390x            go build -trimpath -ldflags "-s -w" -o dstat .
```

Release archives are produced automatically by
[.github/workflows/release.yml](.github/workflows/release.yml) when a `v*` tag is pushed:
each `dstat-<version>-linux-<arch>.tar.gz` contains the binary, both READMEs, `LICENSE`
and the `docs/` directory.

## Usage

Installed from a package the command is `dstat-go`; the examples below use `dstat`
for brevity (`dstat-go -cdn 2` behaves exactly like `dstat -cdn 2`).

```
dstat [-afv] [options..] [delay [count]]
```

```bash
dstat                       # default -cdngy, 1 second interval, forever
dstat 5                     # 5 second interval
dstat 5 3                   # 5 second interval, 3 updates
dstat -cdn 2                # cpu, disk, net every 2 seconds (combined short options)
dstat -c -d -n -m -s -y     # pick plugins explicitly
dstat -a                    # same as -cdngy
dstat -v                    # vmstat-like: proc, mem, page, disk, sys, cpu
dstat --float -cdm          # force floating point output
dstat --integer -cdm        # force integer output
dstat --noheaders -cdn      # do not repeat headers
dstat --noupdate -cd 2 3    # print one header, then only data rows
dstat -o stats.csv 1 10     # write CSV (plus normal screen output)
dstat --pidfile /run/dstat.pid
dstat --list                # list available internal plugins
dstat -h                    # help, up to and including external plugins
dstat -V                    # version / platform / terminal / counts
dstat --bits -dn            # network and disk in bits instead of bytes
dstat -d -f                 # one column group per block device
dstat -n -N total,eth0      # restrict/reorder network interfaces
dstat -c -C 0,3,total       # restrict CPU stats
```

Environment: `DSTAT_OPTS` is prepended to the command line (same as upstream).

## Supported options

| Short | Long | Meaning |
|---|---|---|
| `-c` | `--cpu` | CPU usage (`-C 0,3,total` / `-C all`) |
| `-d` | `--disk` | Disk I/O (`-D total,sda`) |
| `-g` | `--page` | Paging activity |
| `-i` | `--int` | Interrupts (`-I 5,eth2`) |
| `-l` | `--load` | Load average |
| `-m` | `--mem` | Memory usage (`-M`, `--mods`, `--modules` deprecated) |
| `-n` | `--net` | Network I/O (`-N eth1,total`) |
| `-p` | `--proc` | Process counts |
| `-r` | `--io` | I/O requests completed |
| `-s` | `--swap` | Swap usage (`-S swap1,total`) |
| `-t` | `--time` | Date/time column |
| `-T` | `--epoch` | Seconds since epoch column |
| `-y` | `--sys` | System stats (interrupts, context switches) |
| `-a` | `--all` | `-cdngy` |
| `-v` | `--vmstat` | proc, mem, page, disk, sys, cpu |
| `-f` | `--full` | Show all devices/interfaces instead of `total` |
| | `--bits` | Show rates in bits per second |
| | `--bw`, `--black-on-white` | Color theme for light terminals |
| | `--color`, `--nocolor` | Force color on/off |
| | `--debug` | Debug output (repeatable) |
| | `--float`, `--integer` | Force number format (mutually exclusive) |
| | `--noheaders` | Do not repeat column headers |
| | `--noupdate` | Do not redraw the header on every update |
| `-o` | `--output FILE` | Append CSV output to FILE |
| | `--pidfile FILE` | Write the process id to FILE |
| | `--profile` | Accepted, prints "not supported" |
| | `--all-plugins` | Enable every internal plugin |
| | `--list` | List plugins |
| `-h` | `--help` | Help text (same layout as upstream) |
| `-V` | `--version` | Version information |

## Built-in plugins

34 internal plugins, same names and long options as upstream. Each may be selected by its
long option (`--cpu24`, `--page24`, ...) or by the short flag shown above.

| Plugin | Selected by | Description |
|---|---|---|
| `cpu` | `-c`, `--cpu` | Total CPU usage: usr, sys, idl, wai, stl (hld, guest on `--cpu-adv`) |
| `cpu-adv` | `--cpu-adv` | CPU usage with hardware interrupts and guest time |
| `cpu-use` | `--cpu-use` | Per-CPU utilization |
| `cpu24` | `--cpu24` | Legacy 2.4 style CPU line |
| `disk` | `-d`, `--disk` | Disk read/write throughput |
| `disk24` | `--disk24` | Legacy 2.4 style disk I/O |
| `disk24-old` | `--disk24-old` | Legacy `disk_io` from `/proc/stat` |
| `page` | `-g`, `--page` | Page in/out (page faults on `--page24`) |
| `page24` | `--page24` | Legacy 2.4 style paging |
| `int` | `-i`, `--int` | Hardware interrupts per second |
| `int24` | `--int24` | Legacy 2.4 style interrupts |
| `io` | `-r`, `--io` | Block I/O requests completed |
| `load` | `-l`, `--load` | 1/5/15 minute load average |
| `mem` | `-m`, `--mem` | used, free, buff, cach, avail |
| `mem-adv` | `--mem-adv` | Memory usage with advanced counters |
| `net` | `-n`, `--net` | Network receive/send |
| `proc` | `-p`, `--proc` | runnable, uninterruptible, new |
| `swap` | `-s`, `--swap` | Swap used/free |
| `swap-old` | `--swap-old` | Legacy swap counters |
| `sys` | `-y`, `--sys` | Interrupts and context switches |
| `time` | `-t`, `--time` | Current date and time |
| `epoch` | `-T`, `--epoch` | Seconds since epoch |
| `aio` | `--aio` | Async I/O counters |
| `fs` | `--filesystem`, `--fs` | Open files, inodes |
| `ipc` | `--ipc` | Message queues, semaphores, shared memory |
| `lock` | `--lock` | File locks |
| `raw` | `--raw` | Raw socket counters |
| `socket` | `--socket` | Socket counters by state/protocol |
| `tcp` | `--tcp` | TCP socket states |
| `udp` | `--udp` | UDP sockets |
| `unix` | `--unix` | UNIX domain sockets |
| `vm` | `--vm` | Virtual memory stats |
| `vm-adv` | `--vm-adv` | Advanced virtual memory stats |
| `zones` | `--zones` | Zone CPU/memory (Solaris-only upstream) |

## Output format notes

* The first data row (`loop 0`) is the **average since boot** (upstream semantics), not a
  one-second delta. Network rates are therefore `0` on the first row.
* Column headers are repeated by default; `--noheaders` suppresses only the repetitions,
  the first header block is still printed.
* Wide rows are split by the terminal width, columns are separated by `|` as upstream does.
* `--output` CSV: the file gets a 4-line preamble (`Dstat 0.8.0 CSV output`, author,
  host/user, command/date) then one header row per plugin and one data row per sample.
  Appending to an existing file inserts a blank-line separator.
* When stdout is not a terminal, color is disabled and headers/updates are turned off,
  exactly like upstream.

## Platform support

| OS | Status |
|---|---|
| Linux (any architecture with `/proc`) | Supported; release builds for amd64, arm64, 386, arm(v7), ppc64le, s390x |
| macOS / BSD / Windows | Not supported (no `/proc`; upstream targets Linux too) |

## Known limitations

| Item | Behaviour |
|---|---|
| External Python plugins (`plugins/dstat_*.py`) | Not implemented. Selecting one reports `option --<name> not recognized`, the same message upstream prints for an unknown long option. |
| `--profile` | Accepted, prints a "not supported" notice; no Python profiler equivalent. |
| `-V` details | Prints `Python (go port)` in place of the interpreter version; all other lines match upstream. |
| `-i -I <interface>` | Upstream crashes in `dstat_int24` (uses a method as a dict); this port reports `No counter objects to monitor` instead. |
| `--disk24-old` | Mirrors upstream: on kernels without `disk_io` in `/proc/stat` it reports `No suitable block devices found to monitor`. |
| `-o` CSV vs upstream 0.8.0 | Upstream 0.8.0 crashes while writing data rows (`NameError: types`); this port writes the complete CSV described above. |
| Version string | Reports `Dstat 0.8.0` on purpose, so `-V` output stays comparable with upstream. |

## Verification

Two layers, both scripted as concurrent runs (upstream and this port started at the same
moment, compared by structure rather than exact values, since sampling instants differ):

1. Fixture-based comparison on WSL1 Ubuntu 22.04 + Python 3.10: 80 plain-text cases,
   6 CSV cases, 8 pseudo-terminal cases, 10 CLI cases.
2. Real machine (aarch64, 64 KiB pages): 24 plain-text cases + 3 CSV cases, all
   structurally identical — column count/order, `|` positions, row width, headers, units,
   magnitudes and trends.

Reports: [docs/verification-real.md](docs/verification-real.md) (real machine) and
section 12 of [docs/dstat-spec.md](docs/dstat-spec.md) (fixtures).

## License

GNU General Public License v2.0, see [LICENSE](LICENSE). The license is inherited from the
upstream project this port reproduces.
