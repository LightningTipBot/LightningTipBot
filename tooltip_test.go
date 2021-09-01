package main

import (
	"testing"
	"time"

	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	tipper1 = &tb.User{Username: "username1", FirstName: "firstname1", ID: 1}
	tipper2 = &tb.User{Username: "username2", FirstName: "firstname2", ID: 2}
	tipper3 = &tb.User{Username: "username3", FirstName: "firstname3", ID: 3}
	tipper4 = &tb.User{Username: "username4", FirstName: "firstname4", ID: 4}
	tipper5 = &tb.User{Username: "username5", FirstName: "firstname5", ID: 5}
	tipper6 = &tb.User{Username: "username6", FirstName: "firstname6", ID: 6}
)

func Test_getTippersString(t *testing.T) {
	type args struct {
		tippers []*tb.User
	}
	var tippers = make([]*tb.User, 0)
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "1", args: args{tippers: append(tippers, tipper1)}, want: "@username1"},
		{name: "2", args: args{tippers: append(tippers, tipper1, tipper2)}, want: "@username1, @username2"},
		{name: "3", args: args{tippers: append(tippers, tipper1, tipper2, tipper3)}, want: "@username1, @username2, @username3"},
		{name: "4", args: args{tippers: append(tippers, tipper1, tipper2, tipper3, tipper4)}, want: "@username1, @username2, @username3, @username4"},
		{name: "5", args: args{tippers: append(tippers, tipper1, tipper2, tipper3, tipper4, tipper5)}, want: "@username1, @username2, @username3, @username4, @username5"},
		{name: "6", args: args{tippers: append(tippers, tipper1, tipper2, tipper3, tipper4, tipper5, tipper6)}, want: "@username1, @username2, @username3, @username4, @username5, ... and others"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTippersString(tt.args.tippers); got != tt.want {
				t.Errorf("getTippersString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_getTooltipMessage(t *testing.T) {
	type fields struct {
		Message   Message
		TipAmount int
		Ntips     int
		LastTip   time.Time
		Tippers   []*tb.User
	}
	type args struct {
		botUserName          string
		notInitializedWallet bool
	}
	var tippers = make([]*tb.User, 0)

	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name:   "1",
			args:   args{botUserName: "@test-bot", notInitializedWallet: true},
			fields: fields{Message: Message{}, TipAmount: 10, Ntips: 1, Tippers: append(tippers, tipper1)},
			want:   "🏅 10 sat (by @username1)\n🗑 Chat with @test-bot to manage your wallet.",
		},
		{
			name:   "2",
			args:   args{botUserName: "@test-bot", notInitializedWallet: true},
			fields: fields{Message: Message{}, TipAmount: 100, Ntips: 6, Tippers: append(tippers, tipper1, tipper2, tipper3, tipper4, tipper5, tipper6)},
			want:   "🏅 100 sat (6 tips by @username1, @username2, @username3, @username4, @username5, ... and others)\n🗑 Chat with @test-bot to manage your wallet.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := TipTooltip{
				Message:   tt.fields.Message,
				TipAmount: tt.fields.TipAmount,
				Ntips:     tt.fields.Ntips,
				LastTip:   tt.fields.LastTip,
				Tippers:   tt.fields.Tippers,
			}
			if got := x.getTooltipMessage(tt.args.botUserName, tt.args.notInitializedWallet); got != tt.want {
				t.Errorf("getTooltipMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
