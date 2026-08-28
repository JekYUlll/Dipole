package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type LocalSearchApplication struct {
	core  application.CoreCapability
	index application.SearchIndex
}

var _ application.SearchApplication = (*LocalSearchApplication)(nil)

func NewSearchApplication(core application.CoreCapability, index application.SearchIndex) (*LocalSearchApplication, error) {
	if core == nil {
		return nil, errors.New("Search Core capability is required")
	}
	if index == nil {
		return nil, errors.New("Search index is required")
	}
	return &LocalSearchApplication{core: core, index: index}, nil
}

func (s *LocalSearchApplication) Search(principal, text string, limit int) ([]*model.MessageSearchDocument, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, application.ErrSearchTextRequired
	}
	conversationKeys, err := s.core.ListSearchConversationKeys(principal)
	if err != nil {
		return nil, fmt.Errorf("resolve Search authorization scope: %w", err)
	}
	if len(conversationKeys) == 0 {
		return []*model.MessageSearchDocument{}, nil
	}
	return s.index.Search(model.MessageSearchQuery{ConversationKeys: conversationKeys, Text: text, Limit: limit})
}
