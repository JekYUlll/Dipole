package bootstrap

func kafkaManagedTopics() []string {
	return []string{
		"message.direct.send_requested",
		"message.direct.created",
		"message.group.send_requested",
		"message.group.created",
		"conversation.direct.read",
		"group.created",
		"group.updated",
		"group.members.added",
		"group.members.removed",
		"group.dismissed",
		"contact.friend.deleted",
		"session.force_logout",
	}
}
