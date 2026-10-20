// Package audit defines cross-business operation audit contracts.
//
// This package keeps zero external dependencies (standard library only) and only
// describes "what to record" and "where to write", leaving persistence open:
// local database, remote RPC or message queue are all up to the Sink implementation.
//
// Auditing is a side-channel capability: no write failure may affect the main
// business flow. Column-width truncation and degradation semantics follow the
// constants and Sink comments of this package.
package audit

import "strconv"

// UserType is the actor category: distinguishes C-side members from B-side admins.
// Independent of any specific business module; shared by every audit scenario
// that needs to tell where an actor comes from.
type UserType int64

const (
	// UserTypeMember is an ordinary C-side user (member).
	UserTypeMember UserType = 1
	// UserTypeAdmin is a B-side back-office user (admin).
	UserTypeAdmin UserType = 2
)

// userTypeLabels maps enum values to their display labels, aligned with business enum copy.
var userTypeLabels = map[UserType]string{
	UserTypeMember: "Member",
	UserTypeAdmin:  "Admin",
}

// Valid reports whether the value is a defined enum constant.
func (u UserType) Valid() bool {
	_, ok := userTypeLabels[u]
	return ok
}

// String returns the enum label; undefined values fall back to Unknown(<value>).
func (u UserType) String() string {
	if label, ok := userTypeLabels[u]; ok {
		return label
	}
	return "Unknown(" + strconv.FormatInt(int64(u), 10) + ")"
}

// UserTypes returns all enum values in ascending numeric order, for enum docs and validation traversal.
func UserTypes() []UserType {
	return []UserType{UserTypeMember, UserTypeAdmin}
}
