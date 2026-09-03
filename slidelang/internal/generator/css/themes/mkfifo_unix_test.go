// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package themes

import "syscall"

// mkfifoForTest creates a FIFO at path — used by
// TestValidateAssets_FontFileNotRegular to reproduce a non-regular file at
// a font's 'local' path. Split into a build-tagged file rather than a
// runtime OS check so the package still compiles on windows (syscall.Mkfifo
// doesn't exist there), matching the windows target this repo's goreleaser
// config actually cross-compiles for.
func mkfifoForTest(path string) error {
	return syscall.Mkfifo(path, 0600)
}
