package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------- 通用读取 ----------

func readLines(p string) ([]string, error) {
	return readLinesSep(p, "")
}

// readLinesSep 对应 dopen(path).readlines()；replace 对应 splitlines(replace=...)
func readLinesSep(p string, replace string) ([]string, error) {
	b, err := os.ReadFile(procPath(p))
	if err != nil {
		return nil, err
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil, nil
	}
	raw := strings.Split(s, "\n")
	out := make([]string, len(raw))
	for i, l := range raw {
		if replace != "" {
			l = strings.ReplaceAll(l, replace, " ")
		}
		out[i] = l
	}
	return out, nil
}

func linesOrEmpty(p string) []string {
	l, err := readLines(p)
	if err != nil {
		return nil
	}
	return l
}

func atoi(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

func atof(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func fnmatch(name, pattern string) bool {
	ok, err := path.Match(pattern, name)
	if err != nil {
		return false
	}
	return ok
}

var diskFilter = regexp.MustCompile(`^([hsv]d[a-z]+\d+|cciss/c\d+d\d+p\d+|dm-\d+|md\d+|mmcblk\d+p\d0|VxVM\d+)$`)
var netTotalFilter = regexp.MustCompile(`^(lo|bond\d+|face|.+\.\d+)$`)

func blockdevices() []string {
	dir := procPath("/sys/block")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		out = append(out, strings.ReplaceAll(e.Name(), "!", "/"))
	}
	return out
}

// sysfsDev 对应上游 sysfs_dev()：把 sysfs 设备路径转换成短设备名
func sysfsDev(device string) string {
	if m := regexp.MustCompile(`^ide/host(\d)/bus(\d)/target(\d)/lun(\d)/disc$`).FindStringSubmatch(device); m != nil {
		nr := atoi(m[2])*2 + atoi(m[4])
		return "hd" + string(rune('a'+nr))
	}
	if m := regexp.MustCompile(`^cciss/(c\dd\d)`).FindStringSubmatch(device); m != nil {
		return m[1]
	}
	if strings.HasPrefix(device, "placeholder") {
		return "sdX"
	}
	return device
}

func improve(devname string) string {
	parts := strings.Split(devname, "/")
	if strings.HasPrefix(devname, "/dev/mapper/") && len(parts) > 3 {
		return parts[3]
	}
	if strings.HasPrefix(devname, "/dev/") && len(parts) > 2 {
		return parts[2]
	}
	return devname
}

func setVecCopy(dst map[string]any, src map[string]any, name string) {
	v := vec(src[name])
	cp := make([]float64, len(v))
	copy(cp, v)
	dst[name] = cp
}

// ---------- 注册表 ----------

func newPluginByName(name string) *Plugin {
	switch name {
	case "aio":
		return newAio()
	case "cpu":
		return newCPU(5)
	case "cpu-adv":
		return newCPU(7)
	case "cpu-use":
		return newCPUUse()
	case "cpu24":
		return newCPU24()
	case "disk":
		return newDisk()
	case "disk24":
		return newDisk24()
	case "disk24-old":
		return newDisk24Old()
	case "epoch":
		return newEpoch()
	case "fs":
		return newFS()
	case "int":
		return newInt()
	case "int24":
		return newInt24()
	case "io":
		return newIO()
	case "ipc":
		return newIPC()
	case "load":
		return newLoad()
	case "lock":
		return newLock()
	case "mem":
		return newMem()
	case "mem-adv":
		return newMemAdv()
	case "net":
		return newNet()
	case "page":
		return newPage()
	case "page24":
		return newPage24()
	case "proc":
		return newProc()
	case "raw":
		return newRaw()
	case "socket":
		return newSocket()
	case "swap":
		return newSwap()
	case "swap-old":
		return newSwapOld()
	case "sys":
		return newSys()
	case "tcp":
		return newTCP()
	case "time":
		return newTime()
	case "udp":
		return newUDP()
	case "unix":
		return newUnix()
	case "vm":
		return newVM()
	case "vm-adv":
		return newVMAdv()
	case "zones":
		return newZones()
	}
	return nil
}

// ---------- aio ----------

func newAio() *Plugin {
	p := newPlugin()
	p.Name = "async"
	p.Nick = []string{"#aio"}
	p.Vars = []string{"aio"}
	p.Type = "d"
	p.Width = 5
	p.Scale = 1000
	p.Files = []string{"/proc/sys/fs/aio-nr"}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/sys/fs/aio-nr") {
			f := strings.Fields(l)
			if len(f) < 1 {
				continue
			}
			p.Val["aio"] = float64(atoi(f[0]))
		}
	}
	return p
}

// ---------- cpu ----------

func cpuVars(p *Plugin) []string {
	var varlist []string
	switch {
	case len(E.op.cpulist) > 0 && containsStr(E.op.cpulist, "all"):
		for i := 0; i < E.cpunr; i++ {
			varlist = append(varlist, strconv.Itoa(i))
		}
	case len(E.op.cpulist) > 0:
		varlist = E.op.cpulist
	default:
		varlist = []string{"total"}
	}
	var ret []string
	for _, name := range varlist {
		if containsStr(p.discovered, name) || name == "total" {
			ret = append(ret, name)
		}
	}
	return ret
}

func cpuName(p *Plugin) []string {
	var ret []string
	for _, name := range p.Vars {
		if name == "total" {
			ret = append(ret, "total cpu usage")
		} else {
			ret = append(ret, "cpu"+name+" usage")
		}
	}
	return ret
}

func cpuDiscover(p *Plugin) []string {
	var ret []string
	for _, l := range linesOrEmpty("/proc/stat") {
		f := strings.Fields(l)
		if len(f) < 9 || !strings.HasPrefix(f[0], "cpu") {
			continue
		}
		ret = append(ret, f[0][3:])
	}
	sortStrings(ret)
	return ret
}

func newCPU(cols int) *Plugin {
	p := newPlugin()
	p.Type = "p"
	p.Width = 3
	p.Scale = 34
	p.Cols = cols
	p.Files = []string{"/proc/stat"}
	if cols == 7 {
		p.Nick = []string{"usr", "sys", "idl", "wai", "hiq", "siq", "stl"}
	} else {
		p.Nick = []string{"usr", "sys", "idl", "wai", "stl"}
	}
	p.DiscoverFn = cpuDiscover
	p.VarsFn = cpuVars
	p.NameFn = cpuName
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/stat") {
			f := strings.Fields(l)
			if len(f) < 9 {
				continue
			}
			for _, name := range p.Vars {
				if f[0] == "cpu"+name || (f[0] == "cpu" && name == "total") {
					if p.Cols == 7 {
						p.Set2[name] = []float64{
							float64(atoi(f[1]) + atoi(f[2])), float64(atoi(f[3])),
							float64(atoi(f[4])), float64(atoi(f[5])),
							float64(atoi(f[6])), float64(atoi(f[7])),
							float64(atoi(f[8])),
						}
					} else {
						p.Set2[name] = []float64{
							float64(atoi(f[1]) + atoi(f[2]) + atoi(f[6]) + atoi(f[7])),
							float64(atoi(f[3])), float64(atoi(f[4])),
							float64(atoi(f[5])), float64(atoi(f[8])),
						}
					}
				}
			}
		}
		for _, name := range p.Vars {
			s2 := vec(p.Set2[name])
			s1 := vec(p.Set1[name])
			v := vec(p.Val[name])
			if sumFloat(s2) > sumFloat(s1) {
				den := sumFloat(s2) - sumFloat(s1)
				for i := range v {
					v[i] = 100.0 * (s2[i] - s1[i]) / den
				}
			} else {
				for i := range v {
					v[i] = 0
				}
			}
		}
		if E.step == E.op.delay {
			for _, name := range p.Vars {
				setVecCopy(p.Set1, p.Set2, name)
			}
		}
	}
	return p
}

func newCPUUse() *Plugin {
	p := newPlugin()
	p.Name = "per cpu usage"
	p.Type = "p"
	p.Width = 3
	p.Scale = 34
	p.Cols = 7
	p.Files = []string{"/proc/stat"}
	p.DiscoverFn = cpuDiscover
	p.VarsFn = func(p *Plugin) []string {
		if len(E.op.cpulist) == 0 {
			var vars []string
			for i := 0; i < E.cpunr; i++ {
				vars = append(vars, strconv.Itoa(i))
			}
			return vars
		}
		return cpuVars(p)
	}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/stat") {
			f := strings.Fields(l)
			if len(f) < 9 {
				continue
			}
			for _, name := range p.Vars {
				if f[0] == "cpu"+name || (f[0] == "cpu" && name == "total") {
					p.Set2[name] = []float64{
						float64(atoi(f[1]) + atoi(f[2])), float64(atoi(f[3])),
						float64(atoi(f[4])), float64(atoi(f[5])),
						float64(atoi(f[6])), float64(atoi(f[7])),
						float64(atoi(f[8])),
					}
				}
			}
		}
		for _, name := range p.Vars {
			s2 := vec(p.Set2[name])
			s1 := vec(p.Set1[name])
			if sumFloat(s2) > sumFloat(s1) {
				den := sumFloat(s2) - sumFloat(s1)
				p.Val[name] = 100.0 - 100.0*(s2[2]-s1[2])/den
			} else {
				p.Val[name] = float64(0)
			}
		}
		if E.step == E.op.delay {
			for _, name := range p.Vars {
				setVecCopy(p.Set1, p.Set2, name)
			}
		}
	}
	return p
}

func newCPU24() *Plugin {
	p := newPlugin()
	p.Type = "p"
	p.Width = 3
	p.Scale = 34
	p.Cols = 3
	p.Files = []string{"/proc/stat"}
	p.Nick = []string{"usr", "sys", "idl"}
	p.DiscoverFn = func(p *Plugin) []string {
		var ret []string
		for _, l := range linesOrEmpty("/proc/stat") {
			f := strings.Fields(l)
			if len(f) < 5 || !strings.HasPrefix(f[0], "cpu") {
				continue
			}
			ret = append(ret, f[0][3:])
		}
		sortStrings(ret)
		return ret
	}
	p.VarsFn = cpuVars
	p.NameFn = func(p *Plugin) []string {
		var ret []string
		for _, name := range p.Vars {
			if name == "total" {
				ret = append(ret, "cpu usage")
			} else {
				ret = append(ret, "cpu"+name)
			}
		}
		return ret
	}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/stat") {
			f := strings.Fields(l)
			if len(f) < 5 {
				continue
			}
			for _, name := range p.Vars {
				if f[0] == "cpu"+name || (f[0] == "cpu" && name == "total") {
					p.Set2[name] = []float64{
						float64(atoi(f[1]) + atoi(f[2])), float64(atoi(f[3])), float64(atoi(f[4])),
					}
				}
			}
		}
		for _, name := range p.Vars {
			s2 := vec(p.Set2[name])
			s1 := vec(p.Set1[name])
			v := vec(p.Val[name])
			if sumFloat(s2) > sumFloat(s1) {
				den := sumFloat(s2) - sumFloat(s1)
				for i := range v {
					v[i] = 100.0 * (s2[i] - s1[i]) / den
				}
			} else {
				for i := range v {
					v[i] = 0
				}
			}
		}
		if E.step == E.op.delay {
			for _, name := range p.Vars {
				setVecCopy(p.Set1, p.Set2, name)
			}
		}
	}
	return p
}

// ---------- disk ----------

func diskLines() [][]string {
	var out [][]string
	for _, l := range linesOrEmpty("/proc/diskstats") {
		f := strings.Fields(l)
		if len(f) < 13 {
			continue
		}
		out = append(out, f)
	}
	return out
}

func diskDiscover(p *Plugin) []string {
	var ret []string
	for _, f := range diskLines() {
		if isAllZero(f[3:], 11) {
			continue
		}
		ret = append(ret, f[2])
	}
	if len(ret) == 0 {
		p.loadErr = fmt.Errorf("No suitable block devices found to monitor")
	}
	return ret
}

func isAllZero(f []string, n int) bool {
	if len(f) != n {
		return false
	}
	for _, v := range f {
		if v != "0" {
			return false
		}
	}
	return true
}

func diskVars(p *Plugin, basename bool) []string {
	var varlist []string
	switch {
	case len(E.op.disklist) > 0:
		for _, d := range E.op.disklist {
			if basename {
				varlist = append(varlist, diskBasename(d))
			} else {
				varlist = append(varlist, d)
			}
		}
	case !E.op.full:
		varlist = []string{"total"}
	default:
		bd := blockdevices()
		for _, name := range p.discovered {
			if diskFilter.MatchString(name) {
				continue
			}
			if !containsStr(bd, name) {
				continue
			}
			varlist = append(varlist, name)
		}
		sortStrings(varlist)
	}
	var ret []string
	for _, name := range varlist {
		if containsStr(p.discovered, name) || name == "total" {
			ret = append(ret, name)
		}
	}
	return ret
}

func diskBasename(disk string) string {
	if strings.HasPrefix(disk, "/dev/") {
		if _, err := os.Stat(disk); err == nil {
			if fi, err := os.Lstat(disk); err == nil && fi.Mode()&os.ModeSymlink != 0 {
				if target, err := os.Readlink(disk); err == nil {
					if !strings.HasPrefix(target, "/") {
						target = filepath.Join(filepath.Dir(disk), target)
					}
					fmt.Printf("dstat: symlink %s -> %s\n", disk, target)
					disk = target
				}
			}
			return strings.TrimPrefix(disk, "/dev/")
		}
		fmt.Printf("dstat: %s does not exist\n", disk)
		return disk
	}
	return disk
}

func newDisk() *Plugin {
	p := newPlugin()
	p.Nick = []string{"read", "writ"}
	p.Type = "b"
	p.Cols = 2
	p.Files = []string{"/proc/diskstats"}
	p.DiscoverFn = diskDiscover
	p.VarsFn = func(p *Plugin) []string { return diskVars(p, true) }
	p.NameFn = func(p *Plugin) []string {
		ret := make([]string, len(p.Vars))
		for i, name := range p.Vars {
			ret[i] = "dsk/" + sysfsDev(name)
		}
		return ret
	}
	p.ExtractFn = func(p *Plugin) {
		for _, name := range p.Vars {
			p.Set2[name] = []float64{0, 0}
		}
		for _, f := range diskLines() {
			if f[5] == "0" && f[9] == "0" {
				continue
			}
			if isAllZero(f[3:], 11) {
				continue
			}
			name := f[2]
			rs, ws := float64(atoi(f[5])), float64(atoi(f[9]))
			if !diskFilter.MatchString(name) {
				t := vec(p.Set2["total"])
				t[0] += rs
				t[1] += ws
			}
			if containsStr(p.Vars, name) && name != "total" {
				v := vec(p.Set2[name])
				v[0] += rs
				v[1] += ws
			}
		}
		for name := range p.Set2 {
			s2 := vec(p.Set2[name])
			s1 := vec(p.Set1[name])
			v := vec(p.Val[name])
			for i := range v {
				v[i] = (s2[i] - s1[i]) * 512.0 / E.elapsed
			}
		}
		if E.step == E.op.delay {
			for name := range p.Set2 {
				setVecCopy(p.Set1, p.Set2, name)
			}
		}
	}
	return p
}

func newDisk24() *Plugin {
	p := newPlugin()
	p.Nick = []string{"read", "writ"}
	p.Type = "b"
	p.Cols = 2
	p.Files = []string{"/proc/partitions"}
	p.DiscoverFn = func(p *Plugin) []string {
		var ret []string
		for _, l := range linesOrEmpty("/proc/partitions") {
			f := strings.Fields(l)
			if len(f) < 15 || f[0] == "major" || atoi(f[1])%16 != 0 {
				continue
			}
			ret = append(ret, f[3])
		}
		if len(ret) == 0 {
			p.loadErr = fmt.Errorf("No suitable block devices found to monitor")
		}
		return ret
	}
	p.VarsFn = func(p *Plugin) []string { return diskVars(p, false) }
	p.NameFn = func(p *Plugin) []string {
		ret := make([]string, len(p.Vars))
		for i, name := range p.Vars {
			ret[i] = "dsk/" + sysfsDev(name)
		}
		return ret
	}
	p.ExtractFn = func(p *Plugin) {
		for _, name := range p.Vars {
			p.Set2[name] = []float64{0, 0}
		}
		for _, l := range linesOrEmpty("/proc/partitions") {
			f := strings.Fields(l)
			if len(f) < 15 || f[0] == "major" || atoi(f[1])%16 != 0 {
				continue
			}
			name := f[3]
			rs, ws := float64(atoi(f[6])), float64(atoi(f[10]))
			if !diskFilter.MatchString(name) {
				t := vec(p.Set2["total"])
				t[0] += rs
				t[1] += ws
			}
			if containsStr(p.Vars, name) && name != "total" {
				v := vec(p.Set2[name])
				v[0] += rs
				v[1] += ws
			}
		}
		for name := range p.Set2 {
			s2 := vec(p.Set2[name])
			s1 := vec(p.Set1[name])
			v := vec(p.Val[name])
			for i := range v {
				v[i] = (s2[i] - s1[i]) * 512.0 / E.elapsed
			}
		}
		if E.step == E.op.delay {
			for name := range p.Set2 {
				setVecCopy(p.Set1, p.Set2, name)
			}
		}
	}
	return p
}

// diskIOPairRe 匹配 /proc/stat 旧版 disk_io 条目：(maj,min):(rd_ios,rd_merges,rd_sects,...wr_sects)
var diskIOPairRe = regexp.MustCompile(`\((\d+),(\d+)\):\(\d+,\d+,(\d+),\d+,(\d+)\)`)

// devName 对应上游 dev(maj, min)：把主/次设备号转成设备名（旧版内核用）
func devName(maj, min int) string {
	scsi := []int{8, 65, 66, 67, 68, 69, 70, 71, 128, 129, 130, 131, 132, 133, 134, 135}
	ide := []int{3, 22, 33, 34, 56, 57, 88, 89, 90, 91}
	md := []int{9}
	loop := []int{7}
	ram := []int{1}
	ida := []int{72, 73, 74, 75, 76, 77, 78, 79}
	ubd := []int{98}
	cciss := []int{104}
	if i := indexOfInt(scsi, maj); i >= 0 {
		disc := byte('a' + i*16 + min/16)
		if part := min % 16; part != 0 {
			return fmt.Sprintf("sd%c%d", disc, part)
		}
		return fmt.Sprintf("sd%c", disc)
	}
	if i := indexOfInt(ide, maj); i >= 0 {
		disc := byte('a' + i*2 + min/64)
		if part := min % 64; part != 0 {
			return fmt.Sprintf("hd%c%d", disc, part)
		}
		return fmt.Sprintf("hd%c", disc)
	}
	if indexOfInt(md, maj) >= 0 {
		return fmt.Sprintf("md%d", min)
	}
	if indexOfInt(loop, maj) >= 0 {
		return fmt.Sprintf("loop%d", min)
	}
	if indexOfInt(ram, maj) >= 0 {
		return fmt.Sprintf("ram%d", min)
	}
	if indexOfInt(cciss, maj) >= 0 {
		disc := indexOfInt(cciss, maj)*16 + min/16
		if part := min % 16; part != 0 {
			return fmt.Sprintf("c0d%dp%d", disc, part)
		}
		return fmt.Sprintf("c0d%d", disc)
	}
	if i := indexOfInt(ida, maj); i >= 0 {
		disc := min / 16
		if part := min % 16; part != 0 {
			return fmt.Sprintf("ida%d-%d-%d", i, disc, part)
		}
		return fmt.Sprintf("ida%d-%d", i, disc)
	}
	if i := indexOfInt(ubd, maj); i >= 0 {
		disc := i*16 + min/16
		if part := min % 16; part != 0 {
			return fmt.Sprintf("ubd%dp%d", disc, part)
		}
		return fmt.Sprintf("ubd%d", disc)
	}
	if maj == 253 {
		return fmt.Sprintf("dm-%d", min)
	}
	return fmt.Sprintf("dev%d-%d", maj, min)
}

func indexOfInt(list []int, v int) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return -1
}

func newDisk24Old() *Plugin {
	p := newPlugin()
	p.Nick = []string{"read", "writ"}
	p.Type = "b"
	p.Cols = 2
	p.Files = []string{"/proc/stat"}
	parseDiskIO := func(fn func(name string, rs, ws float64)) {
		for _, l := range linesOrEmpty("/proc/stat") {
			parts := strings.Split(l, ":")
			if len(parts) < 3 || strings.TrimSpace(parts[0]) != "disk_io" {
				continue
			}
			for _, pair := range strings.Fields(l)[1:] {
				m := diskIOPairRe.FindStringSubmatch(pair)
				if m == nil {
					continue
				}
				fn(devName(atoi(m[1]), atoi(m[2])), float64(atoi(m[3])), float64(atoi(m[4])))
			}
		}
	}
	p.DiscoverFn = func(p *Plugin) []string {
		var ret []string
		parseDiskIO(func(name string, rs, ws float64) {
			if !containsStr(ret, name) {
				ret = append(ret, name)
			}
		})
		if len(ret) == 0 {
			p.loadErr = fmt.Errorf("No suitable block devices found to monitor")
		}
		return ret
	}
	p.VarsFn = func(p *Plugin) []string { return diskVars(p, false) }
	p.NameFn = func(p *Plugin) []string {
		ret := make([]string, len(p.Vars))
		for i, name := range p.Vars {
			ret[i] = "dsk/" + sysfsDev(name)
		}
		return ret
	}
	p.ExtractFn = func(p *Plugin) {
		for _, name := range p.Vars {
			p.Set2[name] = []float64{0, 0}
		}
		parseDiskIO(func(name string, rs, ws float64) {
			if !diskFilter.MatchString(name) {
				t := vec(p.Set2["total"])
				t[0] += rs
				t[1] += ws
			}
			if containsStr(p.Vars, name) && name != "total" {
				v := vec(p.Set2[name])
				v[0] += rs
				v[1] += ws
			}
		})
		for name := range p.Set2 {
			s2 := vec(p.Set2[name])
			s1 := vec(p.Set1[name])
			v := vec(p.Val[name])
			for i := range v {
				v[i] = (s2[i] - s1[i]) * 512.0 / E.elapsed
			}
		}
		if E.step == E.op.delay {
			for name := range p.Set2 {
				setVecCopy(p.Set1, p.Set2, name)
			}
		}
	}
	return p
}

// ---------- epoch / fs ----------

func newEpoch() *Plugin {
	p := newPlugin()
	p.Name = "epoch"
	p.Vars = []string{"epoch"}
	p.Width = 10
	p.Scale = 0
	if E != nil && E.op.debug > 0 {
		p.Width = 13
	}
	p.ExtractFn = func(p *Plugin) { p.Val["epoch"] = E.starttime }
	return p
}

func newFS() *Plugin {
	p := newPlugin()
	p.Name = "filesystem"
	p.Vars = []string{"files", "inodes"}
	p.Type = "d"
	p.Width = 6
	p.Scale = 1000
	p.Files = []string{"/proc/sys/fs/file-nr", "/proc/sys/fs/inode-nr"}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/sys/fs/file-nr") {
			f := strings.Fields(l)
			if len(f) < 1 {
				continue
			}
			p.Val["files"] = float64(atoi(f[0]))
		}
		for _, l := range linesOrEmpty("/proc/sys/fs/inode-nr") {
			f := strings.Fields(l)
			if len(f) < 2 {
				continue
			}
			p.Val["inodes"] = float64(atoi(f[0]) - atoi(f[1]))
		}
	}
	return p
}

// ---------- int ----------

func intMap() map[string]string {
	ret := map[string]string{}
	for _, line := range linesOrEmpty("/proc/interrupts") {
		f := strings.Fields(line)
		if len(f) <= E.cpunr {
			continue
		}
		l1 := strings.Split(f[0], ":")[0]
		rest := f
		if E.cpunr+2 <= len(f) {
			rest = f[E.cpunr+2:]
		} else {
			rest = nil
		}
		ret[l1] = l1
		for _, name := range strings.Split(strings.Join(rest, " "), ",") {
			ret[strings.ToLower(strings.TrimSpace(name))] = l1
		}
	}
	return ret
}

func intVarsFilter(varlist []string, exclusions []string) []string {
	i := 0
	for i < len(varlist) {
		if containsStr(exclusions, varlist[i]) {
			varlist = append(varlist[:i], varlist[i+1:]...)
		}
		i++
	}
	return varlist
}

func newInt() *Plugin {
	p := newPlugin()
	p.Name = "interrupts"
	p.Type = "d"
	p.Width = 5
	p.Scale = 1000
	p.Files = []string{"/proc/stat", "/proc/interrupts"}
	p.DiscoverFn = func(p *Plugin) []string {
		var ret []string
		for _, l := range linesOrEmpty("/proc/stat") {
			f := strings.Fields(l)
			if len(f) == 0 || f[0] != "intr" {
				continue
			}
			for idx, v := range f[2:] {
				if atoi(v) > 10 {
					ret = append(ret, strconv.Itoa(idx))
				}
			}
		}
		return ret
	}
	p.VarsFn = func(p *Plugin) []string {
		imap := intMap()
		var varlist []string
		if len(E.op.intlist) > 0 {
			varlist = append([]string{}, E.op.intlist...)
		} else {
			varlist = append([]string{}, p.discovered...)
			varlist = intVarsFilter(varlist, []string{"0", "1", "2", "8", "NMI", "LOC", "MIS", "CPU0"})
			if !E.op.full && len(varlist) > 3 {
				varlist = varlist[len(varlist)-3:]
			}
		}
		var ret []string
		for _, name := range varlist {
			if containsStr(p.discovered, name) || name == "total" {
				ret = append(ret, name)
			} else if v, ok := imap[strings.ToLower(name)]; ok {
				ret = append(ret, v)
			}
		}
		return ret
	}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/stat") {
			f := strings.Fields(l)
			if len(f) == 0 || f[0] != "intr" {
				continue
			}
			for _, name := range p.Vars {
				if name != "total" {
					idx := atoi(name) + 2
					if idx < len(f) {
						p.Set2[name] = float64(atoi(f[idx]))
					}
				}
			}
			if len(f) > 1 {
				p.Set2["total"] = float64(atoi(f[1]))
			}
		}
		for _, name := range p.Vars {
			p.Val[name] = (toFloat(p.Set2[name]) - toFloat(p.Set1[name])) / E.elapsed
		}
		if E.step == E.op.delay {
			for _, name := range p.Vars {
				p.Set1[name] = p.Set2[name]
			}
		}
	}
	return p
}

func newInt24() *Plugin {
	p := newPlugin()
	p.Name = "interrupts"
	p.Type = "d"
	p.Width = 5
	p.Scale = 1000
	p.Files = []string{"/proc/interrupts"}
	p.DiscoverFn = func(p *Plugin) []string {
		var ret []string
		for _, l := range linesOrEmpty("/proc/interrupts") {
			f := strings.Fields(l)
			if len(f) < E.cpunr+1 {
				continue
			}
			name := strings.Split(f[0], ":")[0]
			if atoi(f[1]) > 10 {
				ret = append(ret, name)
			}
		}
		return ret
	}
	p.VarsFn = func(p *Plugin) []string {
		imap := intMap()
		var varlist []string
		if len(E.op.intlist) > 0 {
			varlist = append([]string{}, E.op.intlist...)
		} else {
			varlist = append([]string{}, p.discovered...)
			varlist = intVarsFilter(varlist, []string{"0", "1", "2", "8", "CPU0", "ERR", "LOC", "MIS", "NMI"})
			if !E.op.full && len(varlist) > 3 {
				varlist = varlist[len(varlist)-3:]
			}
		}
		var ret []string
		for _, name := range varlist {
			if containsStr(p.discovered, name) {
				ret = append(ret, name)
			} else if v, ok := imap[strings.ToLower(name)]; ok {
				ret = append(ret, v)
			}
		}
		return ret
	}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/interrupts") {
			f := strings.Fields(l)
			if len(f) < E.cpunr+1 {
				continue
			}
			name := strings.Split(f[0], ":")[0]
			if containsStr(p.Vars, name) {
				total := 0
				lim := 1 + E.cpunr
				if lim > len(f) {
					lim = len(f)
				}
				for _, v := range f[1:lim] {
					total += atoi(v)
				}
				p.Set2[name] = float64(total)
			}
		}
		for name := range p.Set2 {
			p.Val[name] = (toFloat(p.Set2[name]) - toFloat(p.Set1[name])) / E.elapsed
		}
		if E.step == E.op.delay {
			for name := range p.Set2 {
				p.Set1[name] = p.Set2[name]
			}
		}
	}
	return p
}

// ---------- io ----------

func newIO() *Plugin {
	p := newPlugin()
	p.Nick = []string{"read", "writ"}
	p.Type = "f"
	p.Width = 5
	p.Scale = 1000
	p.Cols = 2
	p.Files = []string{"/proc/diskstats"}
	p.DiscoverFn = diskDiscover
	p.VarsFn = func(p *Plugin) []string { return diskVars(p, false) }
	p.NameFn = func(p *Plugin) []string {
		ret := make([]string, len(p.Vars))
		for i, name := range p.Vars {
			ret[i] = "io/" + name
		}
		return ret
	}
	p.ExtractFn = func(p *Plugin) {
		for _, name := range p.Vars {
			p.Set2[name] = []float64{0, 0}
		}
		for _, f := range diskLines() {
			if f[3] == "0" && f[7] == "0" {
				continue
			}
			if isAllZero(f[3:], 11) {
				continue
			}
			name := f[2]
			r, w := float64(atoi(f[3])), float64(atoi(f[7]))
			if !diskFilter.MatchString(name) {
				t := vec(p.Set2["total"])
				t[0] += r
				t[1] += w
			}
			if containsStr(p.Vars, name) && name != "total" {
				v := vec(p.Set2[name])
				v[0] += r
				v[1] += w
			}
		}
		for name := range p.Set2 {
			s2 := vec(p.Set2[name])
			s1 := vec(p.Set1[name])
			v := vec(p.Val[name])
			for i := range v {
				v[i] = (s2[i] - s1[i]) * 1.0 / E.elapsed
			}
		}
		if E.step == E.op.delay {
			for name := range p.Set2 {
				setVecCopy(p.Set1, p.Set2, name)
			}
		}
	}
	return p
}

// ---------- ipc / load / lock ----------

func newIPC() *Plugin {
	p := newPlugin()
	p.Name = "sysv ipc"
	p.Vars = []string{"msg", "sem", "shm"}
	p.Nick = []string{"msg", "sem", "shm"}
	p.Type = "d"
	p.Width = 3
	p.Scale = 10
	p.Files = []string{"/proc/sysvipc/msg", "/proc/sysvipc/sem", "/proc/sysvipc/shm"}
	p.ExtractFn = func(p *Plugin) {
		for _, name := range p.Vars {
			lines, err := readLines("/proc/sysvipc/" + name)
			if err != nil {
				p.Val[name] = float64(-1)
				continue
			}
			p.Val[name] = float64(len(lines) - 1)
		}
	}
	return p
}

func newLoad() *Plugin {
	p := newPlugin()
	p.Name = "load avg"
	p.Nick = []string{"1m", "5m", "15m"}
	p.Vars = []string{"load1", "load5", "load15"}
	p.Type = "f"
	p.Width = 4
	p.Scale = 0.5
	p.Files = []string{"/proc/loadavg"}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/loadavg") {
			f := strings.Fields(l)
			if len(f) < 3 {
				continue
			}
			p.Val["load1"] = atof(f[0])
			p.Val["load5"] = atof(f[1])
			p.Val["load15"] = atof(f[2])
		}
	}
	return p
}

func newLock() *Plugin {
	p := newPlugin()
	p.Name = "file locks"
	p.Nick = []string{"pos", "lck", "rea", "wri"}
	p.Vars = []string{"posix", "flock", "read", "write"}
	p.Type = "f"
	p.Width = 3
	p.Scale = 10
	p.Files = []string{"/proc/locks"}
	p.ExtractFn = func(p *Plugin) {
		for _, name := range p.Vars {
			p.Val[name] = float64(0)
		}
		for _, l := range linesOrEmpty("/proc/locks") {
			f := strings.Fields(l)
			if len(f) < 4 {
				continue
			}
			switch f[1] {
			case "POSIX":
				p.Val["posix"] = toFloat(p.Val["posix"]) + 1
			case "FLOCK":
				p.Val["flock"] = toFloat(p.Val["flock"]) + 1
			}
			switch f[3] {
			case "READ":
				p.Val["read"] = toFloat(p.Val["read"]) + 1
			case "WRITE":
				p.Val["write"] = toFloat(p.Val["write"]) + 1
			}
		}
	}
	return p
}

// ---------- mem ----------

func memExtract(p *Plugin) {
	advVars := []string{"MemTotal", "Shmem", "SReclaimable"}
	advVal := map[string]float64{}
	for _, l := range linesOrEmpty("/proc/meminfo") {
		f := strings.Fields(l)
		if len(f) < 2 {
			continue
		}
		name := strings.Split(f[0], ":")[0]
		if containsStr(p.Vars, name) {
			p.Val[name] = float64(atoi(f[1])) * 1024.0
		}
		if containsStr(advVars, name) {
			advVal[name] = float64(atoi(f[1])) * 1024.0
		}
	}
	p.Val["MemUsed"] = advVal["MemTotal"] - toFloat(p.Val["MemFree"]) - toFloat(p.Val["Buffers"]) -
		toFloat(p.Val["Cached"]) - advVal["SReclaimable"] + advVal["Shmem"]
}

func newMem() *Plugin {
	p := newPlugin()
	p.Name = "memory usage"
	p.Nick = []string{"used", "free", "buff", "cach"}
	p.Vars = []string{"MemUsed", "MemFree", "Buffers", "Cached"}
	p.Files = []string{"/proc/meminfo"}
	p.ExtractFn = memExtract
	return p
}

func newMemAdv() *Plugin {
	p := newPlugin()
	p.Name = "advanced memory usage"
	p.Nick = []string{"total", "used", "free", "buff", "cach", "dirty", "shmem", "recl"}
	p.Vars = []string{"MemTotal", "MemUsed", "MemFree", "Buffers", "Cached", "Dirty", "Shmem", "SReclaimable"}
	p.Files = []string{"/proc/meminfo"}
	p.ExtractFn = memExtract
	return p
}

// ---------- net ----------

func netSplit() [][]string {
	var out [][]string
	for _, l := range linesOrEmpty("/proc/net/dev") {
		f := strings.Fields(strings.ReplaceAll(l, ":", " "))
		out = append(out, f)
	}
	return out
}

func netDiscover(p *Plugin) []string {
	var ret []string
	for _, f := range netSplit() {
		if len(f) < 17 {
			continue
		}
		if f[2] == "0" && f[10] == "0" {
			continue
		}
		if f[0] != "lo" && f[0] != "face" {
			ret = append(ret, f[0])
		}
	}
	sortStrings(ret)
	return ret
}

func newNet() *Plugin {
	p := newPlugin()
	p.Nick = []string{"recv", "send"}
	p.Type = "b"
	p.Cols = 2
	p.Files = []string{"/proc/net/dev"}
	p.DiscoverFn = netDiscover
	p.VarsFn = func(p *Plugin) []string {
		var varlist []string
		if len(E.op.netlist) > 0 {
			varlist = E.op.netlist
		} else if !E.op.full {
			varlist = []string{"total"}
		} else {
			varlist = append([]string{}, p.discovered...)
			sortStrings(varlist)
		}
		var ret []string
		for _, name := range varlist {
			if containsStr(p.discovered, name) || name == "total" || name == "lo" {
				ret = append(ret, name)
			}
		}
		if len(ret) == 0 {
			p.loadErr = fmt.Errorf("No suitable network interfaces found to monitor")
		}
		return ret
	}
	p.NameFn = func(p *Plugin) []string {
		ret := make([]string, len(p.Vars))
		for i, name := range p.Vars {
			ret[i] = "net/" + name
		}
		return ret
	}
	p.ExtractFn = func(p *Plugin) {
		p.Set2["total"] = []float64{0, 0}
		for _, f := range netSplit() {
			if len(f) < 17 {
				continue
			}
			if f[2] == "0" && f[10] == "0" {
				continue
			}
			name := f[0]
			if containsStr(p.Vars, name) {
				p.Set2[name] = []float64{float64(atofInt64(f[1])), float64(atofInt64(f[9]))}
			}
			if !netTotalFilter.MatchString(name) {
				t := vec(p.Set2["total"])
				t[0] += float64(atofInt64(f[1]))
				t[1] += float64(atofInt64(f[9]))
			}
		}
		if E.update != 0 {
			for name := range p.Set2 {
				s2 := vec(p.Set2[name])
				s1 := vec(p.Set1[name])
				v := vec(p.Val[name])
				for i := range v {
					v[i] = (s2[i] - s1[i]) * 1.0 / E.elapsed
					if v[i] < 0 {
						v[i] += E.maxint + 1
					}
				}
			}
		}
		if E.step == E.op.delay {
			for name := range p.Set2 {
				setVecCopy(p.Set1, p.Set2, name)
			}
		}
	}
	return p
}

func atofInt64(s string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v
}

// ---------- page ----------

func newPage() *Plugin {
	p := newPlugin()
	p.Name = "paging"
	p.Nick = []string{"in", "out"}
	p.Vars = []string{"pswpin", "pswpout"}
	p.Type = "d"
	p.Files = []string{"/proc/vmstat"}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/vmstat") {
			f := strings.Fields(l)
			if len(f) < 2 {
				continue
			}
			if containsStr(p.Vars, f[0]) {
				p.Set2[f[0]] = float64(atoi(f[1]))
			}
		}
		for _, name := range p.Vars {
			p.Val[name] = (toFloat(p.Set2[name]) - toFloat(p.Set1[name])) * float64(E.pagesize) / E.elapsed
		}
		if E.step == E.op.delay {
			for _, name := range p.Vars {
				p.Set1[name] = p.Set2[name]
			}
		}
	}
	return p
}

func newPage24() *Plugin {
	p := newPlugin()
	p.Name = "paging"
	p.Nick = []string{"in", "out"}
	p.Vars = []string{"pswpin", "pswpout"}
	p.Type = "d"
	p.Files = []string{"/proc/stat"}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/stat") {
			f := strings.Fields(l)
			if len(f) < 3 || f[0] != "swap" {
				continue
			}
			p.Set2["pswpin"] = float64(atoi(f[1]))
			p.Set2["pswpout"] = float64(atoi(f[2]))
			break
		}
		for _, name := range p.Vars {
			p.Val[name] = (toFloat(p.Set2[name]) - toFloat(p.Set1[name])) * float64(E.pagesize) / E.elapsed
		}
		if E.step == E.op.delay {
			for _, name := range p.Vars {
				p.Set1[name] = p.Set2[name]
			}
		}
	}
	return p
}

// ---------- proc ----------

func newProc() *Plugin {
	p := newPlugin()
	p.Name = "procs"
	p.Nick = []string{"run", "blk", "new"}
	p.Vars = []string{"procs_running", "procs_blocked", "processes"}
	p.Type = "f"
	p.Width = 3
	p.Scale = 10
	p.Files = []string{"/proc/stat"}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/stat") {
			f := strings.Fields(l)
			if len(f) < 2 {
				continue
			}
			switch f[0] {
			case "processes":
				p.Val["processes"] = float64(0)
				p.Set2["processes"] = float64(atoi(f[1]))
			case "procs_running":
				p.Set2["procs_running"] = toFloat(p.Set2["procs_running"]) + float64(atoi(f[1])) - 1
			case "procs_blocked":
				p.Set2["procs_blocked"] = toFloat(p.Set2["procs_blocked"]) + float64(atoi(f[1]))
			}
		}
		p.Val["processes"] = (toFloat(p.Set2["processes"]) - toFloat(p.Set1["processes"])) / E.elapsed
		p.Val["procs_running"] = toFloat(p.Set2["procs_running"])
		p.Val["procs_blocked"] = toFloat(p.Set2["procs_blocked"])
		if E.step == E.op.delay {
			for _, name := range p.Vars {
				p.Set1[name] = p.Set2[name]
			}
			p.Set2["procs_running"] = float64(0)
			p.Set2["procs_blocked"] = float64(0)
		}
	}
	return p
}

// ---------- raw / socket ----------

func newRaw() *Plugin {
	p := newPlugin()
	p.Name = "raw"
	p.Nick = []string{"raw"}
	p.Vars = []string{"sockets"}
	p.Type = "d"
	p.Width = 4
	p.Scale = 1000
	p.Files = []string{"/proc/net/raw"}
	p.ExtractFn = func(p *Plugin) {
		lines := -1
		for range linesOrEmpty("/proc/net/raw") {
			lines++
		}
		p.Val["sockets"] = float64(lines)
	}
	return p
}

func newSocket() *Plugin {
	p := newPlugin()
	p.Name = "sockets"
	p.Nick = []string{"tot", "tcp", "udp", "raw", "frg"}
	p.Vars = []string{"sockets:", "TCP:", "UDP:", "RAW:", "FRAG:"}
	p.Type = "d"
	p.Width = 4
	p.Scale = 1000
	p.Files = []string{"/proc/net/sockstat"}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/net/sockstat") {
			f := strings.Fields(l)
			if len(f) < 3 {
				continue
			}
			p.Val[f[0]] = float64(atoi(f[2]))
		}
		p.Val["other"] = toFloat(p.Val["sockets:"]) - toFloat(p.Val["TCP:"]) -
			toFloat(p.Val["UDP:"]) - toFloat(p.Val["RAW:"]) - toFloat(p.Val["FRAG:"])
	}
	return p
}

// ---------- swap ----------

func newSwap() *Plugin {
	p := newPlugin()
	p.Name = "swap"
	p.Nick = []string{"used", "free"}
	p.Type = "d"
	p.Cols = 2
	p.Files = []string{"/proc/swaps"}
	p.DiscoverFn = func(p *Plugin) []string {
		var ret []string
		for _, l := range linesOrEmpty("/proc/swaps") {
			f := strings.Fields(l)
			if len(f) < 5 || f[0] == "Filename" {
				continue
			}
			if !isIntStr(f[2]) || !isIntStr(f[3]) {
				continue
			}
			ret = append(ret, f[0])
		}
		sortStrings(ret)
		return ret
	}
	p.VarsFn = func(p *Plugin) []string {
		var varlist []string
		if len(E.op.swaplist) > 0 {
			varlist = E.op.swaplist
		} else if !E.op.full {
			varlist = []string{"total"}
		} else {
			varlist = append([]string{}, p.discovered...)
			sortStrings(varlist)
		}
		var ret []string
		for _, name := range varlist {
			if containsStr(p.discovered, name) || name == "total" {
				ret = append(ret, name)
			}
		}
		if len(ret) == 0 {
			p.loadErr = fmt.Errorf("No suitable swap devices found to monitor")
		}
		return ret
	}
	p.NameFn = nil
	p.ExtractFn = func(p *Plugin) {
		p.Val["total"] = []float64{0, 0}
		for _, l := range linesOrEmpty("/proc/swaps") {
			f := strings.Fields(l)
			if len(f) < 5 || f[0] == "Filename" {
				continue
			}
			name := f[0]
			used := float64(atoi(f[3])) * 1024.0
			free := float64(atoi(f[2])-atoi(f[3])) * 1024.0
			p.Val[name] = []float64{used, free}
			t := vec(p.Val["total"])
			t[0] += used
			t[1] += free
		}
	}
	return p
}

func isIntStr(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func newSwapOld() *Plugin {
	p := newPlugin()
	p.Name = "swap"
	p.Nick = []string{"used", "free"}
	p.Vars = []string{"SwapUsed", "SwapFree"}
	p.Type = "d"
	p.Files = []string{"/proc/meminfo"}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/meminfo") {
			f := strings.Fields(l)
			if len(f) < 2 {
				continue
			}
			name := strings.Split(f[0], ":")[0]
			if containsStr(p.Vars, name) || name == "SwapTotal" {
				p.Val[name] = float64(atoi(f[1])) * 1024.0
			}
		}
		p.Val["SwapUsed"] = toFloat(p.Val["SwapTotal"]) - toFloat(p.Val["SwapFree"])
	}
	return p
}

// ---------- sys ----------

func newSys() *Plugin {
	p := newPlugin()
	p.Name = "system"
	p.Nick = []string{"int", "csw"}
	p.Vars = []string{"intr", "ctxt"}
	p.Type = "d"
	p.Width = 5
	p.Scale = 1000
	p.Files = []string{"/proc/stat"}
	p.ExtractFn = func(p *Plugin) {
		for _, l := range linesOrEmpty("/proc/stat") {
			f := strings.Fields(l)
			if len(f) < 2 {
				continue
			}
			if containsStr(p.Vars, f[0]) {
				p.Set2[f[0]] = float64(atoi(f[1]))
			}
		}
		for _, name := range p.Vars {
			p.Val[name] = (toFloat(p.Set2[name]) - toFloat(p.Set1[name])) / E.elapsed
		}
		if E.step == E.op.delay {
			for _, name := range p.Vars {
				p.Set1[name] = p.Set2[name]
			}
		}
	}
	return p
}

// ---------- tcp / udp / unix ----------

func newTCP() *Plugin {
	p := newPlugin()
	p.Name = "tcp sockets"
	p.Nick = []string{"lis", "act", "syn", "tim", "clo"}
	p.Vars = []string{"listen", "established", "syn", "wait", "close"}
	p.Type = "d"
	p.Width = 4
	p.Scale = 1000
	p.Files = []string{"/proc/net/tcp", "/proc/net/tcp6"}
	p.ExtractFn = func(p *Plugin) {
		for _, name := range p.Vars {
			p.Val[name] = float64(0)
		}
		for _, file := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
			for _, l := range linesOrEmpty(file) {
				f := strings.Fields(l)
				if len(f) < 12 {
					continue
				}
				switch f[3] {
				case "0A":
					p.Val["listen"] = toFloat(p.Val["listen"]) + 1
				case "01":
					p.Val["established"] = toFloat(p.Val["established"]) + 1
				case "02", "03", "09":
					p.Val["syn"] = toFloat(p.Val["syn"]) + 1
				case "06":
					p.Val["wait"] = toFloat(p.Val["wait"]) + 1
				case "04", "05", "07", "08", "0B":
					p.Val["close"] = toFloat(p.Val["close"]) + 1
				}
			}
		}
	}
	return p
}

func newUDP() *Plugin {
	p := newPlugin()
	p.Name = "udp"
	p.Nick = []string{"lis", "act"}
	p.Vars = []string{"listen", "established"}
	p.Type = "d"
	p.Width = 4
	p.Scale = 1000
	p.Files = []string{"/proc/net/udp", "/proc/net/udp6"}
	p.ExtractFn = func(p *Plugin) {
		for _, name := range p.Vars {
			p.Val[name] = float64(0)
		}
		for _, file := range []string{"/proc/net/udp", "/proc/net/udp6"} {
			for _, l := range linesOrEmpty(file) {
				f := strings.Fields(l)
				if len(f) < 4 {
					continue
				}
				switch f[3] {
				case "07":
					p.Val["listen"] = toFloat(p.Val["listen"]) + 1
				case "01":
					p.Val["established"] = toFloat(p.Val["established"]) + 1
				}
			}
		}
	}
	return p
}

func newUnix() *Plugin {
	p := newPlugin()
	p.Name = "unix sockets"
	p.Nick = []string{"dgm", "str", "lis", "act"}
	p.Vars = []string{"datagram", "stream", "listen", "established"}
	p.Type = "d"
	p.Width = 4
	p.Scale = 1000
	p.Files = []string{"/proc/net/unix"}
	p.ExtractFn = func(p *Plugin) {
		for _, name := range p.Vars {
			p.Val[name] = float64(0)
		}
		for _, l := range linesOrEmpty("/proc/net/unix") {
			f := strings.Fields(l)
			if len(f) < 6 {
				continue
			}
			switch f[4] {
			case "0002":
				p.Val["datagram"] = toFloat(p.Val["datagram"]) + 1
			case "0001":
				p.Val["stream"] = toFloat(p.Val["stream"]) + 1
				switch f[5] {
				case "01":
					p.Val["listen"] = toFloat(p.Val["listen"]) + 1
				case "03":
					p.Val["established"] = toFloat(p.Val["established"]) + 1
				}
			}
		}
	}
	return p
}

// ---------- time ----------

func newTime() *Plugin {
	p := newPlugin()
	p.Name = "system"
	p.Type = "s"
	p.Scale = 0
	p.Vars = []string{"time"}
	tf := os.Getenv("DSTAT_TIMEFMT")
	if tf == "" {
		tf = "%d-%m %H:%M:%S"
	}
	nowStr := strf(tf, time.Now())
	if E != nil && E.op.debug > 0 {
		p.Width = len(nowStr) + 4
	} else {
		p.Width = len(nowStr)
	}
	p.ExtractFn = func(p *Plugin) {
		st := time.Unix(int64(E.starttime), int64((E.starttime-float64(int64(E.starttime)))*1e9))
		s := strf(tf, st)
		if E.op.debug > 0 {
			s += fmt.Sprintf(".%03d", int(pyRound(E.starttime*1000))%1000)
		}
		p.Val["time"] = s
	}
	return p
}

// ---------- vm ----------

func vmExtract(p *Plugin) {
	for _, name := range p.Vars {
		p.Set2[name] = float64(0)
	}
	for _, l := range linesOrEmpty("/proc/vmstat") {
		f := strings.Fields(l)
		if len(f) < 2 {
			continue
		}
		for _, name := range p.Vars {
			if fnmatch(f[0], name) {
				p.Set2[name] = toFloat(p.Set2[name]) + float64(atoi(f[1]))
			}
		}
	}
	for _, name := range p.Vars {
		p.Val[name] = (toFloat(p.Set2[name]) - toFloat(p.Set1[name])) / E.elapsed
	}
	if E.step == E.op.delay {
		for _, name := range p.Vars {
			p.Set1[name] = p.Set2[name]
		}
	}
}

func newVM() *Plugin {
	p := newPlugin()
	p.Name = "virtual memory"
	p.Nick = []string{"majpf", "minpf", "alloc", "free"}
	p.Vars = []string{"pgmajfault", "pgfault", "pgalloc_*", "pgfree"}
	p.Type = "d"
	p.Width = 5
	p.Scale = 1000
	p.Files = []string{"/proc/vmstat"}
	p.ExtractFn = vmExtract
	return p
}

func newVMAdv() *Plugin {
	p := newPlugin()
	p.Name = "advanced virtual memory"
	p.Nick = []string{"steal", "scanK", "scanD", "pgoru", "astll"}
	p.Vars = []string{"pgsteal_*", "pgscan_kswapd_*", "pgscan_direct_*", "pageoutrun", "allocstall"}
	p.Type = "d"
	p.Width = 5
	p.Scale = 1000
	p.Files = []string{"/proc/vmstat"}
	p.ExtractFn = vmExtract
	return p
}

// ---------- zones ----------

func newZones() *Plugin {
	p := newPlugin()
	p.Name = "zones memory"
	p.Nick = []string{"d32F", "d32H", "normF", "normH"}
	p.Vars = []string{"DMA32_free", "DMA32_high", "Normal_free", "Normal_high"}
	p.Type = "d"
	p.Width = 5
	p.Scale = 1000
	p.Files = []string{"/proc/zoneinfo"}
	p.ExtractFn = func(p *Plugin) {
		zone := ""
		foundHigh := false
		for _, l := range linesOrEmpty("/proc/zoneinfo") {
			f := strings.Fields(l)
			if len(f) < 2 {
				continue
			}
			switch {
			case strings.HasPrefix(f[0], "Node"):
				if len(f) > 3 {
					zone = f[3]
				}
				foundHigh = false
			case strings.HasPrefix(f[0], "pages"):
				if len(f) > 2 {
					p.Val[zone+"_free"] = float64(atoi(f[2]))
				}
			case strings.HasPrefix(f[0], "high") && !foundHigh:
				p.Val[zone+"_high"] = float64(atoi(f[1]))
				foundHigh = true
			}
		}
	}
	return p
}
