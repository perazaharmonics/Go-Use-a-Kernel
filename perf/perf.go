//go:build linux
package perf
import(
  "encoding/binary"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)
// ------------------------------------ //
// openCycles returns an *os.File whose Read() gives the current CPU-cycle count
// for the calling thread (FD is CLOEXEC). User should call ReadUint64()
// to read the cycle count before/after the critical section and subtract
// the two values to get the cycle count for that section.
// ------------------------------------ //
/*
type PerfEventAttr struct {
    Type   uint32
    Size   uint32
    Config uint64
    Union_0 [8]byte // Sample_period or Sample_freq
    Sample_type uint64
    Read_format uint64
    Flags  uint64    <---- Contains the flags like Exclude_kernel, Exclude_idle, etc.
    Union_1 [8]byte //Wakeup_events or Watermark
    Bp_type uint32
    Union_2 [8]byte // Config1 or BP_addr
    Union_3 [8]byte // Config2 or BP_len
    Branch_sample_type uint64
    Sample_regs_user uint64
    Sample_stack_user uint32
    Spare_2 uint32
    Sample_regs_intr uint64
    Aux_watermark uint32
    Spare_3 uint32
    Mmap_data uint64
    Mmap_addr uint64
    Kernel_overflow_count uint32
    Tail_opts uint32
    Sample_max_stack uint16
    Reserved_1 uint16
    Aux_sample_size uint32
    Reserved_2 uint32
}
*/
func OpenCycles() (*os.File,error){
  attr:=unix.PerfEventAttr{
	  Type:        unix.PERF_TYPE_HARDWARE,
		Config:      unix.PERF_COUNT_HW_CPU_CYCLES,
		Size:        uint32(binary.Size(unix.PerfEventAttr{})),
		//Flags:       unix.PerfBitExcludeIdle|unix.PerfBitExcludeHv,
	}                                     // Done defining the perf_event_attr struct
	fd,err:=unix.PerfEventOpen(&attr,     // Our attribute struct.
	0,                                    // PID=0 is set to pid of the calling thread.
	-1,                                   // cpu=-1 any CPU its scheduled to run on.
	-1,                                   // group_fd=-1 no group.
	unix.PERF_FLAG_FD_CLOEXEC,            // CLOEXEC flag.
	)          // Done getting the fd assigned for the perf_event_open syscall.
	if err!=nil{                          // Error opening the perf event and getting fd?
	  return nil,fmt.Errorf("perf_event_open: %w",err)
	}                                     // Done with error opening perf event.
	// ---------------------------------- //
	// We have a valid fd, so now we can return the file associated with it.
	// ---------------------------------- //
	return os.NewFile(uintptr(fd),"perf_cycle"),nil
}                                       // ----------- openCycles ----------- //
// ------------------------------------ //
// ReadUint64 fetches the current current CPU-cycle count from the file
// associated with the perf_event_open syscall.
// It returns the cycle count as a uint64.
// ------------------------------------ //
func ReadUint64(f *os.File) (uint64,error){
  var buf [8]byte                       // Buffer to read the cycle count into.
	_,err:=f.Read(buf[:])                 // Read cycle count from file into buf.
	return binary.LittleEndian.Uint64(buf[:]),err // Return the cycle count as uint64.
}                                       // ----------- ReadUint64 ----------- //
