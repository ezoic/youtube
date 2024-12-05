package youtube

import (
	_ "embed"
	"testing"
)

//go:embed testdata/yt_vid_page.html
var sampleYTVideoPage []byte

func Test_extractYTConfig(t *testing.T) {
	type args struct {
		html []byte
	}
	tests := []struct {
		name string
		args args
		want YTConfig
	}{
		{
			args: args{
				html: sampleYTVideoPage,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractYTConfig(tt.args.html)
			if err != nil {
				t.Errorf("extractYTConfig() error = %v", err)
				return
			}
			t.Logf("%#v", got)
		})
	}
}
