package version

import (
	"runtime/debug"
	"testing"
)

func TestFromBuildInfo(t *testing.T) {
	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{
			name:     "clean build",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "7e796617aa0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d"}, {Key: "vcs.modified", Value: "false"}},
			want:     "7e796617aa0b",
		},
		{
			name:     "build with uncommitted changes",
			settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "7e796617aa0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d"}, {Key: "vcs.modified", Value: "true"}},
			want:     "7e796617aa0b-dirty",
		},
		{
			name:     "no VCS information",
			settings: nil,
			want:     Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fromBuildInfo(&debug.BuildInfo{Settings: tt.settings})
			if got != tt.want {
				t.Errorf("fromBuildInfo() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFromBuildInfoNil(t *testing.T) {
	if got := fromBuildInfo(nil); got != Unknown {
		t.Errorf("fromBuildInfo(nil) = %q, want %q", got, Unknown)
	}
}
