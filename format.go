package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ANSI 颜色表，与上游 color 字典一致
const (
	cBlack       = "\033[0;30m"
	cDarkRed     = "\033[0;31m"
	cDarkGreen   = "\033[0;32m"
	cDarkYellow  = "\033[0;33m"
	cDarkBlue    = "\033[0;34m"
	cDarkMagenta = "\033[0;35m"
	cDarkCyan    = "\033[0;36m"
	cGray        = "\033[0;37m"

	cDarkGray = "\033[1;30m"
	cRed      = "\033[1;31m"
	cGreen    = "\033[1;32m"
	cYellow   = "\033[1;33m"
	cBlue     = "\033[1;34m"
	cMagenta  = "\033[1;35m"
	cCyan     = "\033[1;36m"
	cWhite    = "\033[1;37m"

	cBlackBg = "\033[40m"
	cRedBg   = "\033[41m"
)

const (
	ansiReset     = "\033[0;0m"
	ansiBold      = "\033[1m"
	ansiUnderline = "\033[4m"
	ansiClearLine = "\033[2K"
	ansiSave      = "\033[s"
	ansiRestore   = "\033[u"
	ansiNoLineWr  = "\033[7l"
)

// 分隔符，与上游 char 字典一致
const (
	chPipe  = "|"
	chColon = ":"
	chGt    = ">"
	chSpace = " "
	chDash  = "-"
	chSep   = ","
)

type theme struct {
	Title     string
	Subtitle  string
	Frame     string
	Default   string
	Error     string
	Roundtrip string
	Debug     string
	Input     string

	DoneLo, DoneHi string
	TextLo, TextHi string
	UnitLo, UnitHi string

	ColorsLo []string
	ColorsHi []string
}

func emptyTheme() *theme {
	// 无颜色时上游把 colors_hi/colors_lo 置为 (ansi['default'],)（即空串）
	return &theme{ColorsLo: []string{""}, ColorsHi: []string{""}}
}

// setTheme 对应上游 set_theme()
func setTheme(blackOnWhite bool) *theme {
	t := &theme{
		Default:   ansiReset,
		Roundtrip: cDarkBlue,
		Debug:     cDarkRed,
		Input:     cDarkGray,
	}
	if blackOnWhite {
		t.Title = cDarkBlue
		t.Subtitle = cDarkCyan + ansiUnderline
		t.Frame = cDarkBlue
		t.Error = cWhite + cRedBg
		t.DoneLo = cBlack
		t.DoneHi = cDarkGray
		t.TextLo = cBlack
		t.TextHi = cDarkGray
		t.UnitLo = cBlack
		t.UnitHi = cDarkGray
		t.ColorsLo = []string{cDarkRed, cDarkMagenta, cDarkGreen, cDarkBlue, cDarkCyan, cBlack, cRed, cGreen}
		t.ColorsHi = []string{cRed, cMagenta, cGreen, cBlue, cCyan, cDarkGray, cDarkRed, cDarkGreen}
		return t
	}
	t.Title = cDarkBlue
	t.Subtitle = cBlue + ansiUnderline
	t.Frame = cDarkBlue
	t.Error = cWhite + cRedBg
	t.DoneLo = cWhite
	t.DoneHi = cGray
	t.TextLo = cGray
	t.TextHi = cDarkGray
	t.UnitLo = cDarkGray
	t.UnitHi = cDarkGray
	t.ColorsLo = []string{cRed, cYellow, cGreen, cBlue, cCyan, cWhite, cDarkRed, cDarkGreen}
	t.ColorsHi = []string{cDarkRed, cDarkYellow, cDarkGreen, cDarkBlue, cDarkCyan, cGray, cRed, cGreen}
	return t
}

// pyCenter 复刻 Python str.center 的补空格规则
func pyCenter(s string, width int) string {
	if len(s) >= width {
		return s
	}
	marg := width - len(s)
	left := marg/2 + (marg & width & 1)
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", marg-left)
}

func rjust(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

func ljust(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// pyRound 对应 Python 2 的 round()：四舍五入（远离零）
func pyRound(v float64) float64 {
	if v < 0 {
		return -math.Floor(-v + 0.5)
	}
	return math.Floor(v + 0.5)
}

// pyRoundND 对应 round(v, nd)
func pyRoundND(v float64, nd int) float64 {
	p := math.Pow(10, float64(nd))
	return pyRound(v*p) / p
}

// dchg 对应上游 dchg()
func dchg(v float64, width int, base float64) (string, int) {
	c := 0
	for {
		ret := strconv.FormatInt(int64(pyRound(v)), 10)
		if len(ret) <= width {
			return ret, c
		}
		v /= base
		c++
		if c > 8 {
			return ret, c
		}
	}
}

// fchg 对应上游 fchg()
func fchg(v float64, width int, base float64) (string, int) {
	c := 0
	for {
		if v == 0 {
			return "0", c
		}
		orig := strconv.FormatInt(int64(math.Trunc(pyRoundND(v, width))), 10)
		ret := orig
		if len(ret) <= width {
			i := width - len(ret) - 1
			ok := false
			for i > 0 {
				cand := fmt.Sprintf("%.*f", i, v)
				if len(cand) <= width && cand != orig {
					ret = cand
					ok = true
					break
				}
				i--
			}
			if !ok {
				ret = strconv.FormatInt(int64(pyRound(v)), 10)
			}
			return ret, c
		}
		v /= base
		c++
		if c > 8 {
			return ret, c
		}
	}
}

// tchg 对应上游 tchg()
func tchg(v float64, width int) string {
	ret := fmt.Sprintf("%2dh%02d", int(v)/60, int(v)%60)
	if len(ret) > width {
		ret = fmt.Sprintf("%2dh", int(v)/60)
		if len(ret) > width {
			ret = fmt.Sprintf("%2dd", int(v)/60/24)
			if len(ret) > width {
				ret = fmt.Sprintf("%2dw", int(v)/60/24/7)
			}
		}
	}
	return ret
}

// cprint 数值格式化，对应上游 cprint()
func (r *renderer) cprint(v float64, ctype string, width int, scale float64) string {
	base := 1000.0
	if scale == 1024 {
		base = 1024
	}

	unit := false
	if (scale == 1000 || scale == 1024) && width >= len(strconv.Itoa(int(base))) {
		unit = true
		width--
	}

	if ctype != "s" && v < 0 {
		if unit {
			return r.theme.Error + rjust("-", width) + chSpace + r.theme.Default
		}
		return r.theme.Error + rjust("-", width) + r.theme.Default
	}

	var units []string
	if base != 1024 {
		units = []string{chSpace, "k", "M", "G", "T", "P", "E", "Z", "Y"}
	} else if r.E.op.bits && ctype == "b" {
		units = []string{"b", "k", "M", "G", "T", "P", "E", "Z", "Y"}
		base = 1000
		scale = 1000
		v *= 8.0
	} else {
		units = []string{"B", "k", "M", "G", "T", "P", "E", "Z", "Y"}
	}

	var colors []string
	var ctext, cunit, cdone string
	if r.E.step == r.E.op.delay {
		colors = r.theme.ColorsLo
		ctext = r.theme.TextLo
		cunit = r.theme.UnitLo
		cdone = r.theme.DoneLo
	} else {
		colors = r.theme.ColorsHi
		ctext = r.theme.TextHi
		cunit = r.theme.UnitHi
		cdone = r.theme.DoneHi
	}

	var ret string
	var c int
	switch {
	case r.E.op.integer && (ctype == "b" || ctype == "d" || ctype == "p" || ctype == "f"):
		ret, c = dchg(v, width, base)
	case r.E.op.float && (ctype == "b" || ctype == "d" || ctype == "p" || ctype == "f"):
		ret, c = fchg(v, width, base)
	case ctype == "b" || ctype == "d" || ctype == "p":
		ret, c = dchg(v, width, base)
	case ctype == "f":
		ret, c = fchg(v, width, base)
	case ctype == "t":
		ret, c = tchg(v, width), 0
	default:
		panic("Type " + ctype + " not known to dstat.")
	}

	var col string
	switch {
	case ret == "0":
		col = cunit
	case scale <= 0:
		col = ctext
	case ctype == "p" && pyRound(v) >= 100.0:
		col = cdone
	case scale != 1000 && scale != 1024:
		col = colors[int(v/scale)%len(colors)]
	case ctype == "b" || ctype == "d" || ctype == "f":
		col = colors[c%len(colors)]
	default:
		col = ctext
	}

	ret = col + rjust(ret, width)

	if unit {
		if c != -1 && pyRound(v) != 0 {
			ret += cunit + units[c]
		} else {
			ret += chSpace
		}
	}
	return ret
}

// cprintStr 对应 cprint(type='s')
func (r *renderer) cprintStr(s string, width int) string {
	var ctext string
	if r.E.step == r.E.op.delay {
		ctext = r.theme.TextLo
	} else {
		ctext = r.theme.TextHi
	}
	return ctext + ljust(s, width)
}

func (r *renderer) cprintList(list []float64, ctype string, width int, scale float64) string {
	parts := make([]string, len(list))
	for i, v := range list {
		parts[i] = r.cprint(v, ctype, width, scale)
	}
	return strings.Join(parts, chSpace)
}
