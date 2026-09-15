package info

import (
	"context"

	"4zreco/sliver/client/console"
	"4zreco/sliver/protobuf/sliverpb"
	"4zreco/sliver/util"
	"github.com/spf13/cobra"
)

// PingCmd - Send a round trip C2 message to an implant (does not use ICMP).
func PingCmd(cmd *cobra.Command, con *console.SliverClient, args []string) {
	session := con.ActiveTarget.GetSessionInteractive()
	if session == nil {
		return
	}

	nonce := util.Intn(999999)
	con.PrintInfof("Ping %d\n", nonce)
	pong, err := con.Rpc.Ping(context.Background(), &sliverpb.Ping{
		Nonce:   int32(nonce),
		Request: con.ActiveTarget.Request(cmd),
	})
	if err != nil {
		con.PrintErrorf("%s\n", err)
	} else {
		con.PrintInfof("Pong %d\n", pong.Nonce)
	}
}
