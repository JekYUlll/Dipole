package bootstrap

import (
	"testing"

	"github.com/JekYUlll/Dipole/internal/config"
)

func TestValidateTimelineNotifyMode(t *testing.T) {
	for _, test := range []struct {
		mode      string
		wantError bool
	}{
		{mode: "off"},
		{mode: "shadow"},
		{mode: "primary"},
		{mode: "", wantError: true},
	} {
		err := validateTimelineNotifyMode(config.Message{TimelineNotifyMode: test.mode})
		if (err != nil) != test.wantError {
			t.Fatalf("mode=%q err=%v want_error=%t", test.mode, err, test.wantError)
		}
	}
}
