package bootstrap

import corerpc "github.com/JekYUlll/Dipole/internal/services/core/rpc"

const (
	coreServiceName    = "dipole-core"
	agentServiceName   = "dipole-agent"
	gatewayServiceName = "dipole-gateway"
	messageServiceName = "dipole-message"
	searchServiceName  = "dipole-search"
	syncServiceName    = "dipole-sync"
)

// InternalRPCServer remains as a type alias for embedded compatibility. Core
// RPC construction is owned by internal/services/core/rpc.
type InternalRPCServer = corerpc.Server
