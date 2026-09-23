//go:build !darwin && !linux && !freebsd && !openbsd && !dragonfly

package tunnel_handlers

import (
	"4zreco/var/sliver/implant/sliver/transports"
	"4zreco/var/sliver/protobuf/commonpb"
	"4zreco/var/sliver/protobuf/sliverpb"
	"google.golang.org/protobuf/proto"
)

func ShellResizeReqHandler(envelope *sliverpb.Envelope, connection *transports.Connection) {
	resp, _ := proto.Marshal(&commonpb.Empty{})
	connection.SendEnvelope(&sliverpb.Envelope{
		ID:   envelope.ID,
		Data: resp,
	})
}
