// Package cron 提供用户 crontab 的解析、编辑与 cron 表达式计算。
//
// 表达式为标准 5 字段格式（支持 * 、列表、区间、步进）：
//
//	分 时 日 月 周    例：*/5 * * * *（每 5 分钟）、0 3 * * 1-5（工作日每天 3 点）
//
// crontab 文本解析见 crontab.go。
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Expr 是解析后的 cron 表达式，用于计算下一次执行时间。
type Expr struct {
	minutes [60]bool
	hours   [24]bool
	dom     [32]bool // day of month，1-31（下标 0 弃用）
	months  [13]bool // 1-12（下标 0 弃用）
	dow     [7]bool  // day of week，0-6 = 周日-周六
	domAll  bool     // 日字段是否为 *（涉及标准 cron 的日/周组合语义）
	dowAll  bool
}

// Parse 解析 5 字段 cron 表达式。
func Parse(expr string) (*Expr, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("表达式需为 5 个字段（分 时 日 月 周），当前 %d 个", len(fields))
	}

	e := &Expr{}
	if err := parseField(fields[0], 0, 59, e.minutes[:]); err != nil {
		return nil, fmt.Errorf("分钟字段无效: %w", err)
	}
	if err := parseField(fields[1], 0, 23, e.hours[:]); err != nil {
		return nil, fmt.Errorf("小时字段无效: %w", err)
	}
	if err := parseField(fields[2], 1, 31, e.dom[:]); err != nil {
		return nil, fmt.Errorf("日字段无效: %w", err)
	}
	if err := parseField(fields[3], 1, 12, e.months[:]); err != nil {
		return nil, fmt.Errorf("月字段无效: %w", err)
	}
	// 周 7 视为 0（周日）
	dowSet := map[int]bool{}
	if err := parseFieldFunc(fields[4], 0, 7, func(v int) { dowSet[v%7] = true }); err != nil {
		return nil, fmt.Errorf("周字段无效: %w", err)
	}
	for v := range dowSet {
		e.dow[v] = true
	}

	e.domAll = fields[2] == "*"
	e.dowAll = fields[4] == "*"
	return e, nil
}

// Next 返回 after 之后的下一次执行时间；找不到（5 年内无匹配）返回零值。
func (e *Expr) Next(after time.Time) time.Time {
	// 从下一分钟开始向后逐级跳转：月 → 日 → 时 → 分
	t := after.Truncate(time.Minute).Add(time.Minute)
	limit := t.AddDate(5, 0, 0)
	for t.Before(limit) {
		if !e.months[int(t.Month())] {
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, 0)
			continue
		}
		if !e.dayMatch(t) {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, 1)
			continue
		}
		if !e.hours[t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location()).Add(time.Hour)
			continue
		}
		if !e.minutes[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	return time.Time{}
}

// dayMatch 按标准 cron 语义判断日匹配：
// 日与周均为 * 时每天匹配；仅一个为 * 时按受限字段；均受限时满足其一即可。
func (e *Expr) dayMatch(t time.Time) bool {
	domMatch := e.dom[t.Day()]
	dowMatch := e.dow[int(t.Weekday())]
	switch {
	case e.domAll && e.dowAll:
		return true
	case e.domAll:
		return dowMatch
	case e.dowAll:
		return domMatch
	default:
		return domMatch || dowMatch
	}
}

// parseField 解析单个字段到布尔集合，支持 *、a、a-b、*/n、a-b/n、逗号列表。
func parseField(field string, min, max int, set []bool) error {
	return parseFieldFunc(field, min, max, func(v int) { set[v] = true })
}

// parseFieldFunc 解析单个字段并对每个命中的值调用 fn。
func parseFieldFunc(field string, min, max int, fn func(int)) error {
	for _, part := range strings.Split(field, ",") {
		if part == "" {
			return fmt.Errorf("存在空片段")
		}
		rangePart, step := part, 1
		if idx := strings.Index(part, "/"); idx >= 0 {
			rangePart = part[:idx]
			stepStr := part[idx+1:]
			n, err := strconv.Atoi(stepStr)
			if err != nil || n <= 0 {
				return fmt.Errorf("步进 %q 无效", stepStr)
			}
			step = n
		}

		lo, hi := min, max
		switch {
		case rangePart == "*":
			// 保持默认区间
		case strings.Contains(rangePart, "-"):
			bounds := strings.SplitN(rangePart, "-", 2)
			a, err1 := strconv.Atoi(bounds[0])
			b, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil {
				return fmt.Errorf("区间 %q 无效", rangePart)
			}
			lo, hi = a, b
		default:
			v, err := strconv.Atoi(rangePart)
			if err != nil {
				return fmt.Errorf("值 %q 无效", rangePart)
			}
			lo, hi = v, v
		}
		if lo < min || hi > max || lo > hi {
			return fmt.Errorf("区间 %d-%d 超出范围 [%d, %d]", lo, hi, min, max)
		}
		for v := lo; v <= hi; v += step {
			fn(v)
		}
	}
	return nil
}
