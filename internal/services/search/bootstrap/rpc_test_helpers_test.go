package bootstrap

import (
	"github.com/JekYUlll/Dipole/internal/model"
)

type rpcCoreStub struct{}

func (rpcCoreStub) GetUserByUUID(userUUID string) (*model.User, error) {
	return &model.User{UUID: userUUID, Nickname: "RPC User"}, nil
}
func (rpcCoreStub) CanSendDirectMessage(string, string) (bool, error) { return true, nil }
func (rpcCoreStub) GetGroupByUUID(groupUUID string) (*model.Group, error) {
	return &model.Group{UUID: groupUUID, Name: "RPC Group"}, nil
}
func (rpcCoreStub) GetGroupMember(groupUUID, userUUID string) (*model.GroupMember, error) {
	return &model.GroupMember{GroupUUID: groupUUID, UserUUID: userUUID}, nil
}
func (rpcCoreStub) ListGroupMembers(groupUUID string) ([]*model.GroupMember, error) {
	return []*model.GroupMember{{GroupUUID: groupUUID, UserUUID: "U1"}}, nil
}
func (rpcCoreStub) GetOwnedFile(uploaderUUID, fileUUID string) (*model.UploadedFile, error) {
	return &model.UploadedFile{UUID: fileUUID, UploaderUUID: uploaderUUID, FileName: "rpc-file"}, nil
}
func (rpcCoreStub) ListSearchConversationKeys(userUUID string) ([]string, error) {
	return []string{"direct:" + userUUID + ":U2"}, nil
}
