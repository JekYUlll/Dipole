package bootstrap

import (
	messagev1 "github.com/JekYUlll/Dipole/api/gen/go/message/v1"
	"github.com/JekYUlll/Dipole/internal/application"
	"github.com/JekYUlll/Dipole/internal/config"
	platformrpc "github.com/JekYUlll/Dipole/internal/platform/rpc"
	messagegrpc "github.com/JekYUlll/Dipole/internal/transport/grpc/message"
	"google.golang.org/grpc"
)

type InternalRPCServer = platformrpc.Server

func NewMessageRPCServer(cfg config.InternalRPC, messages application.MessageApplication) (*InternalRPCServer, error) {
	adapter, err := messagegrpc.NewServer(messages)
	if err != nil {
		return nil, err
	}
	return platformrpc.NewServer(cfg, cfg.MessageListenAddress, []string{"dipole-gateway", "dipole-core"}, func(server *grpc.Server) {
		messagev1.RegisterMessageServiceServer(server, adapter)
	})
}
