package state

import (
	"sync/atomic"

	"github.com/google/uuid"
)

var (
	siteID    = uuid.NewString()
	lamport   uint64
	OnLocalOp func(Op) // set by main/UI to broadcast
)

func nextLamport() uint64 {
	return atomic.AddUint64(&lamport, 1)
}

func EmitLocal(op Op) {
	op.Lamport = nextLamport()
	op.Site = siteID
	if OnLocalOp != nil {
		OnLocalOp(op)
	}
}
