package telegram

import (
	tb "gopkg.in/lightningtipbot/telebot.v3"
	"testing"
)

func TestGetUserStr(t *testing.T) {
	type args struct {
		user *tb.User
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "NoUserName", args: args{user: &tb.User{FirstName: "BotUser", ID: 12351241}}, want: "BotUser"},
		{name: "UserName", args: args{user: &tb.User{Username: "Username", FirstName: "BotUser", ID: 12351241}}, want: "@Username"},
		{name: "NoUserName", args: args{user: &tb.User{Username: "", FirstName: "", ID: 12351241}}, want: "12351241"},
		{name: "NoUserName", args: args{user: &tb.User{Username: "Username", FirstName: "", ID: 12351241}}, want: "@Username"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetUserStr(tt.args.user); got != tt.want {
				t.Errorf("GetUserStr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkGetUserName(b *testing.B) {
	benchmarkGetUserStr(&tb.User{Username: "dawg", ID: 1235123, FirstName: "adolf"}, b)
}
func BenchmarkGetUserStrNoUsername(b *testing.B) {
	benchmarkGetUserStr(&tb.User{Username: "", ID: 1235123, FirstName: "adolf"}, b)
}

func BenchmarkGetUserStrNoFirstName(b *testing.B) {
	benchmarkGetUserStr(&tb.User{Username: "", ID: 1235123, FirstName: ""}, b)
}

func benchmarkGetUserStr(s *tb.User, b *testing.B) {
	for n := 0; n < b.N; n++ {
		GetUserStr(s)
	}
}

func TestGetUserStrMd(t *testing.T) {
	type args struct {
		user *tb.User
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "NoUserName", args: args{user: &tb.User{FirstName: "BotUser", ID: 12351241}}, want: "[BotUser](tg://user?id=12351241)"},
		{name: "UserName", args: args{user: &tb.User{Username: "Username", FirstName: "BotUser", ID: 12351241}}, want: "@Username"},
		{name: "NoUserName", args: args{user: &tb.User{Username: "", FirstName: "", ID: 12351241}}, want: "[12351241](tg://user?id=12351241)"},
		{name: "NoUserName", args: args{user: &tb.User{Username: "Username", FirstName: "", ID: 12351241}}, want: "@Username"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetUserStrMd(tt.args.user); got != tt.want {
				t.Errorf("GetUserStr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkGetUserStrMdName(b *testing.B) {
	benchmarkGetUserStrMd(&tb.User{Username: "dawg", ID: 1235123, FirstName: "adolf"}, b)
}
func BenchmarkGetUserStrMdNoUsername(b *testing.B) {
	benchmarkGetUserStrMd(&tb.User{Username: "", ID: 1235123, FirstName: "adolf"}, b)
}

func BenchmarkGetUserStrMdNoFirstName(b *testing.B) {
	benchmarkGetUserStrMd(&tb.User{Username: "", ID: 1235123, FirstName: ""}, b)
}

func benchmarkGetUserStrMd(s *tb.User, b *testing.B) {
	for n := 0; n < b.N; n++ {
		GetUserStrMd(s)
	}
}
