package kademlia

// NewRoutingTable initializes a routing table.
func NewRoutingTable(selfID NodeID) *RoutingTable {
	rt := &RoutingTable{SelfID: selfID}
	for i := 0; i < 160; i++ {
		rt.Buckets[i] = &KBucket{}
	}
	return rt
}

// AddContact adds a node to the correct bucket.
func (rt *RoutingTable) AddContact(c Contact) {
	bucketIndex := c.ID.Distance(rt.SelfID).LeadingZeros()
	rt.BucketMu.Lock()
	rt.Buckets[bucketIndex].AddContact(c)
	rt.BucketMu.Unlock()
}

// FindClosest returns k closest nodes to a target ID.
func (rt *RoutingTable) FindClosest(target NodeID, k int) []Contact {
	// Find the appropriate bucket
	bucketIndex := target.Distance(rt.SelfID).LeadingZeros()
	rt.BucketMu.Lock()
	closest := rt.Buckets[bucketIndex].GetContacts()
	rt.BucketMu.Unlock()

	// If not enough contacts, look in neighboring buckets.
	if len(closest) < k {
		for i := 1; i < 160 && len(closest) < k; i++ {
			if bucketIndex-i >= 0 {
				rt.BucketMu.Lock()
				closest = append(closest, rt.Buckets[bucketIndex-i].GetContacts()...)
				rt.BucketMu.Unlock()
			}
			if bucketIndex+i < 160 {
				rt.BucketMu.Lock()
				closest = append(closest, rt.Buckets[bucketIndex+i].GetContacts()...)
				rt.BucketMu.Unlock()
			}
		}
	}

	// Return k closest contacts
	if len(closest) > k {
		closest = closest[:k]
	}
	return closest
}
