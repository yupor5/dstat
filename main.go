package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const VERSION = "0.8.0"

var (
	E          *engine
	stdoutTTY  = false
	procRootEn = ""
)

type optionArg struct {
	opt string
	arg string
}

type options struct {
	bits         bool
	blackonwhite bool
	color        *bool
	debug        int
	float        bool
	full         bool
	header       bool
	integer      bool
	output       string
	pidfile      string
	profile      string
	update       bool
	delay        int
	count        int
	plugins      []string
	cpulist      []string
	intlist      []string
	disklist     []string
	netlist      []string
	swaplist     []string
	args         []string
}

type engine struct {
	op         *options
	theme      *theme
	cpunr      int
	hz         int
	pagesize   int
	maxint     float64
	inittime   float64
	starttime  float64
	elapsed    float64
	step       int
	loop       int
	update     int
	missed     int
	totlist    []*Plugin
	vislist    []*Plugin
	oldvislist []*Plugin
	showheader bool
	rows       int
	cols       int
	output     *os.File
	linewidth  int
}

type renderer struct {
	E     *engine
	theme *theme
}

func (e *engine) r() *renderer { return &renderer{E: e, theme: e.theme} }

// ---------- 内部插件名（其余同名函数见 plugins.go） ----------

var internalPlugins = []string{
	"aio", "cpu", "cpu-adv", "cpu-use", "cpu24",
	"disk", "disk24", "disk24-old", "epoch", "fs",
	"int", "int24", "io", "ipc", "load", "lock", "mem", "mem-adv",
	"net", "page", "page24", "proc", "raw", "socket", "swap",
	"swap-old", "sys", "tcp", "time", "udp", "unix", "vm", "vm-adv", "zones",
}

var longOptionNames = []string{
	"all", "all-plugins", "bits", "bw", "black-on-white", "color",
	"debug", "filesystem", "float", "full", "help", "integer",
	"list", "mods", "modules", "nocolor", "noheaders", "noupdate",
	"output", "pidfile", "profile", "version", "vmstat",
}

var shortOptsWithArg = map[byte]bool{'C': true, 'D': true, 'I': true, 'M': true, 'N': true, 'S': true, 'o': true}

const shortOptStr = "acdfghilmno:prstTvyC:D:I:M:N:S:V"

// parseOptions 复刻上游 Options 的行为（含 getopt 唯一前缀匹配）
func parseOptions(argv []string) (*options, error) {
	op := &options{
		delay:  1,
		count:  -1,
		header: true,
		update: true,
		args:   argv,
	}
	stdoutTTY = isTerminal(os.Stdout)
	if !stdoutTTY {
		no := false
		op.color = &no
		op.header = false
		op.update = false
	}

	opts, err := getopt(argv)
	if err != nil {
		return nil, err
	}

	longNames := append(append([]string{}, longOptionNames...), internalPlugins...)
	_ = longNames

	for _, oa := range opts {
		opt, arg := oa.opt, oa.arg
		switch opt {
		case "-c":
			op.plugins = append(op.plugins, "cpu")
		case "-C":
			op.cpulist = strings.Split(arg, ",")
		case "-d":
			op.plugins = append(op.plugins, "disk")
		case "-D":
			op.disklist = strings.Split(arg, ",")
		case "--filesystem":
			op.plugins = append(op.plugins, "fs")
		case "-g":
			op.plugins = append(op.plugins, "page")
		case "-i":
			op.plugins = append(op.plugins, "int")
		case "-I":
			op.intlist = strings.Split(arg, ",")
		case "-l":
			op.plugins = append(op.plugins, "load")
		case "-m":
			op.plugins = append(op.plugins, "mem")
		case "-M", "--mods", "--modules":
			fmt.Fprintf(os.Stderr, "WARNING: Option %s is deprecated, please use --%s instead\n",
				opt, strings.Join(strings.Split(arg, ","), " --"))
			op.plugins = append(op.plugins, strings.Split(arg, ",")...)
		case "-n":
			op.plugins = append(op.plugins, "net")
		case "-N":
			op.netlist = strings.Split(arg, ",")
		case "-p":
			op.plugins = append(op.plugins, "proc")
		case "-r":
			op.plugins = append(op.plugins, "io")
		case "-s":
			op.plugins = append(op.plugins, "swap")
		case "-S":
			op.swaplist = strings.Split(arg, ",")
		case "-t":
			op.plugins = append(op.plugins, "time")
		case "-T":
			op.plugins = append(op.plugins, "epoch")
		case "-y":
			op.plugins = append(op.plugins, "sys")
		case "-a", "--all":
			op.plugins = append(op.plugins, "cpu", "disk", "net", "page", "sys")
		case "-v", "--vmstat":
			op.plugins = append(op.plugins, "proc", "mem", "page", "disk", "sys", "cpu")
		case "-f", "--full":
			op.full = true
		case "--all-plugins":
			op.plugins = append(op.plugins, internalPlugins...)
		case "--bits":
			op.bits = true
		case "--bw", "--black-on-white", "--blackonwhite":
			op.blackonwhite = true
		case "--color":
			yes := true
			op.color = &yes
			op.update = true
		case "--debug":
			op.debug++
		case "--float":
			op.float = true
		case "--integer":
			op.integer = true
		case "--list":
			showPlugins()
			os.Exit(0)
		case "--nocolor":
			no := false
			op.color = &no
		case "--noheaders":
			op.header = false
		case "--noupdate":
			op.update = false
		case "-o", "--output":
			op.output = arg
		case "--pidfile":
			op.pidfile = arg
		case "--profile":
			op.profile = "dstat_profile.log"
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		case "-V", "--version":
			printVersion()
			os.Exit(0)
		default:
			if strings.HasPrefix(opt, "--") {
				op.plugins = append(op.plugins, opt[2:])
			} else {
				fmt.Printf("dstat: option %s unknown to getopt, try dstat -h for a list of all the options\n", opt)
				os.Exit(1)
			}
		}
	}

	if op.float && op.integer {
		fmt.Println("dstat: option --float and --integer are mutual exclusive, you can only force one")
		os.Exit(1)
	}

	if len(op.plugins) == 0 {
		fmt.Println("You did not select any stats, using -cdngy by default.")
		op.plugins = []string{"cpu", "disk", "net", "page", "sys"}
	}

	// 位置参数 delay/count
	if len(restArgs) > 0 {
		v, e := strconv.Atoi(restArgs[0])
		if e != nil {
			fmt.Println("dstat: incorrect argument, try dstat -h for the correct syntax")
			os.Exit(1)
		}
		op.delay = v
	}
	if len(restArgs) > 1 {
		v, e := strconv.Atoi(restArgs[1])
		if e != nil {
			fmt.Println("dstat: incorrect argument, try dstat -h for the correct syntax")
			os.Exit(1)
		}
		op.count = v
	}

	if op.delay <= 0 {
		fmt.Println("dstat: delay must be an integer, greater than zero")
		os.Exit(1)
	}

	if op.debug > 0 {
		fmt.Printf("Plugins: %s\n", pyList(op.plugins))
	}
	return op, nil
}

var restArgs []string

// getopt 复刻 Python getopt：短参数聚合、长参数唯一前缀匹配
func getopt(argv []string) ([]optionArg, error) {
	var out []optionArg
	i := 0
	args := []string{}
	for i < len(argv) {
		s := argv[i]
		i++
		if s == "--" {
			args = append(args, argv[i:]...)
			break
		}
		if strings.HasPrefix(s, "--") {
			body := s[2:]
			name := body
			val := ""
			hasVal := false
			if idx := strings.IndexByte(body, '='); idx >= 0 {
				name = body[:idx]
				val = body[idx+1:]
				hasVal = true
			}
			all := append(append([]string{}, longOptionNames...), internalPlugins...)
			found, exact, ambiguous := matchLong(all, name)
			if uncertain := ambiguous; uncertain {
				return nil, fmt.Errorf("option --%s not a unique prefix", name)
			}
			if found == "" {
				// 上游 getopt 只接受已注册的长选项（内置插件 + 外部插件名）
				return nil, fmt.Errorf("option --%s not recognized", name)
			}
			if longHasArg(found) {
				if !hasVal {
					if i >= len(argv) {
						return nil, fmt.Errorf("option --%s requires argument", name)
					}
					val = argv[i]
					i++
				}
			} else if hasVal {
				return nil, fmt.Errorf("option --%s must not have an argument", name)
			}
			_ = exact
			out = append(out, optionArg{"--" + found, val})
			continue
		}
		if strings.HasPrefix(s, "-") && len(s) > 1 {
			j := 1
			for j < len(s) {
				c := s[j]
				j++
				pos := strings.IndexByte(shortOptStr, c)
				if pos < 0 || c == ':' {
					return nil, fmt.Errorf("option -%c not recognized", c)
				}
				if shortOptsWithArg[c] {
					val := s[j:]
					if val == "" {
						if i >= len(argv) {
							return nil, fmt.Errorf("option -%c requires argument", c)
						}
						val = argv[i]
						i++
					}
					out = append(out, optionArg{"-" + string(c), val})
					j = len(s)
				} else {
					out = append(out, optionArg{"-" + string(c), ""})
				}
			}
			continue
		}
		args = append(args, s)
	}
	restArgs = args
	return out, nil
}

func matchLong(names []string, s string) (string, bool, bool) {
	for _, n := range names {
		if n == s {
			return n, true, false
		}
	}
	var cands []string
	for _, n := range names {
		if strings.HasPrefix(n, s) {
			cands = append(cands, n)
		}
	}
	switch len(cands) {
	case 1:
		return cands[0], false, false
	case 0:
		return "", false, false
	}
	return "", false, true
}

func longHasArg(name string) bool {
	switch name {
	case "output", "pidfile", "mods", "modules":
		return true
	}
	return false
}

func pyList(v []string) string {
	if len(v) == 0 {
		return "[]"
	}
	q := make([]string, len(v))
	for i, s := range v {
		q[i] = "'" + s + "'"
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// ---------- 入口 ----------

func main() {
	procRootEn = os.Getenv("DSTAT_PROC_ROOT")

	op, err := parseOptions(append(strings.Fields(os.Getenv("DSTAT_OPTS")), os.Args[1:]...))
	if err != nil {
		fmt.Printf("dstat: %s, try dstat -h for a list of all the options\n", err)
		os.Exit(1)
	}

	E = &engine{op: op}
	E.theme = setTheme(op.blackonwhite)
	if op.color == nil {
		c := getTermColor()
		op.color = &c
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		if op.update {
			fmt.Print("\n")
		}
		os.Exit(0)
	}()

	run()
}

func run() {
	op := E.op
	E.cpunr = getCPUNr()
	E.hz = clockTicks()
	E.maxint = maxIntVal()
	E.pagesize = pageSize()
	interval := 1

	hostname, _ := os.Hostname()
	user := currentUser()

	if !*op.color {
		E.theme = emptyTheme()
	}

	// 禁止自动换行
	if *op.color {
		fmt.Print(ansiNoLineWr)
	}

	if !op.update {
		interval = op.delay
	}

	// CSV 输出
	if op.output != "" {
		if _, err := os.Stat(op.output); os.IsNotExist(err) {
			f, err := os.Create(op.output)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create output file %s: %v\n", op.output, err)
				os.Exit(1)
			}
			E.output = f
			fmt.Fprintf(f, "\"Dstat %s CSV output\"\n", VERSION)
			fmt.Fprintf(f, "\"Author:\",\"Dag Wieers <dag@wieers.com>\",,,,\"URL:\",\"http://dag.wieers.com/home-made/dstat/\"\n")
		} else {
			f, err := os.OpenFile(op.output, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create output file %s: %v\n", op.output, err)
				os.Exit(1)
			}
			E.output = f
			fmt.Fprint(f, "\n\n")
		}
		fmt.Fprintf(E.output, "\"Host:\",\"%s\",,,,\"User:\",\"%s\"\n", hostname, user)
		fmt.Fprintf(E.output, "\"Cmdline:\",\"dstat %s\",,,,\"Date:\",\"%s\"\n",
			strings.Join(op.args, " "), time.Now().Format("02 Jan 2006 15:04:05 MST"))
	}

	if op.pidfile != "" {
		if err := os.WriteFile(op.pidfile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create pidfile %s: %v\n", op.pidfile, err)
			op.pidfile = ""
		}
	}

	// 构建插件列表（含回退链）
	for _, name := range op.plugins {
		mods := fallbackChain(name)
		for mi, mod := range mods {
			plug := newPluginByName(mod)
			if plug == nil {
				reportLoadFailure(mod, mi == len(mods)-1, fmt.Errorf("No module named dstat_%s", strings.ReplaceAll(mod, "-", "_")))
				continue
			}
			if err := plug.Prepare(); err != nil {
				reportLoadFailure(mod, mi == len(mods)-1, err)
				continue
			}
			if err := plug.Check(); err != nil {
				reportLoadFailure(mod, mi == len(mods)-1, err)
				continue
			}
			E.linewidth += plug.StatWidth() + 1
			E.totlist = append(E.totlist, plug)
			if op.debug > 0 {
				fmt.Printf("Module dstat_%s\n", strings.ReplaceAll(mod, "-", "_"))
			}
			break
		}
	}

	if len(E.totlist) == 0 {
		fmt.Fprintln(os.Stderr, "None of the stats you selected are available.")
		os.Exit(8)
	}

	if E.output != nil {
		fmt.Fprint(E.output, csvHeader(E.totlist))
	}

	E.inittime = now()
	E.update = 0
	E.missed = 0

	for E.update <= op.delay*(op.count-1) || op.count == -1 {
		// 绝对时刻调度
		target := E.inittime + float64(E.update)
		if d := target - now(); d > 0 {
			time.Sleep(time.Duration(d * float64(time.Second)))
		}
		E.perform(E.update)
		os.Stdout.Sync()
		E.update += interval
	}

	if op.update {
		fmt.Print("\n")
	}
	if E.output != nil {
		E.output.Close()
	}
	// 对齐上游 exit()：退出时输出 reset 序列，并清理 pidfile
	fmt.Print(E.ansi(ansiReset))
	if op.pidfile != "" {
		os.Remove(op.pidfile)
	}
	if op.profile != "" {
		fmt.Fprintf(os.Stderr, "dstat: profiling is not supported in this build\n")
	}
}

func fallbackChain(name string) []string {
	switch name {
	case "cpu":
		return []string{"cpu", "cpu24"}
	case "disk":
		return []string{"disk", "disk24", "disk24-old"}
	case "int":
		return []string{"int", "int24"}
	case "page":
		return []string{"page", "page24"}
	case "swap":
		return []string{"swap", "swap-old"}
	}
	return []string{name}
}

func reportLoadFailure(mod string, last bool, err error) {
	pf := "dstat_" + strings.ReplaceAll(mod, "-", "_")
	if last {
		fmt.Fprintf(os.Stderr, "Module %s failed to load. (%v)\n", pf, err)
	} else if E.op.debug > 0 {
		fmt.Fprintf(os.Stderr, "Module %s failed to load, trying another. (%v)\n", pf, err)
	}
}

func csvHeader(totlist []*Plugin) string {
	line := ""
	for i, o := range totlist {
		line += o.CsvTitle()
		if i != len(totlist)-1 {
			line += chSep
		}
	}
	line += "\n"
	for i, o := range totlist {
		line += o.CsvSubtitle()
		if i != len(totlist)-1 {
			line += chSep
		}
	}
	return line + "\n"
}

func (e *engine) header() string {
	r := e.r()
	line := ""
	for i, o := range e.vislist {
		line += o.Title(r)
		if i != len(e.vislist)-1 {
			line += e.theme.Frame + chSpace
		} else if !sameList(e.totlist, e.vislist) {
			line += e.theme.Title + chGt
		}
	}
	line += "\n"
	for i, o := range e.vislist {
		line += o.Subtitle(r)
		if i != len(e.vislist)-1 {
			line += e.theme.Frame + chPipe
		} else if !sameList(e.totlist, e.vislist) {
			line += e.theme.Title + chGt
		}
	}
	return line + "\n"
}

func (e *engine) ansi(s string) string {
	if e.op.color == nil || !*e.op.color {
		return ""
	}
	return s
}

// perform 对应上游 perform()
func (e *engine) perform(update int) {
	r := e.r()
	op := e.op
	e.starttime = now()
	e.update = update

	e.loop = pyDiv(update-1+op.delay, op.delay)
	e.step = pyMod(update-1, op.delay) + 1

	curwidth := 0
	if op.debug > 0 {
		curwidth = 8
	}

	if e.loop != 0 && e.starttime-e.inittime-float64(update) > 1 {
		e.missed++
		return
	}

	if e.loop == 0 {
		e.elapsed = ticks()
		e.rows, e.cols = 0, 0
		e.vislist = nil
		e.oldvislist = nil
		e.showheader = true
	} else {
		e.elapsed = float64(e.step)
	}

	if stdoutTTY {
		oldcols := e.cols
		e.rows, e.cols = getTermSize()
		if oldcols != e.cols {
			e.vislist = nil
			for _, o := range e.totlist {
				newwidth := curwidth + o.StatWidth() + 1
				if newwidth <= e.cols || (len(e.vislist) == len(e.totlist)-1 && newwidth < e.cols) {
					e.vislist = append(e.vislist, o)
					curwidth = newwidth
				}
			}
		}
		if op.header && e.rows >= 6 {
			if !sameList(e.oldvislist, e.vislist) {
				e.showheader = true
			} else if !op.update && e.loop%(e.rows-2) == 0 {
				e.showheader = true
			} else if op.update && e.step == 1 && e.loop%(e.rows-1) == 0 {
				e.showheader = true
			}
		}
		e.oldvislist = e.vislist
	} else {
		e.vislist = e.totlist
	}

	if e.step == op.delay {
		e.theme.Default = e.ansi(ansiReset)
	} else {
		e.theme.Default = e.theme.TextLo
	}

	newline := ""
	if op.update {
		if e.step == 1 && update != 0 {
			newline = "\n" + e.ansi(ansiReset) + e.ansi(ansiClearLine) + e.ansi(ansiSave)
		} else if e.loop != 0 {
			newline = e.ansi(ansiRestore)
		}
	}

	if e.showheader {
		if e.loop == 0 && !sameList(e.totlist, e.vislist) {
			fmt.Fprintln(os.Stderr, "Terminal width too small, trimming output.")
		}
		e.showheader = false
		fmt.Print(newline)
		newline = e.header()
	}

	line := newline
	oline := ""
	for _, o := range e.totlist {
		o.ExtractFn(o)
		if containsPlugin(e.vislist, o) {
			line += o.Show(r) + o.ShowEnd(r, e.totlist, e.vislist)
		}
		if op.output != "" && e.step == op.delay {
			oline += o.ShowCsv() + o.ShowCsvEnd(e.totlist, e.vislist)
		}
	}

	fmt.Print(line + e.theme.Input)
	if op.output != "" && e.step == op.delay {
		fmt.Fprintln(E.output, oline)
	}

	if e.missed > 0 {
		fmt.Print(" " + e.theme.Error + "missed " + strconv.Itoa(e.missed+1) + " ticks" + e.theme.Input)
		e.missed = 0
	}

	if !op.update {
		fmt.Print("\n")
	}
}

func containsPlugin(list []*Plugin, p *Plugin) bool {
	for _, x := range list {
		if x == p {
			return true
		}
	}
	return false
}

func pyDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func pyMod(a, b int) int {
	m := a % b
	if m != 0 && ((m < 0) != (b < 0)) {
		m += b
	}
	return m
}

func now() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

// ticks 对应上游 ticks()：读取 /proc/uptime
func ticks() float64 {
	lines, err := readLines("/proc/uptime")
	if err == nil && len(lines) > 0 {
		f := strings.Fields(lines[0])
		if len(f) >= 2 {
			if v, e := strconv.ParseFloat(f[0], 64); e == nil {
				return v
			}
		}
	}
	// 回退：用 /proc/stat 的 btime 推算
	if lines, err := readLines("/proc/stat"); err == nil {
		for _, l := range lines {
			f := strings.Fields(l)
			if len(f) == 2 && f[0] == "btime" {
				if bt, e := strconv.ParseFloat(f[1], 64); e == nil {
					return now() - bt
				}
			}
		}
	}
	return now()
}

func getCPUNr() int {
	if lines, err := readLines("/proc/stat"); err == nil {
		n := 0
		for _, l := range lines {
			if strings.HasPrefix(l, "cpu") {
				n++
			}
		}
		if n > 1 {
			return n - 1
		}
	}
	return 1
}

func currentUser() string {
	for _, k := range []string{"USER", "LOGNAME", "USERNAME"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return "root"
}

// ---------- 帮助/版本/插件清单 ----------

func printUsage() {
	fmt.Print(`Usage: dstat [-afv] [options..] [delay [count]]
Versatile tool for generating system resource statistics)

Dstat options:
  -c, --cpu                enable cpu stats
     -C 0,3,total             include cpu0, cpu3 and total
  -d, --disk               enable disk stats
     -D total,hda             include hda and total
  -g, --page               enable page stats
  -i, --int                enable interrupt stats
     -I 5,eth2                include int5 and interrupt used by eth2
  -l, --load               enable load stats
  -m, --mem                enable memory stats
  -n, --net                enable network stats
     -N eth1,total            include eth1 and total
  -p, --proc               enable process stats
  -r, --io                 enable io stats (I/O requests completed)
  -s, --swap               enable swap stats
     -S swap1,total           include swap1 and total
  -t, --time               enable time/date output
  -T, --epoch              enable time counter (seconds since epoch)
  -y, --sys                enable system stats

  --aio                    enable aio stats
  --fs, --filesystem       enable fs stats
  --ipc                    enable ipc stats
  --lock                   enable lock stats
  --raw                    enable raw stats
  --socket                 enable socket stats
  --tcp                    enable tcp stats
  --udp                    enable udp stats
  --unix                   enable unix stats
  --vm                     enable vm stats
  --vm-adv                 enable advanced vm stats
  --zones                  enable zoneinfo stats

  --list                   list all available plugins
  --<plugin-name>          enable external plugin by name (see --list)

  -a, --all                equals -cdngy (default)
  -f, --full               automatically expand -C, -D, -I, -N and -S lists
  -v, --vmstat             equals -pmgdsc -D total

  --bits                   force bits for values expressed in bytes
  --float                  force float values on screen
  --integer                force integer values on screen

  --bw, --black-on-white   change colors for white background terminal
  --color                  force colors
  --nocolor                disable colors
  --noheaders              disable repetitive headers
  --noupdate               disable intermediate updates
  --output file            write CSV output to file
  --profile                show profiling statistics when exiting dstat

delay is the delay in seconds between each update (default: 1)
count is the number of updates to display before exiting (default: unlimited)
`)
	fmt.Println()
}

func printVersion() {
	fmt.Printf("Dstat %s\n", VERSION)
	fmt.Println("Written by Dag Wieers <dag@wieers.com>")
	fmt.Println("Homepage at http://dag.wieers.com/home-made/dstat/")
	fmt.Println()
	fmt.Println("Platform posix/linux")
	fmt.Printf("Kernel %s\n", unameRelease())
	fmt.Println("Python (go port)")
	fmt.Println()
	color := ""
	if !getTermColor() {
		color = "no "
	}
	fmt.Printf("Terminal type: %s (%scolor support)\n", os.Getenv("TERM"), color)
	rows, cols := getTermSize()
	fmt.Printf("Terminal size: %d lines, %d columns\n", rows, cols)
	fmt.Println()
	fmt.Printf("Processors: %d\n", getCPUNr())
	fmt.Printf("Pagesize: %d\n", pageSize())
	fmt.Printf("Clock ticks per secs: %d\n", clockTicks())
	fmt.Println()
	showPlugins()
}

func showPlugins() {
	_, cols := getTermSize()
	plugins := append([]string{}, internalPlugins...)
	sortStrings(plugins)
	// 复刻上游 showplugins()：行内以逗号分隔，无空格
	fmt.Print("internal:\n\t")
	cols2 := cols - 8
	for i, mod := range plugins {
		cols2 -= len(mod) + 2
		if cols2 <= 0 {
			fmt.Print("\n\t")
			cols2 = cols - len(mod) - 10
		}
		if i != len(plugins)-1 {
			fmt.Print(mod + ",")
		}
	}
	if len(plugins) > 0 {
		fmt.Println(plugins[len(plugins)-1])
	}
}

// ---------- 小工具 ----------

func procPath(p string) string {
	if procRootEn != "" {
		return filepath.Join(procRootEn, p)
	}
	return p
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func getTermColor() bool {
	if !stdoutTTY {
		return false
	}
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	return true
}

func getTermSize() (int, int) {
	if stdoutTTY {
		if r, c, ok := termSize(int(os.Stdout.Fd())); ok && r > 0 && c > 0 {
			return r, c
		}
	}
	rows := atoiDefault(os.Getenv("LINES"), 25)
	cols := atoiDefault(os.Getenv("COLUMNS"), 80)
	return rows, cols
}

func atoiDefault(s string, d int) int {
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return d
}

func termSize(fd int) (int, int, bool) {
	return termSizePlatform(fd)
}

func clockTicks() int { return 100 }

func pageSize() int { return os.Getpagesize() }

func sortStrings(v []string) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

func strf(fmtStr string, t time.Time) string {
	return t.Format(strfConvert(fmtStr))
}

func strfConvert(f string) string {
	repl := []struct{ from, to string }{
		{"%Y", "2006"}, {"%y", "06"}, {"%m", "01"}, {"%d", "02"}, {"%e", "_2"},
		{"%H", "15"}, {"%I", "03"}, {"%M", "04"}, {"%S", "05"}, {"%p", "PM"},
		{"%b", "Jan"}, {"%B", "January"}, {"%a", "Mon"}, {"%A", "Monday"},
		{"%Z", "MST"}, {"%z", "-0700"}, {"%j", "002"}, {"%%", "\x00"},
	}
	for _, r := range repl {
		f = strings.ReplaceAll(f, r.from, r.to)
	}
	return strings.ReplaceAll(f, "\x00", "%")
}
