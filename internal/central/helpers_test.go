package central

import (
	"context"
	"net"
)

func newLoopbackListener() (net.Listener, error) {
	var lc net.ListenConfig
	return lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
}
