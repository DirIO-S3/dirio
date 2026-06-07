package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestVersionDefaults(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Version", Version, "dev"},
		{"Commit", Commit, "none"},
		{"BuildTime", BuildTime, "unknown"},
		{"BuiltBy", BuiltBy, "local"},
		{"GitTreeState", GitTreeState, "unknown"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}

	if !strings.HasPrefix(GoVersion, "go1.") {
		t.Errorf("GoVersion = %q, want go1.* prefix", GoVersion)
	}
	if GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want runtime.Version() = %q", GoVersion, runtime.Version())
	}
}
