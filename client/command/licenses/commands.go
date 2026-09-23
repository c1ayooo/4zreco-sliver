package licenses

import (
	"github.com/spf13/cobra"

	"4zreco/var/sliver/client/command/help"
	"4zreco/var/sliver/client/console"
	consts "4zreco/var/sliver/client/constants"
	"4zreco/var/sliver/client/licenses"
)

// Commands returns the `licences` command.
func Commands(con *console.SliverClient) []*cobra.Command {
	licensesCmd := &cobra.Command{
		Use:   consts.LicensesStr,
		Short: "Open source licenses",
		Long:  help.GetHelpFor([]string{consts.LicensesStr}),
		Run: func(cmd *cobra.Command, args []string) {
			con.Println(licenses.All)
		},
		GroupID: consts.GenericHelpGroup,
	}

	return []*cobra.Command{licensesCmd}
}
