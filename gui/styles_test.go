package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/decbr/better-unbeatable-rpc/daemon/presence"
)

func TestStyleRoundTrip(t *testing.T) {
	for _, b := range presence.Styles {
		got, err := importStyle(exportStyle(b.Template))
		if err != nil || !reflect.DeepEqual(got, b.Template) {
			t.Errorf("%s: got %+v, %v", b.Name, got, err)
		}
	}
}

func TestImportStyleFromChat(t *testing.T) {
	c := presence.CustomStyle{Name: "mine", Song: presence.Template{Details: "♪ {title}[ - {artist}]"}}
	pasted := "here's mine:\n```json\n" + exportStyle(c) + "\n```\nhave fun"
	got, err := importStyle(pasted)
	if err != nil || !reflect.DeepEqual(got, c) {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestImportStyleRejects(t *testing.T) {
	cases := map[string]string{
		"not json":     "hello",
		"broken json":  `{"unbeatableRpcStyle": 1, "name": }`,
		"no marker":    `{"name": "x", "song": {"details": "{title}"}}`,
		"other json":   `{"a": 1}`,
		"newer":        `{"unbeatableRpcStyle": 2, "name": "x"}`,
		"broken style": `{"unbeatableRpcStyle": 1, "name": "x", "song": {"details": "{titel}"}}`,
	}
	for name, text := range cases {
		if _, err := importStyle(text); err == nil {
			t.Errorf("%s: imported", name)
		}
	}
	if _, err := importStyle(`{"unbeatableRpcStyle": 2}`); err == nil || !strings.Contains(err.Error(), "newer version") {
		t.Errorf("newer style error = %v", err)
	}
}

func TestStyleFileName(t *testing.T) {
	for in, want := range map[string]string{"mine": "mine.json", " a/b\\c ": "a-b-c.json", "": "style.json", "..": "style.json", "a:b?": "a-b-.json"} {
		if got := styleFileName(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
