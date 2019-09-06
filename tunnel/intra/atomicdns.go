package intra

import (
	"sync/atomic"
)

// atomicdns is atomic.Value, specialized for DNSTransport.
type atomicdns atomic.Value

func (a *atomicdns) Store(d DNSTransport) {
	a.Store(d)
}

func (a *atomicdns) Load() DNSTransport {
	return a.Load().(DNSTransport)
}
