package utils

import "testing"

func TestValidateProfileImageURL(t *testing.T) {
	t.Parallel()

	if !ValidateProfileImageURL("/user.png") {
		t.Fatal("expected /user.png to be valid")
	}
	if !ValidateProfileImageURL("data:image/png;base64,abcd") {
		t.Fatal("expected data uri to be valid")
	}
	if ValidateProfileImageURL("https://example.com/avatar.png") {
		t.Fatal("expected remote url to be invalid")
	}
}
