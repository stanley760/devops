package audit

import "testing"

func TestUserTypeLabels(t *testing.T) {
	if got := UserTypeMember.String(); got != "Member" {
		t.Fatalf("UserTypeMember.String() = %q, want %q", got, "Member")
	}
	if got := UserTypeAdmin.String(); got != "Admin" {
		t.Fatalf("UserTypeAdmin.String() = %q, want %q", got, "Admin")
	}
}

func TestUserTypeNumericValues(t *testing.T) {
	if UserTypeMember != 1 {
		t.Fatalf("UserTypeMember = %d, want 1", UserTypeMember)
	}
	if UserTypeAdmin != 2 {
		t.Fatalf("UserTypeAdmin = %d, want 2", UserTypeAdmin)
	}
}

func TestUserTypeValid(t *testing.T) {
	tests := []struct {
		name string
		in   UserType
		want bool
	}{
		{"member", UserTypeMember, true},
		{"admin", UserTypeAdmin, true},
		{"zero is unset", UserType(0), false},
		{"out of range", UserType(99), false},
		{"negative", UserType(-1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Valid(); got != tt.want {
				t.Fatalf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserTypeStringFallsBackForUnknown(t *testing.T) {
	got := UserType(99).String()
	if got != "Unknown(99)" {
		t.Fatalf("String() = %q, want %q", got, "Unknown(99)")
	}
}

func TestUserTypesEnumeratesAll(t *testing.T) {
	got := UserTypes()
	if len(got) != 2 {
		t.Fatalf("UserTypes() len = %d, want 2", len(got))
	}
	for _, u := range got {
		if !u.Valid() {
			t.Fatalf("UserTypes() contains undefined value %d", u)
		}
	}
}
