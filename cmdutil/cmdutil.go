package cmdutil
import(
  "flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"strings"

	conf "github.com/ljt/DeviceManager/internal/configuration"
	utils "github.com/ljt/DeviceManager/internal/utils"
)




// DeviceConfig holds everything our app needs to know about to configure
// and pass it onwards to the device manager.
type DeviceConfig struct{
	Name string               // Name of the device.
	Type string               // Device type, e.g. "usb","serial","xdma", and so on...
	Domain string             // Domain name for the device, e.g. "trivium-solutions.com"
  Resource string           // Fully qualified resource name (domain/device)
	Vendor   string           // Optional vendor ID. (The USB-Serial uses this.)
	Product string            // Optional product ID.
	Driver string             // Optiona driver name used for IIO devices (which don't use.... for now)
	Interval time.Duration    // Interval for scanning for the devices
	MetricsAddr string        // Address for the metrics server
  Groups [][]string         // Groups of devices, e.g. {USB1,USB3} for catt-0 and so on...'
	// This struct actually just gets populated once, and then never touched again
	// so there is no concurrent access that could ever happen.
	//Mtx sync.Mutex            // Mutex to protect the DeviceConfig from concurrent access.
}



//Parse Flags initialises common CLI flags once for every binary.
// Parsing flags is also optional, if no flags are provided, the defaults
// will be used. This is useful say, if you wanted to change the scan interval
// then an operator could do so by passing the flag on the command line. It
// would prevent the need to recompile the binary just to change the scan interval.
func parseFlags(_ string) (time.Duration,string){
  interval:=flag.Duration("interval",5*time.Second,// Fetch interval or default to 5 seconds.
	"device rescan interval (default: 5 seconds)")   // Help text for the flag.
	metrics:=flag.String("metrics-addr",":9090",     // Fetch metrics server address or default to :9090.
	"address for the metrics server (default: :9090)")// Help text for the flag.
	flag.Parse()                          // Parse the flags.
	return *interval,*metrics             // Return the parsed flags.
}                                       // ------------- ParseFlags --------- //
// LoadDeviceConfiguration merges (in precedence order):
// 1. CLI flags/environment variables
// 2. Configuration file reading and loading into Configuration object
// 3. Default values
func LoadDeviceConfiguration(sect string,def *DeviceConfig) (*DeviceConfig,error){
var log=utils.GetLogger()
  // ---------------------------------- //
	// Parse the command line flags, then check the environment variables
	// to see if we have any overrides defined by symbols.
	// ---------------------------------- //
	interval,metrics:=parseFlags(sect)    // Parse the flags (if any).
	if v:=os.Getenv("METRICS_ADDR");v!=""{// Do we have a predilect metrics address?
	  metrics=v                           // Override the metrics address from env var.
	}                                     // Done checking for metrics address override.
	if v:=os.Getenv("SCAN_INTERVAL");v!=""{// Do we have a predilect scan interval?
	  if d,err:=time.ParseDuration(v);err==nil{// Could we parse the duration?
		  interval=d                        // Yes, override the interval value.
		}                                   // Done with if could parse the duration.
  }                                     // Done checking interval override.
	// ---------------------------------- //
	// Now we will read the configuration file, and store our parameters
	// from the Configuration object and into our DeviceConfig object.
	// ---------------------------------- //
	devman:=os.Getenv("DEVMAN")           // Get the DEVMAN environment
	var cpath string                      // Absolute path to the configuration file.
	if devman==""{                        // Is the DEVMAN symbol defined?
	  log.War("$DEVMAN environment variable not set, falling back to user's $HOME") // Log a warning.
		home:=os.Getenv("HOME")             // No, use the user's home directory.
		cpath=filepath.Join(home,"Projects","NetGo","DeviceManager","cfg","devices.cfg") // Default path to the configuration file.
	} else{                               // Else $DEVMAN is defined.
	  cpath=filepath.Join(devman,"cfg","devices.cfg") // Use the DEVMAN path.
	}                                     // Done checking symbology.
	cfg:=&conf.Configuration{}            // Our configuration object.
	// ---------------------------------- //
	// This call should almost never fail, but if it does, we will notify the
	// caller so that they can override the values with defaults on their side.
	// ---------------------------------- //
	if err:=cfg.ReadFile(cpath,"",false);err!=nil{
	  log.Err("Failed to read configuration file %s: %v",cpath,err) // Log an error.
		return nil,fmt.Errorf("failed to read configuration file %s: %v",cpath,err) // Return an error.
	}                                     // Done reading the configuration file.
	s:=cfg.GetSection(sect)               // Get the requested section.
	def.Name=s.GetName()                  // Set the name of the device to the section name.
	if s!=nil{                            // Did we find the requested section?
	  log.Inf("Found section \"%s\" in the configuration file %s",sect,cpath)
		if p:=s.FindFirstParameter();p!=nil{// Could we find the first parameter?
		  for p!=nil{                       // For every parameter in THIS section.
			  populateDeviceConfig(p,def) // Populate the DeviceConfig with the parameter.
				p=p.GetNext()                   // Get the next parameter in the section.
			}                                 // Done iterating through the parameters.
		}                                   // Done checking if found first parameter.
	}                                     // We found the section if we got here.
	log.Inf("Using section \"%s\" with values: resource=\"%s\", vendor=\"%s\", product=\"%s\"",
		sect,def.Resource,def.Vendor,def.Product) // Log the values we
	// ---------------------------------- //
	// Now build the remaining fields in the DeviceConfig struct.
	// ---------------------------------- //
	if def.Resource==""{
	  def.Resource=fmt.Sprintf("%s/%s",def.Domain,def.Name)
	}
	def.Interval=interval                 // Set the default scan interval.
	def.MetricsAddr=metrics               // Set the metrics address.
	return def,nil                        // We have the object, return it with no error.
}                                       // ---- LoadDeviceConfiguration ----- //

func populateDeviceConfig(p *conf.Parameter, def *DeviceConfig){
	switch p.GetName(){                   // Act according to the parameter name.
		case "domain":                      // Is it the domain?
			def.Resource=p.GetValue(0)        // Set the resource name.
		case "vendor":                      // Is it the vendor ID?
			def.Vendor=p.GetValue(0)          // Set the vendor ID.
		case "type":                        // Does not do much, but it helps logging.
			def.Type=p.GetValue(0)            // Set the device type.
		case "product":                     // Is it the product ID?
			def.Product=p.GetValue(0)         // Set the product ID.
		case "driver":                      // Is it the driver name?
			def.Driver=p.GetValue(0)          // Set the driver name.
		case "group0":                      // Is it the first CATT group?
			raw:=strings.Split(p.GetValue(0),",") // Split the value by commas.
			var tuple []string                // Create a new tuple for the group.
			for _,v:=range raw{               // For the values in raw slice..
				v=strings.TrimSpace(v)          // Trim the value.
				if v!=""{                       // Is the value empty?
					tuple=append(tuple,v)         // No, append the value to the tuple.
				}                               // Done checking if value is empty.
			}                                 // Done iterating through the raw values.
			if len(tuple)>0{                  // Do we have any values in the tuple?
				def.Groups=append(def.Groups,tuple) // Yes, appen tuple to groups.
			}                                 // Done checking for values in tuple.
		case "group1":                      // Is it the second CATT group?
			raw:=strings.Split(p.GetValue(0),",") // Split the value by commas.
			var tuple []string                // Create a new tuple for the group.
			for _,v:=range raw{               // For the values in raw slice..
				v=strings.TrimSpace(v)          // Trim the value.
				if v!=""{                       // Is the value empty?
					tuple=append(tuple,v)         // Append the value to the tuple.
				}                               // Done checking if value is empty.
			}                                 // Done iterating through the raw values.
			if len(tuple)>0{                  // Do we have any values in the tuple?
				def.Groups=append(def.Groups,tuple) // Append the tuple to the groups.
			}                                 // Done checking for values in tuple.
	}                                     // End of switch statement.
}                                       // Done iterating through the parameters.

func LoadAllDeviceConfigurations(sect string,def *DeviceConfig) ([]*DeviceConfig,error){
  var log=utils.GetLogger()             // Our shared log object.
  var result []*DeviceConfig            // Our result slice of DeviceConfig objects.
  // ---------------------------------- //
	// Parse the command line flags, then check the environment variables
	// to see if we have any overrides defined by symbols.
	// ---------------------------------- //
	interval,metrics:=parseFlags("all")    // Parse the flags (if any).
	if v:=os.Getenv("METRICS_ADDR");v!=""{// Do we have a predilect metrics address?
	  metrics=v                           // Override the metrics address from env var.
	}                                     // Done checking for metrics address override.
	if v:=os.Getenv("SCAN_INTERVAL");v!=""{// Do we have a predilect scan interval?
	  if d,err:=time.ParseDuration(v);err==nil{// Could we parse the duration?
		  interval=d                        // Yes, override the interval value.
		}                                   // Done with if could parse the duration.
  }                                     // Done checking interval override.
	// ---------------------------------- //
	// Determine the config-file path.
	// ---------------------------------- //
	devman:=os.Getenv("DEVMAN")           // Get the DEVMAN environment
	var cpath string                      // Absolute path to the configuration file.
	if devman==""{                        // Is the DEVMAN symbol defined?
	  log.War("$DEVMAN environment variable not set, falling back to user's $HOME") // Log a warning.
		home:=os.Getenv("HOME")             // No, use the user's home directory.
		cpath=filepath.Join(home,"Projects","NetGo","DeviceManager","cfg","devices.cfg") // Default path to the configuration file.
	} else{                               // Else $DEVMAN is defined.
	  cpath=filepath.Join(devman,"cfg","devices.cfg") // Use the DEVMAN path.
	}                                     // Done checking symbology.
	cfg:=&conf.Configuration{}            // Our configuration object.
	// ---------------------------------- //
	// This call should almost never fail, but if it does, we will notify the
	// caller so that they can override the values with defaults on their side.
	// ---------------------------------- //
	if err:=cfg.ReadFile(cpath,"",false);err!=nil{
	  log.Err("Failed to read configuration file %s: %v",cpath,err) // Log an error.
		return nil,fmt.Errorf("failed to read configuration file %s: %v",cpath,err) // Return an error.
	}                                     // Done reading the configuration file.
	// ---------------------------------- //
	// Now do a walk through every section in the configuration file,
	// or through the requested section if it is specified.
	// ---------------------------------- //
	for s:=cfg.GetFirst();s!=nil;s=s.GetNext(){
	  if sect!=""&&s.GetName()!=sect{     // We have a section name, and it does not match.
		  continue                          // Yes, skip this section. 
		}                                   // Done checking for section name.
	// ---------------------------------- //
	// Clone the template so we never modify the original.
	// ---------------------------------- //
	  defclone:=*def                      // Clone the DeviceConfig template.
	  if defclone.Type==""{ defclone.Type=s.GetName() } // Set the type to the section name if not set.
	// ---------------------------------- //
	// Populate with stanza parameters.
	// ---------------------------------- //
	  p:=s.FindFirstParameter()           // Get the first parameter in the section.
	  for p!=nil{                         // While we have parameters in the section.
	    populateDeviceConfig(p,&defclone) // Populate the DeviceConfig with the parameter.
	  	p=p.GetNext()                     // Get the next parameter in the section.
	  }                                   // Done iterating through the parameters.
	// ---------------------------------- //
	// Now build the remaining fields in the DeviceConfig struct.
	// ---------------------------------- //
	  if defclone.Domain==""{             // Do we have a domain set?
	    defclone.Domain="trivium-solutions.com" // Set the default domain.
	  }                                   // Done checking for domain.
		if defclone.Resource==""{           // Do we have a resource set?
		  defclone.Resource=fmt.Sprintf("%s/%s",defclone.Domain,s.GetName()) // Set the resource name.
		}
		defclone.Interval=interval         // Set the default scan interval.
		defclone.MetricsAddr=metrics       // Set the metrics address.
		result=append(result,&defclone)    // Append the DeviceConfig to the result slice.
		log.Inf("Using section \"%s\" with values: resource=\"%s\", vendor=\"%s\", product=\"%s\"",
			s.GetName(),defclone.Resource,defclone.Vendor,defclone.Product) // Log the values we used.
	}                                     // Done iterating through the sections.
	if len(result)==0{                    // Did we not find any sections?
		log.Err("No sections found in the configuration file %s",cpath) // Log an error.
		return nil,fmt.Errorf("no sections found in the configuration file %s", cpath) // Return an error.
	}                                     // Done checking for sections.
	return result,nil                     // Return the result slice with no error.
}                                       // ---- LoadAllDeviceConfigurations ----- //

