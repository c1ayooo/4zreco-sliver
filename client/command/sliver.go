package command

/*
	Sliver Implant Framework
	Copyright (C) 2023  Bishop Fox

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"fmt"

	"4zreco/sliver/client/assets"
	"4zreco/sliver/client/command/ai"
	"4zreco/sliver/client/command/aka"
	"4zreco/sliver/client/command/alias"
	"4zreco/sliver/client/command/backdoor"
	"4zreco/sliver/client/command/cursed"
	"4zreco/sliver/client/command/dllhijack"
	docscmd "4zreco/sliver/client/command/docs"
	"4zreco/sliver/client/command/edit"
	"4zreco/sliver/client/command/environment"
	"4zreco/sliver/client/command/exec"
	"4zreco/sliver/client/command/extensions"
	"4zreco/sliver/client/command/filesystem"
	"4zreco/sliver/client/command/hexedit"
	"4zreco/sliver/client/command/info"
	"4zreco/sliver/client/command/kill"
	"4zreco/sliver/client/command/network"
	"4zreco/sliver/client/command/opfor"
	"4zreco/sliver/client/command/pivots"
	"4zreco/sliver/client/command/portfwd"
	"4zreco/sliver/client/command/privilege"
	"4zreco/sliver/client/command/processes"
	"4zreco/sliver/client/command/reconfig"
	"4zreco/sliver/client/command/registry"
	"4zreco/sliver/client/command/rportfwd"
	"4zreco/sliver/client/command/screenshot"
	"4zreco/sliver/client/command/sessions"
	"4zreco/sliver/client/command/shell"
	"4zreco/sliver/client/command/socks"
	"4zreco/sliver/client/command/tasks"
	"4zreco/sliver/client/command/wasm"
	"4zreco/sliver/client/command/wireguard"
	client "4zreco/sliver/client/console"
	consts "4zreco/sliver/client/constants"
	"github.com/reeflective/console"
	"github.com/spf13/cobra"
)

// SliverCommands returns all commands bound to the implant menu.
func SliverCommands(con *client.SliverClient) console.Commands {
	sliverCommands := func() *cobra.Command {
		sliver := &cobra.Command{
			Short: "Implant commands",
			CompletionOptions: cobra.CompletionOptions{
				HiddenDefaultCmd: true,
			},
		}
		if !con.IsCLI {
			sliver.SilenceErrors = true
			sliver.SilenceUsage = true
		}

		// Utility function to be used for binding new commands to
		// the sliver menu: call the function with the name of the
		// group under which this/these commands should be added,
		// and the group will be automatically created if needed.
		bind := makeBind(sliver, con)

		// [ Core ]
		bind(consts.SliverCoreHelpGroup,
			ai.Commands,
			opfor.Commands,
			docscmd.Commands,
			reconfig.Commands,
			// sessions.Commands,
			sessions.SliverCommands,
			kill.Commands,
			// use.Commands,
			tasks.Commands,
			pivots.Commands,
			aka.ImplantCommands,
		)

		// [ Info ]
		bind(consts.InfoHelpGroup,
			// info.Commands,
			info.SliverCommands,
			screenshot.Commands,
			environment.Commands,
			registry.Commands,
			extensions.SliverCommands,
		)

		// [ Filesystem ]
		bind(consts.FilesystemHelpGroup,
			edit.Commands,
			hexedit.Commands,
			filesystem.Commands,
		)

		// [ Network tools ]
		bind(consts.NetworkHelpGroup,
			network.Commands,
			rportfwd.Commands,
			portfwd.Commands,
			socks.Commands,
			wireguard.SliverCommands,
		)

		// [ Execution ]
		bind(consts.ExecutionHelpGroup,
			shell.Commands,
			exec.Commands,
			backdoor.Commands,
			dllhijack.Commands,
			cursed.Commands,
			wasm.Commands,
		)

		// [ Privileges ]
		bind(consts.PrivilegesHelpGroup,
			privilege.Commands,
		)

		// [ Processes ]
		bind(consts.ProcessHelpGroup,
			processes.Commands,
		)

		// [ Aliases ]
		bind(consts.AliasHelpGroup)

		// [ Extensions ]
		bind(consts.ExtensionHelpGroup)

		// [ Post-command declaration setup ]----------------------------------------

		// Load Aliases
		aliasManifests := assets.GetInstalledAliasManifests()
		for _, manifest := range aliasManifests {
			_, err := alias.LoadAlias(manifest, sliver, con)
			if err != nil {
				con.PrintErrorf("Failed to load alias: %s", err)
				continue
			}
		}

		// Load Extensions
		extensionManifests := extensions.GetAllExtensionManifests()
		for _, manifest := range extensionManifests {
			mext, err := extensions.LoadExtensionManifest(manifest)
			// Absorb error in case there's no extensions manifest
			if err != nil {
				//con doesn't appear to be initialised here?
				//con.PrintErrorf("Failed to load extension: %s", err)
				fmt.Printf("Failed to load extension: %s\n", err)
				continue
			}

			for _, ext := range mext.ExtCommand {
				extensions.ExtensionRegisterCommand(ext, sliver, con)
			}
		}

		// Rehydrate CNA-defined Beacon aliases from the persistent OPFOR runtime.
		opfor.RegisterCommands(sliver, con)

		// [ Post-command declaration setup ]----------------------------------------

		// Everything below this line should preferably not be any command binding
		// (although you can do so without fear). If there are any final modifications
		// to make to the server menu command tree, it time to do them here.

		sliver.InitDefaultHelpCmd()
		sliver.SetHelpCommandGroupID(consts.SliverCoreHelpGroup)

		// Compute which commands should be available based on the current session/beacon.
		con.ExposeCommands()

		return sliver
	}

	return sliverCommands
}
