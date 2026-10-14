// crontab.go 解析与序列化用户 crontab 文本。
//
// 设计目标是无损编辑：保留原始行（注释、空行、环境变量赋值），
// 仅对任务行做增删改，写回时不破坏文件中的其他内容。
//
// 任务识别规则：
//   - @宏行：@reboot / @yearly / @annually / @monthly / @weekly / @daily / @midnight / @hourly
//   - 5 字段行：分 时 日 月 周 + 命令（前 5 字段须通过表达式校验）
//   - 禁用任务：行首 # 注释掉的任务行（剥掉 # 后能解析为任务行）
//   - 任务名：紧邻任务行上方的 "# name: xxx" 注释
package cron

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Entry 是 crontab 中的一个定时任务条目。
type Entry struct {
	ID      string `json:"id"`      // 内容摘要（表达式+命令），启用/禁用切换前后保持稳定
	Name    string `json:"name"`    // 任务名（"# name: xxx" 注释，可为空）
	Expr    string `json:"expr"`    // 5 字段表达式或 @宏
	Command string `json:"command"` // 命令原样
	Enabled bool   `json:"enabled"` // false 表示该行被 # 注释禁用

	line     int // 任务行下标（Document.Lines 内）
	nameLine int // 名称注释行下标，-1 表示无
}

// Document 是一次 crontab 解析结果：原始行 + 识别出的任务。
type Document struct {
	Lines []string
	Jobs  []*Entry
}

var (
	envLineRe  = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	nameLineRe = regexp.MustCompile(`^#\s*name\s*:\s*(.*?)\s*$`)
	macroRe    = regexp.MustCompile(`^@(reboot|yearly|annually|monthly|weekly|daily|midnight|hourly)\b`)
)

// ParseDocument 解析 crontab 文本为文档（所有行原样保留，任务行结构化）。
func ParseDocument(content string) *Document {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	d := &Document{Lines: lines}

	pendingName, pendingNameLine := "", -1
	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)

		// 空行 / 环境变量赋值：中断名称关联
		if trimmed == "" || envLineRe.MatchString(raw) {
			pendingName, pendingNameLine = "", -1
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			// "# name: xxx" 注释：名称作用于下一个任务行
			if m := nameLineRe.FindStringSubmatch(trimmed); m != nil {
				pendingName, pendingNameLine = m[1], i
				continue
			}
			// # 注释的任务行 → 禁用任务；其余视为普通注释
			inner := strings.TrimLeft(trimmed[1:], " \t")
			if e, ok := parseJobLine(inner); ok {
				e.Enabled = false
				e.Name, e.nameLine = pendingName, pendingNameLine
				e.line = i
				d.Jobs = append(d.Jobs, e)
			}
			pendingName, pendingNameLine = "", -1
			continue
		}

		// 未注释行：任务行或无法识别的行（原样保留）
		if e, ok := parseJobLine(raw); ok {
			e.Enabled = true
			e.Name, e.nameLine = pendingName, pendingNameLine
			e.line = i
			d.Jobs = append(d.Jobs, e)
		}
		pendingName, pendingNameLine = "", -1
	}
	return d
}

// Render 将文档序列化回 crontab 文本。
func (d *Document) Render() string {
	return strings.Join(d.Lines, "\n")
}

// AddJob 在 crontab 末尾追加任务（expr 须已通过 ValidateExpr 校验）。
// 追加格式：可选空行分隔 + 可选 "# name: xxx" + 任务行。
func (d *Document) AddJob(name, expr, cmd string) error {
	if err := ValidateExpr(expr); err != nil {
		return err
	}
	if strings.TrimSpace(cmd) == "" {
		return fmt.Errorf("执行命令不能为空")
	}

	// 已有内容且末行非空时，先补一个空行分隔
	if n := len(d.Lines); n > 1 || strings.TrimSpace(d.Lines[0]) != "" {
		if strings.TrimSpace(d.Lines[n-1]) != "" {
			d.Lines = append(d.Lines, "")
		}
	}
	if strings.TrimSpace(name) != "" {
		d.Lines = append(d.Lines, "# name: "+strings.TrimSpace(name))
	}
	d.Lines = append(d.Lines, expr+" "+cmd)
	d.resync()
	return nil
}

// UpdateJob 修改任务的表达式、命令与名称；对应字段传空表示保留原值。
// 任务启用状态不变。任务不存在返回 os.ErrNotExist。
func (d *Document) UpdateJob(id, name, expr, cmd string) error {
	e := d.find(id)
	if e == nil {
		return os.ErrNotExist
	}
	newExpr, newCmd := e.Expr, e.Command
	if strings.TrimSpace(expr) != "" {
		if err := ValidateExpr(expr); err != nil {
			return err
		}
		newExpr = expr
	}
	if strings.TrimSpace(cmd) != "" {
		newCmd = cmd
	}
	if strings.TrimSpace(name) != "" {
		name = strings.TrimSpace(name)
	} else {
		name = e.Name
	}

	jobLine := newExpr + " " + newCmd
	if !e.Enabled {
		jobLine = "# " + jobLine
	}

	start, end := e.line, e.line
	if e.nameLine >= 0 {
		start = e.nameLine
	}
	repl := make([]string, 0, 2)
	if name != "" {
		repl = append(repl, "# name: "+name)
	}
	repl = append(repl, jobLine)

	out := make([]string, 0, len(d.Lines)+len(repl))
	out = append(out, d.Lines[:start]...)
	out = append(out, repl...)
	out = append(out, d.Lines[end+1:]...)
	d.Lines = out
	d.resync()
	return nil
}

// RemoveJob 删除任务及其名称注释行。任务不存在返回 os.ErrNotExist。
func (d *Document) RemoveJob(id string) error {
	e := d.find(id)
	if e == nil {
		return os.ErrNotExist
	}

	start, end := e.line, e.line
	if e.nameLine >= 0 {
		start = e.nameLine
	}
	out := make([]string, 0, len(d.Lines))
	out = append(out, d.Lines[:start]...)
	out = append(out, d.Lines[end+1:]...)
	d.Lines = out
	d.resync()
	return nil
}

// SetJobEnabled 启用/禁用任务（行首加减 "# " 注释）。任务不存在返回 os.ErrNotExist。
func (d *Document) SetJobEnabled(id string, enabled bool) error {
	e := d.find(id)
	if e == nil {
		return os.ErrNotExist
	}
	if e.Enabled == enabled {
		return nil
	}
	if enabled {
		d.Lines[e.line] = strings.TrimLeft(strings.TrimPrefix(d.Lines[e.line], "#"), " \t")
	} else {
		d.Lines[e.line] = "# " + strings.TrimSpace(d.Lines[e.line])
	}
	d.resync()
	return nil
}

// find 按 ID 查找任务。
func (d *Document) find(id string) *Entry {
	for _, e := range d.Jobs {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// resync 在行被修改后重新解析，刷新任务列表与行下标。
func (d *Document) resync() {
	nd := ParseDocument(d.Render())
	d.Jobs = nd.Jobs
}

// parseJobLine 尝试将一行解析为任务行；不是任务行（含表达式非法的行）返回 false。
func parseJobLine(line string) (*Entry, bool) {
	trimmed := strings.TrimSpace(line)

	// @宏行
	if m := macroRe.FindStringSubmatch(trimmed); m != nil {
		cmd := strings.TrimSpace(trimmed[len(m[0]):])
		if cmd == "" {
			return nil, false
		}
		e := &Entry{Expr: "@" + m[1], Command: cmd}
		e.ID = jobLineID(e.Expr, e.Command)
		return e, true
	}

	// 5 字段行
	expr, cmd, ok := split5Fields(trimmed)
	if !ok {
		return nil, false
	}
	if _, err := Parse(expr); err != nil {
		return nil, false
	}
	e := &Entry{Expr: expr, Command: cmd}
	e.ID = jobLineID(e.Expr, e.Command)
	return e, true
}

// split5Fields 拆出前 5 个空白分隔字段与其后的命令（命令保留内部空白）。
func split5Fields(line string) (expr, cmd string, ok bool) {
	rest := line
	fields := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" {
			return "", "", false
		}
		if j := strings.IndexAny(rest, " \t"); j >= 0 {
			fields = append(fields, rest[:j])
			rest = rest[j+1:]
		} else {
			fields = append(fields, rest)
			rest = ""
		}
	}
	cmd = strings.TrimLeft(rest, " \t")
	if cmd == "" {
		return "", "", false
	}
	return strings.Join(fields, " "), cmd, true
}

// ValidateExpr 校验任务表达式：@宏或合法 5 字段表达式。
func ValidateExpr(expr string) error {
	expr = strings.TrimSpace(expr)
	if m := macroRe.FindStringSubmatch(expr); m != nil {
		return nil
	}
	_, err := Parse(expr)
	return err
}

// NextRuns 计算表达式（5 字段或 @宏）在当前时刻之后的 n 次执行时间。
// @reboot 等无固定排期或表达式非法时返回 nil。
func NextRuns(expr string, n int) []time.Time {
	base := strings.TrimSpace(expr)
	if m := macroRe.FindStringSubmatch(base); m != nil {
		mapped, ok := macroToExpr(base)
		if !ok {
			return nil
		}
		base = mapped
	}
	e, err := Parse(base)
	if err != nil {
		return nil
	}
	out := make([]time.Time, 0, n)
	t := time.Now()
	for i := 0; i < n; i++ {
		t = e.Next(t)
		if t.IsZero() {
			break
		}
		out = append(out, t)
	}
	return out
}

// macroToExpr 将 @宏映射为等价 5 字段表达式；@reboot 无固定排期返回 false。
func macroToExpr(macro string) (string, bool) {
	switch macro {
	case "@yearly", "@annually":
		return "0 0 1 1 *", true
	case "@monthly":
		return "0 0 1 * *", true
	case "@weekly":
		return "0 0 * * 0", true
	case "@daily", "@midnight":
		return "0 0 * * *", true
	case "@hourly":
		return "0 * * * *", true
	}
	return "", false
}

// jobLineID 计算任务行内容摘要 ID（启用/禁用切换前后稳定）。
func jobLineID(expr, cmd string) string {
	sum := sha1.Sum([]byte(expr + "\x00" + cmd))
	return hex.EncodeToString(sum[:])[:12]
}
