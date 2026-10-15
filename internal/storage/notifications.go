package storage

import (
	"context"
	"fmt"
	"time"
)

// Notification 是站点内通知（持久化，跨会话可见）。
type Notification struct {
	ID        string `json:"id"`
	Level     string `json:"level"`  // 通知类型：info / warn / error
	Module    string `json:"module"` // 来源模块：apps / software / servers / system ...
	Title     string `json:"title"`
	Message   string `json:"message"` // 详情
	Read      bool   `json:"read"`
	CreatedAt int64  `json:"created_at"`
}

// NotificationStore 管理站内通知。
type NotificationStore struct {
	db *DB
}

// NewNotificationStore 创建通知存储实例。
func NewNotificationStore(db *DB) *NotificationStore {
	return &NotificationStore{db: db}
}

// GenerateNotificationID 生成通知 ID。
func GenerateNotificationID() string {
	return fmt.Sprintf("ntf_%d_%s", time.Now().UnixNano(), randomStr(6))
}

// Add 写入一条通知。
func (s *NotificationStore) Add(ctx context.Context, level, module, title, message string) error {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	_, err := s.db.Raw().ExecContext(ctx, `
		INSERT INTO notifications (id, level, module, title, message, read, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?)`,
		GenerateNotificationID(), level, module, title, message, time.Now().Unix())
	return err
}

// List 返回最新通知（limit <= 0 时默认 50）。
func (s *NotificationStore) List(ctx context.Context, limit int) []*Notification {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Raw().QueryContext(ctx, `
		SELECT id, level, module, title, message, read, created_at
		FROM notifications ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []*Notification
	for rows.Next() {
		n := &Notification{}
		var read int
		if err := rows.Scan(&n.ID, &n.Level, &n.Module, &n.Title, &n.Message, &read, &n.CreatedAt); err != nil {
			continue
		}
		n.Read = read == 1
		items = append(items, n)
	}
	return items
}

// UnreadCount 返回未读通知数。
func (s *NotificationStore) UnreadCount(ctx context.Context) int {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	var count int
	if err := s.db.Raw().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE read = 0`).Scan(&count); err != nil {
		return 0
	}
	return count
}

// MarkRead 将单条通知标记为已读。
func (s *NotificationStore) MarkRead(ctx context.Context, id string) error {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	_, err := s.db.Raw().ExecContext(ctx, `UPDATE notifications SET read = 1 WHERE id = ? AND read = 0`, id)
	return err
}

// MarkAllRead 将全部通知标记为已读。
func (s *NotificationStore) MarkAllRead(ctx context.Context) error {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	_, err := s.db.Raw().ExecContext(ctx, `UPDATE notifications SET read = 1 WHERE read = 0`)
	return err
}

// Clear 清空全部通知。
func (s *NotificationStore) Clear(ctx context.Context) error {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	_, err := s.db.Raw().ExecContext(ctx, `DELETE FROM notifications`)
	return err
}
