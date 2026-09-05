package ai

import "testing"

func TestDetectAssistantMention(t *testing.T) {
	t.Parallel()

	const nick = "Dipole AI"
	const uuid = "UAI000000000000000001"

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "leading nickname", content: "@Dipole AI 你好", want: true},
		{name: "case folded", content: "请 @dipole ai 总结一下", want: true},
		{name: "compact nickname", content: "@DipoleAI 在吗", want: true},
		{name: "extra spaces after at", content: "@  Dipole   AI，帮忙", want: true},
		{name: "uuid token", content: "帮我看下 @" + uuid, want: true},
		{name: "no at sign", content: "Dipole AI 你好", want: false},
		{name: "partial nickname", content: "@Dipole 你好", want: false},
		{name: "email local part", content: "mail@dipole.ai", want: false},
		{name: "empty", content: "   ", want: false},
		{name: "unrelated mention", content: "@周友 今晚有空吗", want: false},
		{name: "alias AI", content: "@AI 帮我看下", want: true},
		{name: "alias not a substring", content: "@AIRflow 好棒", want: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := DetectAssistantMention(test.content, nick, uuid, "AI"); got != test.want {
				t.Fatalf("DetectAssistantMention(%q) = %v, want %v", test.content, got, test.want)
			}
		})
	}
}

func TestDetectAssistantMentionRequiresNickname(t *testing.T) {
	t.Parallel()
	if DetectAssistantMention("@Dipole AI", "  ") {
		t.Fatal("empty nickname must not match")
	}
}
