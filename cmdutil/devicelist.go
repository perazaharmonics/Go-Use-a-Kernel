package cmdutil
import "strings"


// A linked list item or node for the device manager.
type DeviceNode struct{
  prev *DeviceNode
	next *DeviceNode
  cfg *DeviceConfig
}
func (dn *DeviceNode) GetNext() *DeviceNode{
  return dn.next
}
func (dn *DeviceNode) GetPrev() *DeviceNode{
	return dn.prev
}
// Insert a new node after the current node.
func (dn *DeviceNode) InsertAfter(p *DeviceNode){
  if dn.next!=nil{                      // Is the next node the leaf?
	  p.prev=dn.next.prev                 // No, set our prev as its previous.
		dn.next.prev=dn                     // Set the next node's prev to us.                      
	}                                     // Done setting the next node's prev.
	p.next=dn.next                        // Update the next node's next to us.	
}                                       // ----------- InsertAfter --------- //

// Inset a new node before the current node.
func (dn *DeviceNode) InsertBefore(p *DeviceNode){
  if p.prev!=nil{
	  dn.prev=p.prev.next                 // Set our prev to the new node next's prev.
		p.prev.next=dn                      // Position new node before us.
	}                                     // Done setting the new node's next.
	p.prev=dn                             // Set the new node's prev to us.
	dn.next=p                             // Set our next to the new node.
}                                       // ----------- InsertBefore -------- //

type DeviceList struct{
  first *DeviceNode
	last *DeviceNode
}
func (dl *DeviceList) GetFirst() *DeviceNode { return dl.first }
func (dl *DeviceList) GetLast() *DeviceNode { return dl.last }
// Append a new device node to the end of the list.
// This is the main function that adds a new device to the list.
func (dl *DeviceList) Insert(cfg *DeviceConfig) *DeviceNode{
  p:=&DeviceNode{cfg:cfg}               // Create a new device node with the config.
	if dl.first==nil{                     // Is this the first node in the list?
	  dl.first=p                          // Yes, set the first node to this one.
		dl.last=p                           // Set the last node to this one too.
		return p                            // Return the new node.
	}                                     // Else we have at least one node in the list.
	// First determine which is the first item with the same name and domain,
	// or whose type name is lexicographically greater than the new node's name.
	var q *DeviceNode                     // The temporary node
	for p:=dl.GetFirst();p!=nil;p=p.GetNext(){
	  if strings.Compare(p.cfg.Name,cfg.Type)>=0{// Does it meet the condition?
		  q=p                               // Yes, set the temporary node to this one.
			break                             // Break out of the loop.
	  }                                   // Done checking the condition.
	}
	// If the type is the same as the new one, then keep looking until we find
	// a lexicographically greater type.
	if q!=nil{                          // We found something?
		isbef:=strings.Compare(p.cfg.Domain,cfg.Domain)==0&&strings.Compare(p.cfg.Name,cfg.Name)<0
		for q!=nil&&isbef{                // But it is lesser than the new one?
			q=q.GetNext()                   // Continue walking the list.
		}                                 // Done walking the list.
	}
	// ------------------------------ //
	// Now we have either found the end of the list or a node with a domain
	// name that is equal, but a device name that is lexicographically
	// greater than the new node's name.
	// ------------------------------ //
	if q==nil{                        // Did we reach the end of the list?
		dl.last.next=p                  // Yes, set last node to curr node.
		p.prev=dl.last                  // Set the curr node's prev to last node.
		dl.last=p                       // Set the last node to the curr node.
	} else{                           // Else, found a node that meets the condition.
		p.InsertBefore(q)               // Insert the new node before the curr node.
		if (p.prev==nil){               // Is there a  prev node?
			dl.first=p                    // No, set the first node to curr node.
		}                               // Done checking if there is a prev node.       
	}                                 // Done with else.
	return p                          // Return the new node.
}                                   // -------------- Insert ---------------- //