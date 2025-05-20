/*
==============================================================================
* Filename: config.go
* Description: A quite simple configuration file reader that reads a YAML
*  configuration file and unmarshals it into a struct. Eventually, more
* complex configuration file parsing will be added, but only on a need to have
* basis. This configuration file reader is used to read the mapping of
* routable IP addresses to Kubernetes pod DNS names.
*
* Author:
*  J.EP J. Enrique Peraza, enrique.peraza@trivium-solutions.com
* Organizations:
*  Trivium Solutions LLC, 9175 Guilford Road, Suite 220, Columbia, MD 21046
==============================================================================
*/
package config

import (
	"os" // File operations, I/O, system calls
	"strconv"
	"strings"

	"github.com/ljt/ProxyServer/internal/logger" // Our custom log package.
	"gopkg.in/yaml.v3"                           // YAML decoding and encoding

	//	"github.com/ljt/ProxyServer/internal/utils" // Our Handlers and Callbacks functions
	"path/filepath" // For file path manipulation
)

// var log = utils.GetLogger()             // Our log object.
// const debug = true                    // Debug flag to enable/disable debug logging
// ------------------------------------ //
// Mapping that represents one stable-to-pod mapping from the config file
// It maps stable addresses to Kubernetes pod DNS names
// ------------------------------------ //
type Mapping struct { // Our mapping object
	Alias string `yaml:"alias"` // Stable address is from a yaml file
	Pods  string `yaml:"pods"`  // Pod DNS name is from a yaml file
}                       // Mapping struct
type Mappings []Mapping // Mappings is a slice of Mapping objects
// ------------------------------------ //
// Config to represent the complete YAML configuration structure
// It contains a list of mappings       //
// ------------------------------------ //
type Config struct { // Our configuration object
	Mappings Mappings   `yaml:"mappings"` // List of mappings
	log      logger.Log // Logger object
} // Config struct
// ------------------------------------ //
// An initializer for the Config struct.//
// ------------------------------------ //
func NewConfig(log logger.Log) *Config { // Our initializer for the Config object.
	return &Config{ // Return a new Config object
		Mappings: Mappings{}, // Initialize the mappings slice
		log:      log,        // Initialize the log object
	} // Done initializing the Config object
} // ---------- NewConfig ------------- //
// ------------------------------------ //
// Getter for the Mappings object. It returns a slice of Mapping objects.
// ------------------------------------ //
func (c *Config) GetMappings() Mappings { // Get the mappings from the config
	return c.Mappings // Return the list of mappings
} // ----------- GetMappings ---------- //
// ------------------------------------ //
// Return a map of all pod names found in mappings.
// ------------------------------------ //
func (c *Config) ValidPods() map[string]bool {
	valid := make(map[string]bool) // Create a map to store valid pod names
	for _, m := range c.Mappings { // Iterate over the mappings
		if m.Pods != "" { // Is the pod name not empty?
			valid[m.Pods] = true // Yes, add it to the map
		} // Done checking the pod name.
	} // Done checking all pod names.
	return valid // Return the map of valid pod names
} // ----------- ValidPods ------------ //
func (m *Mapping) Target() string {
	if strings.Contains(m.Pods, ":") {
		return m.Pods
	}
	return ""
}

// ------------------------------------ //
// ReadConfig decodes and loads the YAML configuration file
// into a Config struct. It takes the path to the YAML file as an argument.
// It returns a pointer to the Config struct and an error if any occurs.
// ------------------------------------ //
func ReadConfig(path string, log logger.Log) (*Config, error) {
	absPath, err := filepath.Abs(path) // Get the absolute path
	if err != nil {                    // Could we find the file at absPath?
		log.Err("Could not find file %s: %v", absPath, err)
		return nil, err // No, return nil and the error
	} // Done checking the file path.
	// ---------------------------------- //
	// Verify for the existence of the file.
	// ---------------------------------- //
	if _, err := os.Stat(absPath); os.IsNotExist(err) { // Does file exists?
		log.Err("File %s does not exist: %v", absPath, err)
		return nil, err // No, return nil and the error
	} // Done checking for file existence.
	// ---------------------------------- //
	// Open the YAML file for reading. (Readfile closes the file when done.)
	// ---------------------------------- //
	data, err := os.ReadFile(absPath) // Read the file
	if err != nil {                   // Could we read the file?
		log.Err("Could not read file %s: %v", absPath, err)
		return nil, err // No, return nil and the error
	} // Done with file read err
	// ---------------------------------- //
	// Attempt to unmarshal the YAML data into the Config struct
	// catch any errors that occur during the unmarshalling process
	// ---------------------------------- //
	var cfg Config // Create a new Config struct
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Err("Could not unmarshal YAML file %s: %v", absPath, err)
		return nil, err // No, return nil and the error
	} // Done unmarshalling the YAML data
	if len(cfg.Mappings) == 0 { // Any bytes in the Mappings slice?
		log.Err("No mappings found in file %s", absPath)
		return nil, err // No mappings found, return nil and error
	} // Done checking if mappings exists
	return &cfg, nil // Return the cfg struct and nil error.
} // ----------- ReadConfig ----------- //

// ------------------------------------ //
// Function to sort the mappings by pod name.
// ------------------------------------ //
func (c *Config) SortMappingsByName() Mappings {
	if len(c.Mappings) == 0 { // Any mappings in the list?
		c.log.Err("No mappings found in the proxy mapping table")
		return nil // No, return nil.
	} // Done checking the mappings.
	maps := c.GetMappings()          // Get the mappings from the config
	for i := 0; i < len(maps); i++ { // For each mapping in the list.
		for j := i + 1; j < len(maps); j++ { // .. and the element after it
			if maps[i].Pods > maps[j].Pods { // Is the first element greater?
				maps[i], maps[j] = maps[j], maps[i] // Yes, swap them.
			} // Done swapping the elements.
		} // Done checking its neighbors.
	} // Done checking all elements.
	for _, m := range maps { // For each mapping in the list.
		if m.Pods != "" { // Is the pod name empty?
			return maps // No, return the sorted list of mappings
		} // Done checking the pod name.
	} // Done checking all mappings.
	return nil // If we got here something is wrong.
} // --- GetListenerAddressByPodName -- //

// ------------------------------------ //
// Function to convert the IP addresses from string to int.
// ------------------------------------ //
func (c *Config) SortMappingsByIP() Mappings {
	if len(c.Mappings) == 0 { // Any mappings in the list?
		return nil // No, return nil.
	} // Done checking the mappings.
	maps := c.GetMappings()          // Get the mappings from the config
	ipInt := 0                       // Initialize the IP address to int
	var ips []int                    // Create a slice to store the IP addresses
	for i := 0; i < len(maps); i++ { // For each mapping in the list.
		addr, _ := strconv.Atoi(maps[i].Alias) // Convert the IP address to int
		ipInt = ipInt<<8 + addr                // Shift the IP address to the left and add the new byte
		ips = append(ips, ipInt)               // Append the IP address to the slice
	} // Done converting the IP address to int.
	for i := 0; i < len(maps); i++ { // For each mapping in the list.
		for j := i + 1; j < len(maps); j++ { // .. and the element after it
			if ips[i] > ips[j] { // Is the first element greater?
				maps[i], maps[j] = maps[j], maps[i] // Yes, swap them.
			} // Done swapping the elements.
		} // Done checking its neighbors.
	} // Done checking all elements.
	for _, m := range maps { // For each mapping in the list.
		if m.Alias != "" { // Is the pod IP address (alias) empty?
			return maps // No return the sorted list of mappings
		} // Done checking the pod alias.
	} // Done checking all mappings.
	return nil // If we got here something is wrong.
}

// ------------------------------------ //
// Helper function to get the pod IP address from the cache.
// It returns the IP address of the pod if it exists in the cache,
// ------------------------------------ //
func (c *Config) GetListenerAddressByPodName(podname string) string {
	if len(c.Mappings) == 0 { // Any mappings in the list?
		return "" // No, return empty string.
	} // Done checking the mappings.
	// ---------------------------------- //
	// Sort the mappings by pod name so we can do a binary search.
	// ---------------------------------- //
	maps := c.SortMappingsByName() // Sort the mappings by pod name.
	low := 0                       // Initialize the low index to 0
	high := len(maps) - 1          // Initialize the high index to the last element
	for low <= high {              // While low is less than or equal to high.
		mid := low + (high-low)/2      // Calculate the middle index.
		if maps[mid].Pods == podname { // Did we find it in the middle?
			return maps[mid].Alias // Yes, return the IP address.
		} else if maps[mid].Pods < podname { // Is the pod name less than the middle?
			low = mid + 1 // Yes, shorten the search to the right.
		} else { // Else podname is greater than the middle.
			high = mid - 1 // So, shorten the search to the left.
		} // Done checking the pod name.
	} // Done searching for the pod name.
	return "" // If we got here, something is wrong.
} // --- GetListenerAddressByPodName -- //
