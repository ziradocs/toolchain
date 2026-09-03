// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package themes

import "errors"

// mkfifoForTest has no windows equivalent — see mkfifo_unix_test.go.
func mkfifoForTest(path string) error {
	return errors.New("FIFOs are not supported on windows")
}
