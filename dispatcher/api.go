package dispatcher

// Mode selects which algorithm to dispatch
type Mode int
// Enum of the dispatch mode
// LazyCopy: Copy the data from source to destination using user space buffers.
// SpliceCopy: Use splice to send the data from source to destination.
// ZeroSend: Use zero copy to send the data from source to destination.
const(
  LazyCopy=iota                        
	SpliceCopy
	ZeroSend
)
// Config is embedded in the server's global config.
type ServerConfig struct {
  Mode    Mode                          // Mode of dispatching data
	BufSize int                           // Size of the buffer to use.
}
// AddBytes from the metrics hook
type AddBytes func(int)
// CopyPair() starts two goroutine for src<->dst using ServerConfig.Mode.
// then wait until both directions are done. It returns the first non-EOF error
// if any were to occur.
