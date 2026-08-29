package service

import coregroup "github.com/JekYUlll/Dipole/internal/services/core/domain/group"

type GroupAvatarResponse = coregroup.GroupAvatarResponse
type CreateGroupInput = coregroup.CreateGroupInput
type UpdateGroupInput = coregroup.UpdateGroupInput
type GroupView = coregroup.GroupView
type GroupMemberView = coregroup.GroupMemberView
type GroupService = coregroup.GroupService
type GroupEventPayload = coregroup.GroupEventPayload

var (
	ErrGroupNameRequired             = coregroup.ErrGroupNameRequired
	ErrGroupNameTooLong              = coregroup.ErrGroupNameTooLong
	ErrGroupNoticeTooLong            = coregroup.ErrGroupNoticeTooLong
	ErrGroupAvatarTooLong            = coregroup.ErrGroupAvatarTooLong
	ErrGroupEmptyUpdate              = coregroup.ErrGroupEmptyUpdate
	ErrGroupNotFound                 = coregroup.ErrGroupNotFound
	ErrGroupDismissed                = coregroup.ErrGroupDismissed
	ErrGroupPermissionDenied         = coregroup.ErrGroupPermissionDenied
	ErrGroupMemberRequired           = coregroup.ErrGroupMemberRequired
	ErrGroupMemberUnavailable        = coregroup.ErrGroupMemberUnavailable
	ErrGroupMemberAlreadyIn          = coregroup.ErrGroupMemberAlreadyIn
	ErrGroupOwnerCannotLeave         = coregroup.ErrGroupOwnerCannotLeave
	ErrGroupOwnerCannotBeRemoved     = coregroup.ErrGroupOwnerCannotBeRemoved
	ErrGroupAvatarMissing            = coregroup.ErrGroupAvatarMissing
	ErrGroupAvatarInvalid            = coregroup.ErrGroupAvatarInvalid
	ErrGroupAvatarTooLarge           = coregroup.ErrGroupAvatarTooLarge
	ErrGroupAvatarStorageUnavailable = coregroup.ErrGroupAvatarStorageUnavailable
)
