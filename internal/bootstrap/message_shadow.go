package bootstrap

import (
	"sync"
	"time"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/logger"
	"github.com/JekYUlll/Dipole/internal/model"
	"go.uber.org/zap"
)

type messageShadowComparison struct {
	Operation    string
	Match        bool
	PrimaryCount int
	ShadowCount  int
	PrimaryError string
	ShadowError  string
}

type messageShadowApplication struct {
	primary application.MessageApplication
	shadow  application.MessageApplication
	observe func(messageShadowComparison)
	work    sync.WaitGroup
}

func newMessageShadowApplication(primary, shadow application.MessageApplication, observe func(messageShadowComparison)) *messageShadowApplication {
	if observe == nil {
		observe = logMessageShadowComparison
	}
	return &messageShadowApplication{primary: primary, shadow: shadow, observe: observe}
}

func (s *messageShadowApplication) SendDirectMessage(senderUUID, targetUUID, content, clientMessageID string) (*model.Message, error) {
	return s.primary.SendDirectMessage(senderUUID, targetUUID, content, clientMessageID)
}

func (s *messageShadowApplication) SendGroupMessage(senderUUID, groupUUID, content, clientMessageID string) (*model.Message, []string, error) {
	return s.primary.SendGroupMessage(senderUUID, groupUUID, content, clientMessageID)
}

func (s *messageShadowApplication) SendDirectFileMessage(senderUUID, targetUUID, fileUUID, clientMessageID string) (*model.Message, error) {
	return s.primary.SendDirectFileMessage(senderUUID, targetUUID, fileUUID, clientMessageID)
}

func (s *messageShadowApplication) SendGroupFileMessage(senderUUID, groupUUID, fileUUID, clientMessageID string) (*model.Message, []string, error) {
	return s.primary.SendGroupFileMessage(senderUUID, groupUUID, fileUUID, clientMessageID)
}

func (s *messageShadowApplication) ListDirectMessages(userUUID, targetUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	primary, err := s.primary.ListDirectMessages(userUUID, targetUUID, beforeID, limit)
	s.compare("list_direct_history", primary, err, func() ([]*model.Message, error) {
		return s.shadow.ListDirectMessages(userUUID, targetUUID, beforeID, limit)
	})
	return primary, err
}

func (s *messageShadowApplication) ListGroupMessages(userUUID, groupUUID string, beforeID uint, limit int) ([]*model.Message, error) {
	primary, err := s.primary.ListGroupMessages(userUUID, groupUUID, beforeID, limit)
	s.compare("list_group_history", primary, err, func() ([]*model.Message, error) {
		return s.shadow.ListGroupMessages(userUUID, groupUUID, beforeID, limit)
	})
	return primary, err
}

func (s *messageShadowApplication) ListGroupMessagesAfter(userUUID, groupUUID string, afterID uint, limit int) ([]*model.Message, error) {
	primary, err := s.primary.ListGroupMessagesAfter(userUUID, groupUUID, afterID, limit)
	s.compare("list_group_history_after", primary, err, func() ([]*model.Message, error) {
		return s.shadow.ListGroupMessagesAfter(userUUID, groupUUID, afterID, limit)
	})
	return primary, err
}

func (s *messageShadowApplication) ListGroupMessagesAfterSeq(userUUID, groupUUID string, afterSeq uint64, limit int) ([]*model.Message, error) {
	primary, err := s.primary.ListGroupMessagesAfterSeq(userUUID, groupUUID, afterSeq, limit)
	s.compare("list_group_history_after_seq", primary, err, func() ([]*model.Message, error) {
		return s.shadow.ListGroupMessagesAfterSeq(userUUID, groupUUID, afterSeq, limit)
	})
	return primary, err
}

func (s *messageShadowApplication) ListOfflineMessages(userUUID string, afterID uint, limit int) ([]*model.Message, error) {
	primary, err := s.primary.ListOfflineMessages(userUUID, afterID, limit)
	s.compare("list_offline_messages", primary, err, func() ([]*model.Message, error) {
		return s.shadow.ListOfflineMessages(userUUID, afterID, limit)
	})
	return primary, err
}

func (s *messageShadowApplication) compare(operation string, primary []*model.Message, primaryErr error, query func() ([]*model.Message, error)) {
	s.work.Add(1)
	go func() {
		defer s.work.Done()
		shadow, shadowErr := query()
		comparison := messageShadowComparison{
			Operation:    operation,
			PrimaryCount: len(primary),
			ShadowCount:  len(shadow),
			PrimaryError: errorText(primaryErr),
			ShadowError:  errorText(shadowErr),
		}
		comparison.Match = comparison.PrimaryError == comparison.ShadowError && equalMessagePages(primary, shadow)
		s.observe(comparison)
	}()
}

func equalMessagePages(primary, shadow []*model.Message) bool {
	if len(primary) != len(shadow) {
		return false
	}
	for index := range primary {
		left, right := primary[index], shadow[index]
		if left == nil || right == nil {
			if left != right {
				return false
			}
			continue
		}
		if left.ID != right.ID || left.UUID != right.UUID || left.ClientMessageID != right.ClientMessageID ||
			left.Seq != right.Seq ||
			left.ConversationKey != right.ConversationKey || left.SenderUUID != right.SenderUUID ||
			left.TargetType != right.TargetType || left.TargetUUID != right.TargetUUID ||
			left.MessageType != right.MessageType || left.Content != right.Content || left.FileID != right.FileID ||
			left.FileName != right.FileName || left.FileSize != right.FileSize || left.FileURL != right.FileURL ||
			left.FileContentType != right.FileContentType || !left.SentAt.Equal(right.SentAt) ||
			!equalOptionalTime(left.FileExpiresAt, right.FileExpiresAt) {
			return false
		}
	}
	return true
}

func equalOptionalTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Equal(*right)
}

func (s *messageShadowApplication) Wait() {
	if s != nil {
		s.work.Wait()
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func logMessageShadowComparison(comparison messageShadowComparison) {
	fields := []zap.Field{
		zap.String("operation", comparison.Operation),
		zap.Bool("match", comparison.Match),
		zap.Int("primary_count", comparison.PrimaryCount),
		zap.Int("shadow_count", comparison.ShadowCount),
		zap.String("primary_error", comparison.PrimaryError),
		zap.String("shadow_error", comparison.ShadowError),
	}
	if comparison.Match {
		logger.L().Debug("message shadow query compared", fields...)
		return
	}
	logger.L().Warn("message shadow query mismatch", fields...)
}
