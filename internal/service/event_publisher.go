package service

import "context"

type eventPublisher interface {
	PublishJSON(ctx context.Context, topic string, key string, payload any, headers map[string]string) error
	PublishEvent(ctx context.Context, topic string, key string, eventType string, payload any, headers map[string]string) error
}
