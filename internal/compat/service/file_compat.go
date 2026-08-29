package service

import corefile "github.com/JekYUlll/Dipole/internal/services/core/domain/file"

type FileService = corefile.FileService
type FileDownloadResult = corefile.FileDownloadResult
type FileContentResult = corefile.FileContentResult
type InitiateMultipartUploadInput = corefile.InitiateMultipartUploadInput
type InitiateMultipartUploadResult = corefile.InitiateMultipartUploadResult

var (
	ErrFileMissing              = corefile.ErrFileMissing
	ErrFileTooLarge             = corefile.ErrFileTooLarge
	ErrFileStorageUnavailable   = corefile.ErrFileStorageUnavailable
	ErrFileNotFound             = corefile.ErrFileNotFound
	ErrFilePermissionDenied     = corefile.ErrFilePermissionDenied
	ErrFileExpired              = corefile.ErrFileExpired
	ErrMultipartSessionNotFound = corefile.ErrMultipartSessionNotFound
	ErrMultipartSessionInvalid  = corefile.ErrMultipartSessionInvalid
	ErrMultipartPartInvalid     = corefile.ErrMultipartPartInvalid
)
