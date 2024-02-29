// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omnictl

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/siderolabs/gen/maps"
	"github.com/spf13/cobra"

	"github.com/siderolabs/omni/client/pkg/omnictl/config"
	"github.com/siderolabs/omni/client/pkg/omnictl/internal/access"
)

// configCmd represents the config command.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the client configuration file (omniconfig)",
	Long:  ``,
}

// configURLCmd represents the `config url` command.
var configURLCmd = &cobra.Command{
	Use:   "url <url>",
	Short: "Set the URL for the current context",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		conf, err := config.Init(access.CmdFlags.Omniconfig, false)
		if err != nil {
			return err
		}

		context, err := conf.GetContext(access.CmdFlags.Context)
		if err != nil {
			return err
		}

		context.URL = args[0]

		return conf.Save()
	},
}

// configIdentityCmd represents the `config identity` command.
var configIdentityCmd = &cobra.Command{
	Use:   "identity <identity>",
	Short: "Set the auth identity for the current context",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		conf, err := config.Init(access.CmdFlags.Omniconfig, false)
		if err != nil {
			return err
		}

		context, err := conf.GetContext(access.CmdFlags.Context)
		if err != nil {
			return err
		}

		context.Auth.SideroV1.Identity = args[0]

		return conf.Save()
	},
}

// configContextCmd represents the `config context` command.
var configContextCmd = &cobra.Command{
	Use:     "context <context>",
	Short:   "Set the current context",
	Aliases: []string{"use-context"},
	Long:    ``,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		conf, err := config.Init(access.CmdFlags.Omniconfig, false)
		if err != nil {
			return err
		}

		context := args[0]

		conf.Context = context

		return conf.Save()
	},
	ValidArgsFunction: CompleteConfigContext,
}

// configAddCmdFlags represents the `config add` command flags.
var configAddCmdFlags struct {
	url      string
	httpURL  string
	identity string
}

// configAddCmd represents the `config add` command.
var configAddCmd = &cobra.Command{
	Use:   "add <context>",
	Short: "Add a new context",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		conf, err := config.Init(access.CmdFlags.Omniconfig, true)
		if err != nil {
			return err
		}

		name := args[0]

		_, alreadyExists := conf.Contexts[name]
		if alreadyExists {
			return fmt.Errorf("context %s already exists", name)
		}

		newContext := config.Context{
			URL: configAddCmdFlags.url,
			Auth: config.Auth{
				SideroV1: config.SideroV1{
					Identity: configAddCmdFlags.identity,
				},
			},
		}

		conf.Contexts[name] = &newContext

		return conf.Save()
	},
}

// configGetContextsCmd represents the `config contexts` command.
var configGetContextsCmd = &cobra.Command{
	Use:     "contexts",
	Short:   "List defined contexts",
	Aliases: []string{"get-contexts"},
	Long:    ``,
	RunE: func(*cobra.Command, []string) error {
		conf, err := config.Init(access.CmdFlags.Omniconfig, false)
		if err != nil {
			return err
		}

		keys := maps.Keys(conf.Contexts)
		slices.Sort(keys)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		defer w.Flush() //nolint:errcheck

		_, err = fmt.Fprintln(w, "CURRENT\tNAME\tURL")
		if err != nil {
			return err
		}

		for _, name := range keys {
			context := conf.Contexts[name]

			var current string

			if name == conf.Context {
				current = "*"
			}

			_, err = fmt.Fprintf(w, "%s\t%s\t%s\n", current, name, context.URL)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

// configMergeCmd represents the `config merge` command.
var configMergeCmd = &cobra.Command{
	Use:   "merge <from>",
	Short: "Merge additional contexts from another client configuration file",
	Long:  "Contexts with the same name are renamed while merging configs.",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		conf, err := config.Init(access.CmdFlags.Omniconfig, true)
		if err != nil {
			return err
		}

		renames, err := conf.Merge(args[0])
		if err != nil {
			return err
		}

		for _, rename := range renames {
			fmt.Printf("renamed omniconfig context %s\n", rename.String())
		}

		return conf.Save()
	},
}

// configNewCmdFlags represents the `config new` command flags.
var configNewCmdFlags struct {
	url      string
	httpURL  string
	identity string
}

// configNewCmd represents the `config new` command.
var configNewCmd = &cobra.Command{
	Use:   "new [<path>]",
	Short: "Generate a new client configuration file",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(_ *cobra.Command, args []string) error {
		path := ""
		if len(args) > 0 {
			path = args[0]
		}

		conf, err := config.Init(path, true)
		if err != nil {
			return err
		}

		context, err := conf.GetContext(access.CmdFlags.Context)
		if err != nil {
			return err
		}

		context.URL = configNewCmdFlags.url
		context.Auth.SideroV1.Identity = configNewCmdFlags.identity

		return conf.Save()
	},
}

// configInfoCmdTemplate represents the `config info` command output template.
var configInfoCmdTemplate = template.Must(template.New("configInfoCmdTemplate").
	Option("missingkey=error").
	Parse(strings.TrimSpace(`
Current context: {{ .Context }}
URL:             {{ .APIURL }}
Identity:        {{ .Identity }}
`)))

// configInfoCmd represents the `config info` command.
var configInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show information about the current context",
	Args:  cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		conf, err := config.Init(access.CmdFlags.Omniconfig, false)
		if err != nil {
			return err
		}

		var result string

		context, err := conf.GetContext(access.CmdFlags.Context)
		if err != nil {
			return err
		}

		var buf bytes.Buffer
		err = configInfoCmdTemplate.Execute(&buf, map[string]string{
			"Context":  conf.Context,
			"APIURL":   context.URL,
			"Identity": context.Auth.SideroV1.Identity,
		})
		if err != nil {
			return err
		}

		result = buf.String() + "\n"

		fmt.Print(result)

		return nil
	},
}

// CompleteConfigContext represents tab completion for `--context` argument and `config context` command.
func CompleteConfigContext(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	conf, err := config.Init(access.CmdFlags.Omniconfig, false)
	if err != nil {
		return nil, 0
	}

	contextNames := maps.Keys(conf.Contexts)
	slices.Sort(contextNames)

	return contextNames, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	configCmd.AddCommand(
		configURLCmd,
		configIdentityCmd,
		configContextCmd,
		configAddCmd,
		configGetContextsCmd,
		configMergeCmd,
		configNewCmd,
		configInfoCmd,
	)

	configAddCmd.Flags().StringVar(&configAddCmdFlags.url, "url", config.DefaultContext.URL, "URL of the server")
	configAddCmd.Flags().StringVar(&configAddCmdFlags.identity, "identity", "", "identity to use for authentication")

	configNewCmd.Flags().StringVar(&configNewCmdFlags.url, "url", config.DefaultContext.URL, "URL of the server")
	configNewCmd.Flags().StringVar(&configNewCmdFlags.identity, "identity", "", "identity to use for authentication")

	RootCmd.AddCommand(configCmd)
}
