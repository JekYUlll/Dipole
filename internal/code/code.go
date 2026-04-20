package code

const (
	Success int = 0

	BadRequest   int = 400000
	Unauthorized int = 401000
	Forbidden    int = 403000
	NotFound     int = 404000
	Internal     int = 500000
)

const (
	AuthInvalidTelephone   int = 100100
	AuthUserAlreadyExists  int = 100101
	AuthInvalidCredentials int = 100102
	AuthUserDisabled       int = 100103
	AuthTokenRequired      int = 100104
	AuthTokenInvalid       int = 100105
	AuthLogoutFailed       int = 100106
	AuthLoginRateLimited   int = 100107
)

const (
	UserNotFound         int = 100200
	UserPermissionDenied int = 100201
	UserEmptyProfile     int = 100202
	UserInvalidNickname  int = 100203
	UserInvalidEmail     int = 100204
	UserInvalidAvatar    int = 100205
	UserAdminRequired    int = 100206
	UserInvalidStatus    int = 100207
	UserSelfStatusChange int = 100208
	UserInvalidSignature int = 100209
)

const (
	MessageTargetRequired    int = 100300
	MessageContentRequired   int = 100301
	MessageContentTooLong    int = 100302
	MessageTargetUnavailable int = 100303
	MessageTargetNotFound    int = 100304
	MessageFriendRequired    int = 100305
	MessageGroupForbidden    int = 100306
	MessageRateLimited       int = 100307
)

const (
	ConversationTargetRequired int = 100400
	ConversationTargetNotFound int = 100401
	ConversationRemarkTooLong  int = 100402
)

const (
	ContactTargetRequired      int = 100500
	ContactTargetNotFound      int = 100501
	ContactTargetUnavailable   int = 100502
	ContactCannotAddSelf       int = 100503
	ContactAlreadyFriends      int = 100504
	ContactApplicationExists   int = 100505
	ContactApplicationNotFound int = 100506
	ContactApplicationHandled  int = 100507
	ContactPermissionDenied    int = 100508
	ContactActionInvalid       int = 100509
	ContactRemarkTooLong       int = 100510
	ContactApplicationExpired  int = 100511
)

const (
	GroupNameRequired             int = 100600
	GroupNameTooLong              int = 100601
	GroupNoticeTooLong            int = 100602
	GroupAvatarTooLong            int = 100603
	GroupNotFound                 int = 100604
	GroupPermissionDenied         int = 100605
	GroupMemberRequired           int = 100606
	GroupMemberUnavailable        int = 100607
	GroupMemberAlreadyIn          int = 100608
	GroupOwnerCannotLeave         int = 100609
	GroupEmptyUpdate              int = 100610
	GroupOwnerCannotBeRemoved     int = 100611
	GroupDismissed                int = 100612
	GroupAvatarInvalid            int = 100613
	GroupAvatarTooLarge           int = 100614
	GroupAvatarStorageUnavailable int = 100615
)

const (
	FileMissing                  int = 100700
	FileTooLarge                 int = 100701
	FileStorageUnavailable       int = 100702
	FileUploadRateLimited        int = 100703
	FileNotFound                 int = 100704
	FilePermissionDenied         int = 100705
	FileExpired                  int = 100706
	FileMultipartSessionNotFound int = 100707
	FileMultipartSessionInvalid  int = 100708
	FileMultipartPartInvalid     int = 100709
)

const (
	SessionConnectionRequired int = 100800
	SessionNotFound           int = 100801
)
