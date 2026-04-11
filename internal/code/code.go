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
)
