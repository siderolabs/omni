// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func frontmatter(title, description string) string {
	frontmatter := "---\n"

	frontmatter += "title: " + title + "\n"
	frontmatter += "description: " + description + "\n"

	frontmatter += "---\n\n"

	return frontmatter + "<!-- markdownlint-disable -->\n\n"
}

func linkHandler(name string) string {
	base := strings.TrimSuffix(name, path.Ext(name))

	base = strings.ReplaceAll(base, "_", "-")

	return "#" + strings.ToLower(base)
}

// docsCmd represents the docs command.
var docsCmd = &cobra.Command{
	Use:    "docs <output> [flags]",
	Short:  "Generate documentation for the CLI",
	Long:   ``,
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(_ *cobra.Command, args []string) error {
		dir := args[0]

		filename := filepath.Join(dir, "cli.md")
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		//nolint:errcheck
		defer f.Close()

		if _, err = f.WriteString(frontmatter("omnictl CLI", "omnictl CLI tool reference.")); err != nil {
			return err
		}

		if err = GenMarkdownReference(RootCmd, f, linkHandler); err != nil {
			return fmt.Errorf("failed to generate docs: %w", err)
		}

		return nil
	},
}

// GenMarkdownReference is the same as GenMarkdownTree, but
// with custom filePrepender and linkHandler.
func GenMarkdownReference(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}

		if err := GenMarkdownReference(c, w, linkHandler); err != nil {
			return err
		}
	}

	return doc.GenMarkdownCustom(cmd, w, linkHandler)
}

func init() {
	RootCmd.AddCommand(docsCmd)
}
