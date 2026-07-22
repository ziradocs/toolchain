// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"go.ziradocs.com/slidelang/v2/internal/generator/css/themes"
)

// expandPath expands ~ to user home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if usr, err := user.Current(); err == nil {
			return filepath.Join(usr.HomeDir, path[2:])
		}
	}
	return path
}

// NewThemesCommand creates the themes command with all subcommands
func NewThemesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "themes",
		Short: "Manage SlideLang themes",
		Long: `Manage SlideLang themes - list, install, validate, and configure themes.
		
Themes allow you to customize the appearance of your presentations.
SlideLang includes built-in themes and supports external themes.`,
		Example: `  # List all available themes
  slidelang themes list
  
  # List only external themes
  slidelang themes list --external
  
  # Install a theme from file
  slidelang themes install ./corporate-theme.json
  
  # Validate a theme
  slidelang themes validate ./my-theme.json
  
  # Show theme information
  slidelang themes info corporate`,
	}

	// Add subcommands
	cmd.AddCommand(newThemesListCommand())
	cmd.AddCommand(newThemesInstallCommand())
	cmd.AddCommand(newThemesValidateCommand())
	cmd.AddCommand(newThemesInfoCommand())
	cmd.AddCommand(newThemesRemoveCommand())
	cmd.AddCommand(newThemesPathsCommand())
	cmd.AddCommand(CreateThemeCmd())
	cmd.AddCommand(PreviewThemeCmd())

	return cmd
}

// newThemesListCommand creates the 'themes list' subcommand
func newThemesListCommand() *cobra.Command {
	var externalOnly bool
	var embeddedOnly bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available themes",
		Long:  "List all available themes (both embedded and external)",
		Example: `  # List all themes
  slidelang themes list
  
  # List only external themes
  slidelang themes list --external
  
  # List only embedded themes  
  slidelang themes list --embedded
  
  # Output as JSON
  slidelang themes list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			loader := themes.NewThemeLoader()
			availableThemes, err := loader.GetAvailableThemes()
			if err != nil {
				return fmt.Errorf("failed to get available themes: %w", err)
			}

			// Filter themes based on flags
			filteredThemes := make(map[string]themes.Theme)
			for name, theme := range availableThemes {
				if externalOnly && !theme.IsExternal {
					continue
				}
				if embeddedOnly && theme.IsExternal {
					continue
				}
				filteredThemes[name] = theme
			}

			if jsonOutput {
				return outputThemesJSON(filteredThemes)
			}

			return outputThemesTable(filteredThemes)
		},
	}

	cmd.Flags().BoolVar(&externalOnly, "external", false, "List only external themes")
	cmd.Flags().BoolVar(&embeddedOnly, "embedded", false, "List only embedded themes")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

// newThemesInstallCommand creates the 'themes install' subcommand
func newThemesInstallCommand() *cobra.Command {
	var installDir string
	var force bool

	cmd := &cobra.Command{
		Use:   "install <theme-file>",
		Short: "Install an external theme",
		Long:  "Install an external theme from a JSON file",
		Example: `  # Install from local file
  slidelang themes install ./corporate-theme.json
  
  # Install to specific directory
  slidelang themes install ./theme.json --dir ~/.slidelang/themes
  
  # Force overwrite existing theme
  slidelang themes install ./theme.json --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			themePath := args[0]

			// Check if file exists
			if _, err := os.Stat(themePath); os.IsNotExist(err) {
				return fmt.Errorf("theme file not found: %s", themePath)
			}

			loader := themes.NewThemeLoader()

			// Check if theme already exists
			if !force {
				// Load theme to get its name
				theme, err := themes.LoadExternalTheme(themePath)
				if err != nil {
					return fmt.Errorf("failed to load theme: %w", err)
				}

				// Check if theme already exists
				if _, err := loader.LoadTheme(theme.Manifest.Name, true); err == nil {
					return fmt.Errorf("theme '%s' already exists, use --force to overwrite", theme.Manifest.Name)
				}
			}

			// Install theme
			if err := loader.InstallTheme(themePath, installDir); err != nil {
				return fmt.Errorf("failed to install theme: %w", err)
			}

			fmt.Printf("Theme installed successfully\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&installDir, "dir", "", "Installation directory (default: ~/.slidelang/themes)")
	cmd.Flags().BoolVar(&force, "force", false, "Force overwrite existing theme")

	return cmd
}

// newThemesValidateCommand creates the 'themes validate' subcommand
func newThemesValidateCommand() *cobra.Command {
	var strict bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "validate <theme-file>",
		Short: "Validate a theme file",
		Long:  "Validate a theme file for correctness and compliance",
		Example: `  # Validate theme file
  slidelang themes validate ./my-theme.json
  
  # Strict validation
  slidelang themes validate ./theme.json --strict
  
  # Output validation result as JSON
  slidelang themes validate ./theme.json --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			themePath := args[0]

			// Load theme
			theme, err := themes.LoadExternalTheme(themePath)
			if err != nil {
				return fmt.Errorf("failed to load theme: %w", err)
			}

			// Create validator
			var validator *themes.ThemeValidator
			if strict {
				validator = themes.NewStrictThemeValidator()
			} else {
				validator = themes.NewThemeValidator()
			}

			// Validate theme
			result := validator.ValidateThemeDetailed(theme)

			if jsonOutput {
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal validation result: %w", err)
				}
				fmt.Println(string(data))
				return nil
			}

			// Output validation result
			if result.IsValid {
				fmt.Printf("✅ Theme '%s' is valid\n", theme.Manifest.Name)
			} else {
				fmt.Printf("❌ Theme '%s' validation failed\n", theme.Manifest.Name)
			}

			if len(result.Errors) > 0 {
				fmt.Println("\nErrors:")
				for _, err := range result.Errors {
					fmt.Printf("  • %s\n", err)
				}
			}

			if len(result.Warnings) > 0 {
				fmt.Println("\nWarnings:")
				for _, warning := range result.Warnings {
					fmt.Printf("  • %s\n", warning)
				}
			}

			if !result.IsValid {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Enable strict validation mode")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output validation result as JSON")

	return cmd
}

// newThemesInfoCommand creates the 'themes info' subcommand
func newThemesInfoCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "info <theme-name>",
		Short: "Show detailed information about a theme",
		Long:  "Show detailed information about a specific theme",
		Example: `  # Show theme information
  slidelang themes info corporate
  
  # Output as JSON
  slidelang themes info corporate --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			themeName := args[0]

			loader := themes.NewThemeLoader()
			theme, err := loader.LoadTheme(themeName, true)
			if err != nil {
				return fmt.Errorf("theme '%s' not found: %w", themeName, err)
			}

			if jsonOutput {
				data, err := json.MarshalIndent(theme, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal theme: %w", err)
				}
				fmt.Println(string(data))
				return nil
			}

			// Output theme information
			fmt.Printf("Theme: %s\n", theme.Name)
			fmt.Printf("Description: %s\n", theme.Description)
			fmt.Printf("Author: %s\n", theme.Author)
			fmt.Printf("Version: %s\n", theme.Version)
			fmt.Printf("Type: %s\n", func() string {
				if theme.IsExternal {
					return "External"
				}
				return "Embedded"
			}())

			fmt.Printf("\nVariables (%d):\n", len(theme.Variables))
			for name, value := range theme.Variables {
				fmt.Printf("  %s: %s\n", name, value)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output theme information as JSON")

	return cmd
}

// newThemesRemoveCommand creates the 'themes remove' subcommand
func newThemesRemoveCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <theme-name>",
		Short: "Remove an external theme",
		Long:  "Remove an external theme (embedded themes cannot be removed)",
		Example: `  # Remove theme
  slidelang themes remove corporate
  
  # Force removal without confirmation
  slidelang themes remove corporate --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			themeName := args[0]

			loader := themes.NewThemeLoader()

			// Check if theme exists and is external
			theme, err := loader.LoadTheme(themeName, true)
			if err != nil {
				return fmt.Errorf("theme '%s' not found: %w", themeName, err)
			}

			if !theme.IsExternal {
				return fmt.Errorf("cannot remove embedded theme '%s'", themeName)
			}

			// Confirm removal unless --force is used
			if !force {
				fmt.Printf("Remove theme '%s'? (y/N): ", themeName)
				var response string
				_, _ = fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Theme removal cancelled")
					return nil
				}
			} // Remove theme
			if err := loader.RemoveTheme(themeName); err != nil {
				return fmt.Errorf("failed to remove theme: %w", err)
			}

			fmt.Printf("Theme '%s' removed successfully\n", themeName)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force removal without confirmation")

	return cmd
}

// newThemesPathsCommand creates the 'themes paths' subcommand
func newThemesPathsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "paths",
		Short: "Manage theme search paths",
		Long:  "Show or manage directories where external themes are searched",
		Example: `  # Show current search paths
  slidelang themes paths
  
  # Add a search path
  slidelang themes paths add ./custom-themes
  
  # Remove a search path
  slidelang themes paths remove ./old-themes`,
	}

	// Add subcommands for paths management
	cmd.AddCommand(newThemesPathsListCommand())
	cmd.AddCommand(newThemesPathsAddCommand())
	cmd.AddCommand(newThemesPathsRemoveCommand())

	// Default action: list paths
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newThemesPathsListCommand().RunE(cmd, args)
	}

	return cmd
}

// newThemesPathsListCommand creates the 'themes paths list' subcommand
func newThemesPathsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List current theme search paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			loader := themes.NewThemeLoader()
			paths := loader.GetPaths()

			if len(paths) == 0 {
				fmt.Println("No theme search paths configured")
				return nil
			}

			fmt.Println("Theme search paths:")
			for i, path := range paths {
				// Expand ~ in path for status check
				expandedPath := expandPath(path)

				// Check if path exists
				status := "✅"
				if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
					status = "❌"
				}
				fmt.Printf("  %d. %s %s\n", i+1, status, path)
			}

			return nil
		},
	}
}

// newThemesPathsAddCommand creates the 'themes paths add' subcommand
func newThemesPathsAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <path>",
		Short: "Add a theme search path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			// Resolve absolute path
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}

			loader := themes.NewThemeLoader()
			loader.AddPath(absPath)

			fmt.Printf("Added theme search path: %s\n", absPath)
			return nil
		},
	}
}

// newThemesPathsRemoveCommand creates the 'themes paths remove' subcommand
func newThemesPathsRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove a theme search path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			// Resolve absolute path
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}

			loader := themes.NewThemeLoader()
			loader.RemovePath(absPath)

			fmt.Printf("Removed theme search path: %s\n", absPath)
			return nil
		},
	}
}

// Helper functions

// outputThemesTable outputs themes in table format
func outputThemesTable(themesMap map[string]themes.Theme) error {
	if len(themesMap) == 0 {
		fmt.Println("No themes found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tTYPE\tVERSION\tAUTHOR\tDESCRIPTION")
	_, _ = fmt.Fprintln(w, "----\t----\t-------\t------\t-----------")

	for name, theme := range themesMap {
		themeType := "Embedded"
		if theme.IsExternal {
			themeType = "External"
		}

		description := theme.Description
		if len(description) > 50 {
			description = description[:50] + "..."
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			name, themeType, theme.Version, theme.Author, description)
	}

	return w.Flush()
}

// outputThemesJSON outputs themes in JSON format
func outputThemesJSON(themesMap map[string]themes.Theme) error {
	data, err := json.MarshalIndent(themesMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal themes: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
