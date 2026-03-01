package ir

import "testing"

func TestGoNameIDSuffix(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "id", want: "ID"},
		{in: "item_id", want: "ItemID"},
		{in: "command_id", want: "CommandID"},
		{in: "clientFlipId", want: "ClientFlipID"},
		{in: "id_value", want: "IdValue"},
	}

	for _, tc := range tests {
		got := GoName(tc.in)
		if got != tc.want {
			t.Fatalf("GoName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
