// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"io/fs"
)

// GetQuizPollJS retorna el JavaScript de quiz/poll cargado desde assets.
func GetQuizPollJS() string {
	content, err := fs.ReadFile(jsModulesFS, "assets/js/modules/quizpoll.js")
	if err != nil {
		return "console.error('QuizPoll JS module not found');"
	}
	return string(content)
}
