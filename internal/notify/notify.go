// Package notify 提供站内通知 + 可选的 SMTP 邮件通知。
//
// Notifier 内嵌 storage.NotificationStore：站内读写直接透传；
// Add 额外支持在启用邮件通知时异步发送邮件（不阻塞请求、失败仅记日志）。
package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/storage"
)

// metaPrefix SMTP 配置在 meta 表中的键前缀（api 层将其列为保留键，禁止通用设置 API 读写）。
const metaPrefix = "smtp_"

// sendTimeout 单次邮件发送超时。
const sendTimeout = 15 * time.Second

// MailConfig SMTP 邮件通知配置。
type MailConfig struct {
	Enabled    bool   // 是否启用邮件通知
	Host       string // SMTP 服务器地址
	Port       int    // 端口：465(SSL) / 587(STARTTLS) / 25(none)
	User       string // 认证账号
	Password   string // 认证密码 / 授权码
	From       string // 发件人地址，空则使用 User
	To         string // 收件人，多个用逗号分隔
	Encryption string // none | ssl | starttls
}

// Notifier 站内通知 + 可选邮件通知。
type Notifier struct {
	*storage.NotificationStore
	db  *storage.DB
	log *logger.Logger
}

// New 创建通知服务。
func New(db *storage.DB, log *logger.Logger) *Notifier {
	return &Notifier{
		NotificationStore: storage.NewNotificationStore(db),
		db:                db,
		log:               log.Named("notify"),
	}
}

// Add 写入站内通知；若邮件通知已启用则异步发送邮件（不阻塞、失败仅记日志）。
func (n *Notifier) Add(ctx context.Context, level, module, title, detail string) error {
	if err := n.NotificationStore.Add(ctx, level, module, title, detail); err != nil {
		return err
	}
	cfg := n.EmailConfig(ctx)
	if !cfg.Enabled || cfg.Host == "" || cfg.To == "" {
		return nil
	}
	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()
		subject := fmt.Sprintf("【%s】%s", levelLabel(level), title)
		body := detail
		if body == "" {
			body = title
		}
		if err := n.sendMail(sendCtx, cfg, splitEmails(cfg.To), subject, body); err != nil {
			n.log.Errorw("send notification email failed", "title", title, "error", err)
		}
	}()
	return nil
}

// EmailConfig 从 meta 读取邮件配置（含密码，仅服务端使用；对外展示用 EmailConfigMasked）。
func (n *Notifier) EmailConfig(ctx context.Context) MailConfig {
	cfg := MailConfig{
		Enabled:    n.meta(ctx, "enabled") == "1",
		Host:       n.meta(ctx, "host"),
		User:       n.meta(ctx, "user"),
		Password:   n.meta(ctx, "pass"),
		From:       n.meta(ctx, "from"),
		To:         n.meta(ctx, "to"),
		Encryption: n.meta(ctx, "encryption"),
	}
	cfg.Port, _ = strconv.Atoi(n.meta(ctx, "port"))
	if cfg.Port <= 0 {
		cfg.Port = 465
	}
	if cfg.Encryption == "" {
		cfg.Encryption = "ssl"
	}
	return cfg
}

// EmailConfigMasked 返回脱敏后的配置（不含密码），has_password 标记是否已保存密码。
func (n *Notifier) EmailConfigMasked(ctx context.Context) (MailConfig, bool) {
	cfg := n.EmailConfig(ctx)
	return MailConfig{
		Enabled:    cfg.Enabled,
		Host:       cfg.Host,
		Port:       cfg.Port,
		User:       cfg.User,
		From:       cfg.From,
		To:         cfg.To,
		Encryption: cfg.Encryption,
	}, cfg.Password != ""
}

// SaveEmailConfig 保存邮件配置；keepPassword 为 true 时保留已存密码。
func (n *Notifier) SaveEmailConfig(ctx context.Context, cfg MailConfig, keepPassword bool) error {
	sets := map[string]string{
		metaPrefix + "enabled":    boolStr(cfg.Enabled),
		metaPrefix + "host":       cfg.Host,
		metaPrefix + "port":       strconv.Itoa(cfg.Port),
		metaPrefix + "user":       cfg.User,
		metaPrefix + "from":       cfg.From,
		metaPrefix + "to":         cfg.To,
		metaPrefix + "encryption": cfg.Encryption,
	}
	if !keepPassword {
		sets[metaPrefix+"pass"] = cfg.Password
	}
	for k, v := range sets {
		if err := n.db.SetMeta(ctx, k, v); err != nil {
			return err
		}
	}
	return nil
}

// SendTest 发送测试邮件；to 为空时使用配置中的收件人。
func (n *Notifier) SendTest(ctx context.Context, cfg MailConfig, to string) error {
	if cfg.Host == "" {
		return fmt.Errorf("请先填写 SMTP 服务器地址")
	}
	if to == "" {
		to = cfg.To
	}
	recipients := splitEmails(to)
	if len(recipients) == 0 {
		return fmt.Errorf("请填写收件人邮箱")
	}
	from := cfg.From
	if from == "" {
		from = cfg.User
	}
	subject := "邮件通知测试"
	body := fmt.Sprintf("这是一封来自 %s 的测试邮件。\n\n若您收到此邮件，说明 SMTP 邮件通知配置正确。", time.Now().Format("2006-01-02 15:04:05"))
	return n.sendMail(ctx, cfg, recipients, subject, body)
}

// sendMail 按 Encryption 建立连接并发送邮件。
func (n *Notifier) sendMail(ctx context.Context, cfg MailConfig, to []string, subject, body string) error {
	host := cfg.Host
	addr := net.JoinHostPort(host, strconv.Itoa(cfg.Port))
	from := cfg.From
	if from == "" {
		from = cfg.User
	}

	var conn net.Conn
	var err error
	d := &net.Dialer{Timeout: sendTimeout}
	switch strings.ToLower(cfg.Encryption) {
	case "ssl":
		conn, err = tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: host})
	default:
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("连接 %s 失败: %w", addr, err)
	}

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("建立 SMTP 会话失败: %w", err)
	}
	defer c.Close()

	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)

	// STARTTLS：服务器支持时升级加密
	if strings.ToLower(cfg.Encryption) == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return fmt.Errorf("STARTTLS 升级失败: %w", err)
			}
		} else {
			return fmt.Errorf("服务器不支持 STARTTLS")
		}
	}

	if cfg.User != "" {
		auth := smtp.PlainAuth("", cfg.User, cfg.Password, host)
		if ok, mechs := c.Extension("AUTH"); ok {
			if !strings.Contains(mechs, "PLAIN") {
				return fmt.Errorf("服务器不支持 PLAIN 认证（支持: %s）", mechs)
			}
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("SMTP 认证失败: %w", err)
			}
		}
	}

	if err := c.Mail(from); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("设置收件人 %s 失败: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("开始写邮件内容失败: %w", err)
	}
	if _, err := w.Write(buildMessage(from, to, subject, body)); err != nil {
		return fmt.Errorf("写邮件内容失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("提交邮件失败: %w", err)
	}
	return c.Quit()
}

// buildMessage 构造 RFC 5322 邮件内容（UTF-8，标题 Q 编码）。
func buildMessage(from string, to []string, subject, body string) []byte {
	headers := []string{
		"From: " + from,
		"To: " + strings.Join(to, ", "),
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body)
}

// splitEmails 拆分逗号/分号/空白分隔的邮箱列表。
func splitEmails(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func (n *Notifier) meta(ctx context.Context, key string) string {
	val, err := n.db.GetMeta(ctx, metaPrefix+key)
	if err != nil {
		return ""
	}
	return val
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func levelLabel(level string) string {
	switch level {
	case "error":
		return "错误"
	case "warn":
		return "警告"
	default:
		return "信息"
	}
}
