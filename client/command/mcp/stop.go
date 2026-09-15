package mcp

import (
	"context"
	"time"

	"4zreco/sliver/client/console"
	clientmcp "4zreco/sliver/client/mcp"
	"github.com/spf13/cobra"
)

const mcpStopTimeout = 5 * time.Second

// McpStopCmd stops the local MCP server.
func McpStopCmd(cmd *cobra.Command, con *console.SliverClient, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), mcpStopTimeout)
	defer cancel()

	if err := clientmcp.Stop(ctx); err != nil {
		con.PrintErrorf("%s\n", err)
		return
	}
	con.PrintInfof("MCP server stopped\n")
}
