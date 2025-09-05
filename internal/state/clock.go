package state

// import "sync/atomic"

// // Clock is a simple logical clock using an atomic counter.
// type Clock struct {
// 	time uint64
// }

// // Tick increments the clock and returns the new time.
// func (c *Clock) Tick() uint64 {
// 	return atomic.AddUint64(&c.time, 1)
// }