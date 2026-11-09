// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !js

package chromium

import (
	"errors"

	"github.com/chromedp/chromedp"
)

// networkIsolation solo existe en Linux (espacios de nombres de red); en
// los demás sistemas RestrictNetwork se queda con las banderas.
func networkIsolation(string) (chromedp.ExecAllocatorOption, error) {
	return nil, errors.New("network namespaces are only available on Linux")
}
