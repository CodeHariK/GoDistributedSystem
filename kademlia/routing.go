package kademlia

import (
	"bytes"
	"container/heap"
)

// NewRoutingTable initializes a routing table.
func NewRoutingTable(selfID KKey) *RoutingTable {
	rt := &RoutingTable{}
	for i := 0; i < CONST_KKEY_BIT_COUNT; i++ {
		rt.Buckets[i] = &KBucket{}
	}
	return rt
}

// AddContact adds a node to the correct bucket.
func (node *Node) AddContact(c Contact) {
	bucketIndex := c.key.Xor(node.Key).LeadingZeros()
	node.routingTable.mu.Lock()
	node.routingTable.Buckets[bucketIndex].AddContact(c)
	node.routingTable.mu.Unlock()
}

// Remove removes the newest node when the bucket is full.
func (node *Node) Remove(c Contact) {
	node.routingTable.mu.Lock()
	defer node.routingTable.mu.Unlock()

	bucketIndex := c.key.Xor(node.Key).LeadingZeros()

	node.routingTable.Buckets[bucketIndex].contacts.Remove(c.key)
}

func (node *Node) NumContacts() int {
	num := 0
	for _, b := range node.routingTable.Buckets {
		num += b.contacts.Len()
	}
	return num
}

func (node *Node) FewContacts() []Contact {
	contacts := make([]Contact, CONST_KKEY_BIT_COUNT)
	n := 0
	for i, b := range node.routingTable.Buckets {
		c := b.contacts.Peek()
		if c != nil {
			contacts[i] = *c
			n++
		}
	}
	return contacts[:n]
}

// FindClosest returns k closest nodes to a target ID.
func (node *Node) FindClosest(target KKey) []Contact {
	// Find the appropriate bucket
	bucketIndex := target.Xor(node.Key).LeadingZeros()

	if bucketIndex == CONST_KKEY_BIT_COUNT {
		return []Contact{}
	}

	node.routingTable.mu.Lock()
	closest := node.routingTable.Buckets[bucketIndex].GetContacts()
	node.routingTable.mu.Unlock()

	// If not enough contacts, look in neighboring buckets.
	if len(closest) < CONST_K {
		for i := 1; i < CONST_KKEY_BIT_COUNT && len(closest) < CONST_ALPHA; i++ {
			if bucketIndex-i >= 0 {
				node.routingTable.mu.Lock()
				closest = append(closest, node.routingTable.Buckets[bucketIndex-i].GetContacts()...)
				node.routingTable.mu.Unlock()
			}
			if bucketIndex+i < CONST_KKEY_BIT_COUNT {
				node.routingTable.mu.Lock()
				closest = append(closest, node.routingTable.Buckets[bucketIndex+i].GetContacts()...)
				node.routingTable.mu.Unlock()
			}
		}
	}

	// Return k closest contacts
	if len(closest) > CONST_K {
		closest = closest[:CONST_K]
	}
	return closest
}

func (node *Node) FindSuccessor(key KKey) Contact {
	closestNodes := node.FindClosest(key)

	// Instead of XOR, find the first node *greater than or equal* to key
	for _, contact := range closestNodes {
		if bytes.Compare(contact.key[:], key[:]) >= 0 {
			return contact
		}
	}

	// If no node is greater, wrap around (i.e., return the first node in the ring)
	return closestNodes[0]
}

func (h ContactHeap) Len() int { return len(h) }

func (h ContactHeap) Less(i, j int) bool {
	// Higher StartTime is better, lower RTT is better.
	if h[i].StartTime == h[j].StartTime {
		return h[i].RTT < h[j].RTT
	}
	return h[i].StartTime > h[j].StartTime
}

func (h ContactHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *ContactHeap) Push(x any) {
	*h = append(*h, x.(Contact)) // Modify heap in place
}

func (h *ContactHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1] // Modify heap in place
	return item
}

// Remove removes a contact by key while maintaining heap properties.
func (h *ContactHeap) Remove(key KKey) {
	for i, contact := range *h {
		if contact.key == key {
			heap.Remove(h, i) // Use heap.Remove to maintain heap structure
			return
		}
	}
}

// Peek returns the highest-priority contact without removing it.
func (h *ContactHeap) Peek() *Contact {
	if len(*h) == 0 {
		return nil
	}
	return &(*h)[0]
}

// NewContactHeap initializes a ContactHeap.
func NewContactHeap() *ContactHeap {
	h := &ContactHeap{}
	heap.Init(h)
	return h
}

// AddContact adds a new contact or updates its position in the heap.
func (b *KBucket) AddContact(c Contact) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if contact already exists and update it
	for i, contact := range b.contacts {
		if contact.key == c.key {
			b.contacts[i] = c        // Update the contact
			heap.Fix(&b.contacts, i) // Reorder heap
			return
		}
	}

	// Add new contact if space is available
	if b.contacts.Len() < CONST_K {
		heap.Push(&b.contacts, c)
	} else {
		// Remove the least prioritized contact and add the new one
		heap.Pop(&b.contacts)
		heap.Push(&b.contacts, c)
	}
}

// GetContacts returns a list of contacts in the bucket.
func (b *KBucket) GetContacts() []Contact {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Return a copy of the heap as a slice
	contactsCopy := make([]Contact, len(b.contacts))
	copy(contactsCopy, b.contacts)
	return contactsCopy
}
