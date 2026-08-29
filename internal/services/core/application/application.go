package coreapplication

import (
	applicationPort "github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/model"
)

type Dependencies struct {
	Users         userFinder
	Contacts      directMessagePolicy
	Groups        groupFinder
	Files         fileFinder
	Conversations conversationFinder
}

type userFinder interface {
	GetByUUID(string) (*model.User, error)
}

type directMessagePolicy interface {
	CanSendDirectMessage(string, string) (bool, error)
}

type groupFinder interface {
	GetByUUID(string) (*model.Group, error)
	GetMember(string, string) (*model.GroupMember, error)
	ListMembers(string) ([]*model.GroupMember, error)
}

type fileFinder interface {
	GetByUUID(string) (*model.UploadedFile, error)
}

type conversationFinder interface {
	ListSearchConversationKeys(string) ([]string, error)
}

type LocalCoreCapability struct {
	users         userFinder
	contacts      directMessagePolicy
	groups        groupFinder
	files         fileFinder
	conversations conversationFinder
}

var _ applicationPort.CoreCapability = (*LocalCoreCapability)(nil)

func New(dependencies Dependencies) *LocalCoreCapability {
	return &LocalCoreCapability{
		users: dependencies.Users, contacts: dependencies.Contacts,
		groups: dependencies.Groups, files: dependencies.Files,
		conversations: dependencies.Conversations,
	}
}

func (c *LocalCoreCapability) GetUserByUUID(userUUID string) (*model.User, error) {
	return c.users.GetByUUID(userUUID)
}

func (c *LocalCoreCapability) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	return c.contacts.CanSendDirectMessage(userUUID, friendUUID)
}

func (c *LocalCoreCapability) GetGroupByUUID(groupUUID string) (*model.Group, error) {
	return c.groups.GetByUUID(groupUUID)
}

func (c *LocalCoreCapability) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return c.groups.GetMember(groupUUID, userUUID)
}

func (c *LocalCoreCapability) ListGroupMembers(groupUUID string) ([]*model.GroupMember, error) {
	return c.groups.ListMembers(groupUUID)
}

func (c *LocalCoreCapability) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	file, err := c.files.GetByUUID(fileUUID)
	if err != nil || file == nil || file.UploaderUUID != uploaderUUID {
		return nil, err
	}
	return file, nil
}

func (c *LocalCoreCapability) ListSearchConversationKeys(userUUID string) ([]string, error) {
	return c.conversations.ListSearchConversationKeys(userUUID)
}
