package service

import "errors"

// Error
var ErrPermission = errors.New("permission denied")
var ErrPermissionWrongEndPoint = errors.New("permission denied. Change it through the resource endpoint or unable to change your own role.")
var ErrIDEmpty = errors.New("cannot operate when ID is empty")
var ErrIDNotMatch = errors.New("cannot operate when ID in HTTP body and URL parameter not match")
var ErrPatch = errors.New("patch syntax error") // json: cannot unmarshal object into Go value of type jsonpatch.Patch
var ErrBatchUpdateOrPatchOneNotFound = errors.New("at least one not found")
