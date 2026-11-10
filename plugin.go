package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// Plugin 对应上游 dstat 基类；内部插件通过函数钩子实现多态
type Plugin struct {
	Name  any // string 或 []string
	Nick  []string
	Vars  []string
	Type  string
	Width int
	Scale float64
	Cols  int

	Val  map[string]any
	Set1 map[string]any
	Set2 map[string]any

	File string
	// Files 对应上游 __init__ 里的 self.open()：任一文件不可读即视为插件加载失败
	Files   []string
	loadErr error

	discovered []string
	hasDisc    bool

	ExtractFn  func(p *Plugin)
	DiscoverFn func(p *Plugin) []string
	VarsFn     func(p *Plugin) []string
	NameFn     func(p *Plugin) []string
}

func newPlugin() *Plugin {
	return &Plugin{Type: "f", Width: 5, Scale: 1024}
}

// Prepare 对应上游 prepare()
func (p *Plugin) Prepare() error {
	if p.loadErr != nil {
		return p.loadErr
	}
	// 对齐上游 open()：逐个尝试，全部失败才报错，报错取最后一个文件名
	if len(p.Files) > 0 {
		ok := false
		last := ""
		for _, f := range p.Files {
			last = f
			if _, err := os.ReadFile(procPath(f)); err == nil {
				ok = true
			}
		}
		if !ok {
			p.loadErr = fmt.Errorf("Cannot open file %s", last)
		}
	}
	if p.loadErr != nil {
		return p.loadErr
	}
	if p.DiscoverFn != nil {
		p.discovered = p.DiscoverFn(p)
		p.hasDisc = true
	}
	if p.loadErr != nil {
		return p.loadErr
	}
	if p.VarsFn != nil {
		p.Vars = p.VarsFn(p)
	}
	if len(p.Vars) == 0 {
		return fmt.Errorf("No counter objects to monitor")
	}
	if p.NameFn != nil {
		p.Name = p.NameFn(p)
	}
	if len(p.Nick) == 0 {
		p.Nick = p.Vars
	}

	p.Val = map[string]any{}
	p.Set1 = map[string]any{}
	p.Set2 = map[string]any{}

	if p.Cols > 0 {
		for _, name := range append(append([]string{}, p.Vars...), "total") {
			p.Val[name] = make([]float64, p.Cols)
			p.Set1[name] = make([]float64, p.Cols)
			p.Set2[name] = make([]float64, p.Cols)
		}
	} else {
		for _, name := range p.Vars {
			p.Val[name] = float64(0)
			p.Set1[name] = float64(0)
			p.Set2[name] = float64(0)
		}
	}
	return nil
}

// Check 对应上游 check()
func (p *Plugin) Check() error {
	if len(p.Vars) == 0 {
		return fmt.Errorf("No objects found, no stats available")
	}
	if p.DiscoverFn != nil && len(p.discovered) == 0 {
		return fmt.Errorf("No objects discovered, no stats available")
	}
	if p.ColWidth() <= 0 {
		return fmt.Errorf("Unknown problem, please report")
	}
	return nil
}

func (p *Plugin) colCount() int {
	if p.Cols > 0 {
		return p.Cols
	}
	return 1
}

func (p *Plugin) ColWidth() int {
	if _, ok := p.Name.(string); ok {
		return p.Width
	}
	n := len(p.Nick)
	if n == 0 {
		n = 1
	}
	return n*p.Width + n - 1
}

func (p *Plugin) StatWidth() int {
	if p.Name == nil {
		return 0
	}
	if _, ok := p.Name.(string); ok {
		return len(p.Nick)*p.ColWidth() + len(p.Nick) - 1
	}
	return len(p.Name.([]string))*p.ColWidth() + len(p.Name.([]string)) - 1
}

// Title 对应上游 title()
func (p *Plugin) Title(r *renderer) string {
	if name, ok := p.Name.(string); ok {
		width := p.StatWidth()
		return r.theme.Title + strings.ReplaceAll(pyCenter(truncStr(name, width), width), " ", "-") + r.theme.Default
	}
	names, _ := p.Name.([]string)
	var b strings.Builder
	b.WriteString(r.theme.Title)
	for i, name := range names {
		width := p.ColWidth()
		b.WriteString(strings.ReplaceAll(pyCenter(truncStr(name, width), width), " ", "-"))
		if i+1 != len(names) {
			if *r.E.op.color {
				b.WriteString(r.theme.Frame + chDash + r.theme.Title)
			} else {
				b.WriteString(chSpace)
			}
		}
	}
	return b.String()
}

// Subtitle 对应上游 subtitle()
func (p *Plugin) Subtitle(r *renderer) string {
	var b strings.Builder
	if _, ok := p.Name.(string); ok {
		for i, nick := range p.Nick {
			b.WriteString(r.theme.Subtitle + pyCenter(truncStr(nick, p.Width), p.Width) + r.theme.Default)
			if i+1 != len(p.Nick) {
				b.WriteString(chSpace)
			}
		}
		return b.String()
	}
	names, _ := p.Name.([]string)
	for i := range names {
		for j, nick := range p.Nick {
			b.WriteString(r.theme.Subtitle + pyCenter(truncStr(nick, p.Width), p.Width) + r.theme.Default)
			if j+1 != len(p.Nick) {
				b.WriteString(chSpace)
			}
		}
		if i+1 != len(names) {
			b.WriteString(r.theme.Frame + chColon)
		}
	}
	return b.String()
}

func (p *Plugin) CsvTitle() string {
	if name, ok := p.Name.(string); ok {
		return `"` + name + `"` + strings.Repeat(chSep, len(p.Nick)-1)
	}
	names, _ := p.Name.([]string)
	var b strings.Builder
	for i, name := range names {
		b.WriteString(`"` + name + `"` + strings.Repeat(chSep, len(p.Nick)-1))
		if i+1 != len(names) {
			b.WriteString(chSep)
		}
	}
	return b.String()
}

func (p *Plugin) CsvSubtitle() string {
	var b strings.Builder
	if _, ok := p.Name.(string); ok {
		for i, nick := range p.Nick {
			b.WriteString(`"` + nick + `"`)
			if i+1 != len(p.Nick) {
				b.WriteString(chSep)
			}
		}
		return b.String()
	}
	names, _ := p.Name.([]string)
	if len(names) == 1 {
		for i := range names {
			for j, nick := range p.Nick {
				b.WriteString(`"` + nick + `"`)
				if j+1 != len(p.Nick) {
					b.WriteString(chSep)
				}
			}
			if i+1 != len(names) {
				b.WriteString(chSep)
			}
		}
		return b.String()
	}
	for i, name := range names {
		for j, nick := range p.Nick {
			b.WriteString(`"` + name + `:` + nick + `"`)
			if j+1 != len(p.Nick) {
				b.WriteString(chSep)
			}
		}
		if i+1 != len(names) {
			b.WriteString(chSep)
		}
	}
	return b.String()
}

// Show 对应上游 show()
func (p *Plugin) Show(r *renderer) string {
	var b strings.Builder
	for i, name := range p.Vars {
		ctype := p.Type
		scale := p.Scale
		switch v := p.Val[name].(type) {
		case []float64:
			b.WriteString(r.cprintList(v, ctype, p.Width, scale))
			if i+1 != len(p.Vars) {
				b.WriteString(r.theme.Frame + chColon)
			}
		case string:
			if i >= len(p.Nick) {
				return b.String()
			}
			b.WriteString(r.cprintStr(v, p.Width))
			if i+1 != len(p.Nick) {
				b.WriteString(chSpace)
			}
		default:
			if i >= len(p.Nick) {
				return b.String()
			}
			b.WriteString(r.cprint(toFloat(v), ctype, p.Width, scale))
			if i+1 != len(p.Nick) {
				b.WriteString(chSpace)
			}
		}
	}
	return b.String()
}

// ShowEnd 对应上游 showend()
func (p *Plugin) ShowEnd(r *renderer, totlist, vislist []*Plugin) string {
	if len(vislist) > 0 && p != vislist[len(vislist)-1] {
		return r.theme.Frame + chPipe
	}
	if !sameList(totlist, vislist) {
		return r.theme.Frame + chGt
	}
	return ""
}

func (p *Plugin) ShowCsv() string {
	var b strings.Builder
	for i, name := range p.Vars {
		switch v := p.Val[name].(type) {
		case []float64:
			for j, val := range v {
				b.WriteString(printCsv(val))
				if j+1 != len(v) {
					b.WriteString(chSep)
				}
			}
		case string:
			b.WriteString(v)
		default:
			b.WriteString(printCsv(toFloat(v)))
		}
		if i+1 != len(p.Vars) {
			b.WriteString(chSep)
		}
	}
	return b.String()
}

func (p *Plugin) ShowCsvEnd(totlist, vislist []*Plugin) string {
	if len(vislist) > 0 && p != vislist[len(vislist)-1] {
		return chSep
	}
	if len(totlist) > 0 && p != totlist[len(totlist)-1] {
		return chSep
	}
	return ""
}

func printCsv(v float64) string {
	// 与 cprint 一致：NaN/±Inf 按 0 写出，避免 int64 转换得到 -9223372036854775808
	if math.IsNaN(v) || math.IsInf(v, 0) {
		v = 0
	}
	if v != pyRound(v) {
		return fmt.Sprintf("%.3f", v)
	}
	return strconv.FormatInt(int64(pyRound(v)), 10)
}

func sameList(a, b []*Plugin) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func truncStr(s string, width int) string {
	if len(s) > width {
		return s[:width]
	}
	return s
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func vec(v any) []float64 {
	if x, ok := v.([]float64); ok {
		return x
	}
	return nil
}

func sumFloat(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s
}

func roundInt(v float64) int {
	return int(pyRound(v))
}

func maxIntVal() float64 { return math.MaxInt64 }
