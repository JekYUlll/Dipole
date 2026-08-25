package app

import "github.com/JekYUlll/Dipole/internal/model"

type LocalCoreCapability struct {
	repos *Repositories
}

func NewLocalCoreCapability(repos *Repositories) *LocalCoreCapability {
	return &LocalCoreCapability{repos: repos}
}

func (c *LocalCoreCapability) GetUserByUUID(userUUID string) (*model.User, error) {
	return c.repos.Users.GetByUUID(userUUID)
}

func (c *LocalCoreCapability) CanSendDirectMessage(userUUID, friendUUID string) (bool, error) {
	return c.repos.Contacts.CanSendDirectMessage(userUUID, friendUUID)
}

func (c *LocalCoreCapability) GetGroupByUUID(groupUUID string) (*model.Group, error) {
	return c.repos.Groups.GetByUUID(groupUUID)
}

func (c *LocalCoreCapability) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return c.repos.Groups.GetMember(groupUUID, userUUID)
}

func (c *LocalCoreCapability) ListGroupMembers(groupUUID string) ([]*model.GroupMember, error) {
	return c.repos.Groups.ListMembers(groupUUID)
}
