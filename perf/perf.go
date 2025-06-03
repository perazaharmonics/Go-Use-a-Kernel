//go:build linux
package perf
import(
  "encoding/binary"
	"fmt"
	"os"
  "syscall"

	"golang.org/x/sys/unix"
)
// ------------------------------------ //
// openCycles returns a Counter object that is used to read the CPU cycle count.
// It uses the perf_event_open syscall to create a file descriptor that can be used
// to read the CPU cycle count. The file descriptor is used to read the cycle count
// using the ReadUint64 method of the Counter object.
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
type Counter struct{
  fd int
}
func OpenCycles() (*Counter,error){
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
  // Reset + enable the perf event.
  // ---------------------------------- //
  if _,_,errno:=syscall.Syscall(syscall.SYS_IOCTL,uintptr(fd),
    uintptr(unix.PERF_EVENT_IOC_RESET),0);errno!=0{
      return nil,fmt.Errorf("ioctl(PERF_EVENT_IOC_RESET): %w",errno)
  }
  if _,_,errno:=syscall.Syscall(syscall.SYS_IOCTL,uintptr(fd),
    uintptr(unix.PERF_EVENT_IOC_ENABLE),0);errno!=0{
      return nil,fmt.Errorf("ioctl(PERF_EVENT_IOC_ENABLE): %w",errno)
  }
  return &Counter{fd},nil
}                                       // ----------- openCycles ----------- //
// ------------------------------------ //
// ReadUint64 fetches the current current CPU-cycle count from the file
// associated with the perf_event_open syscall.
// It returns the cycle count as a uint64.
// ------------------------------------ //
func (c *Counter) ReadUint64(f *os.File) (uint64,error){
  var buf [8]byte                       // Buffer to read the cycle count into.
	if _,err:=unix.Read(c.fd,buf[:]);err!=nil{
    return 0,fmt.Errorf("read perf event fd: %w",err)
  }
  return binary.LittleEndian.Uint64(buf[:]),nil // Return the cycle count as uint64.
}                                       // ----------- ReadUint64 ----------- //
func (c *Counter) Close() { unix.Close(c.fd) }
