// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"go.ziradocs.com/doclang/v2/internal/licenses"
)

// NewLicensesCommand imprime LICENSE, NOTICE y las licencias de terceros que
// el binario lleva embebidas (mermaid, Chart.js, MathJax, los glifos de New
// Computer Modern). Las trae el propio binario, no solo el archivo del
// release.
func NewLicensesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "licenses",
		Short: "Print the license and the third-party notices of the code and fonts embedded in this binary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), licenses.Text())
			return err
		},
	}
}
