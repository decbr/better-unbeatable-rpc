//go:build !linux

package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const clipboardHelperFlag = "--copy-to-clipboard"

func copyText(ctx context.Context, text string) error { return runtime.ClipboardSetText(ctx, text) }

// clipboardHelper is only needed under XWayland.
func clipboardHelper() int { return 1 }
