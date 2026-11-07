package mdns

import (
	"slices"
	"testing"
)

func TestArgumentsRegisterTheService(t *testing.T) {
	if got, want := arguments(8443), []string{"-R", "Kaelix API", "_kaelix._tcp", "local", "8443"}; !slices.Equal(got, want) {
		t.Fatalf("%v", got)
	}
}
