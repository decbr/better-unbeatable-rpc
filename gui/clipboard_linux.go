package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>

static int changes = 0;

static gboolean quit_cb(gpointer data) { gtk_main_quit(); return FALSE; }

// owner-change fires once for our own copy; the next one means something
// else owns the clipboard now, so the helper's job is done.
static void owner_changed(GtkClipboard *c, GdkEvent *e, gpointer data) {
	if (++changes > 1) gtk_main_quit();
}

static int copy_and_wait(const char *text, int seconds) {
	if (!gtk_init_check(NULL, NULL)) return 1;
	GtkClipboard *c = gtk_clipboard_get(GDK_SELECTION_CLIPBOARD);
	g_signal_connect(c, "owner-change", G_CALLBACK(owner_changed), NULL);
	gtk_clipboard_set_text(c, text, -1);
	gtk_clipboard_store(c);
	g_timeout_add_seconds(seconds, quit_cb, NULL);
	gtk_main();
	return 0;
}
*/
import "C"

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const clipboardHelperFlag = "--copy-to-clipboard"

// copyText only exists because wayland support on wails is shite
func copyText(ctx context.Context, text string) error {
	if os.Getenv("WAYLAND_DISPLAY") != "" && os.Getenv("DISPLAY") != "" {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		cmd := exec.Command(self, clipboardHelperFlag)
		cmd.Env = append(os.Environ(), "GDK_BACKEND=x11")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Start(); err != nil {
			return err
		}
		go cmd.Wait()
	} else if err := runtime.ClipboardSetText(ctx, text); err != nil {
		return err
	}

	want := strings.TrimSpace(text)
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(150 * time.Millisecond) {
		if got, err := runtime.ClipboardGetText(ctx); err == nil && strings.TrimSpace(got) == want {
			return nil
		}
	}
	return errors.New("the text didn't reach the clipboard")
}

// clipboardHelper is the XWayland side of copyText: it copies stdin and waits,
// for at most 10 minutes, until something else is copied.
func clipboardHelper() int {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 1
	}
	text := C.CString(string(data))
	defer C.free(unsafe.Pointer(text))
	return int(C.copy_and_wait(text, 600))
}
