package youtube

import (
	_ "embed"
	"testing"
)

//go:embed testdata/player_url.js
var playerConfig1 []byte

//go:embed testdata/player_base_62ccfae7.js
var playerConfig2 []byte

func Test_playerConfig_getNFunctionName(t *testing.T) {
	tests := []struct {
		name    string
		config  playerConfig
		wantErr bool
	}{
		{
			name:    "decipher correctly please",
			config:  playerConfig1,
			wantErr: false,
		},
		{
			name:    "decipher correctly please",
			config:  playerConfig2,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.config.getNFunctionName()
			if (err != nil) != tt.wantErr {
				t.Errorf("playerConfig.getNFunction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Log(got)
		})
	}
}

func Test_playerConfig_extraFunction(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		config  playerConfig
		args    args
		wantErr bool
	}{
		{
			name:   "",
			config: playerConfig1,
			args: args{
				name: "gna",
			},
			wantErr: false,
		},
		{
			name:   "",
			config: playerConfig2,
			args: args{
				name: "hna",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.config.extraFunction(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("playerConfig.extraFunction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Log(got)
		})
	}
}

func Test_playerConfig_decodeNsig(t *testing.T) {
	type args struct {
		encoded string
	}
	tests := []struct {
		name    string
		config  playerConfig
		args    args
		want    string
		wantErr bool
	}{
		{
			name:   "decipher correctly please",
			config: playerConfig2,
			args: args{
				"heQfGvtB42Dx8jRvOPi",
			},
			want: "jZlTeSh50JgiqQ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.config.decodeNsig(tt.args.encoded)
			if (err != nil) != tt.wantErr {
				t.Errorf("playerConfig.decodeNsig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("playerConfig.decodeNsig() = %v, want %v", got, tt.want)
			}
		})
	}
}
