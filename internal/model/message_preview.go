package model

func BuildMessagePreview(message *Message) string {
	if message == nil {
		return ""
	}
	switch message.MessageType {
	case MessageTypeText, MessageTypeAIText:
		return truncatePreview(message.Content, 100)
	case MessageTypeFile:
		if message.FileName != "" {
			return "[file] " + message.FileName
		}
		return "[file]"
	case MessageTypeSystem:
		return "[system] " + truncatePreview(message.Content, 90)
	default:
		return "[unsupported]"
	}
}

func truncatePreview(content string, limit int) string {
	runes := []rune(content)
	if len(runes) <= limit {
		return content
	}
	return string(runes[:limit])
}
