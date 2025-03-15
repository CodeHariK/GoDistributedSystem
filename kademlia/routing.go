package kademlia

// NewRoutingTable initializes a routing table.
func NewRoutingTable(selfID KKey) *RoutingTable {
	rt := &RoutingTable{SelfID: selfID}
	for i := 0; i < CONST_KKEY_BIT_COUNT; i++ {
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

// Remove removes the newest node when the bucket is full.
func (rt *RoutingTable) Remove(contact Contact) {
	rt.BucketMu.Lock()
	defer rt.BucketMu.Unlock()

	for i := len(rt.Buckets) - 1; i >= 0; i-- { // Remove newest node
		bucket := rt.Buckets[i]
		if bucket.contacts.Len() > 0 {
			bucket.contacts.Remove(bucket.contacts.Back()) // Remove newest node
			return
		}
	}
}

// FindClosest returns k closest nodes to a target ID.
func (rt *RoutingTable) FindClosest(target KKey) []Contact {
	// Find the appropriate bucket
	bucketIndex := target.Distance(rt.SelfID).LeadingZeros()
	rt.BucketMu.Lock()
	closest := rt.Buckets[bucketIndex].GetContacts()
	rt.BucketMu.Unlock()

	// If not enough contacts, look in neighboring buckets.
	if len(closest) < CONST_K {
		for i := 1; i < 160 && len(closest) < CONST_ALPHA; i++ {
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
	if len(closest) > CONST_K {
		closest = closest[:CONST_K]
	}
	return closest
}
