package kademlia

// AddContact adds a new contact or moves it to the front if it already exists.
func (b *KBucket) AddContact(c Contact) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	for e := b.contacts.Front(); e != nil; e = e.Next() {
		if e.Value.(Contact).ID == c.ID {
			b.contacts.MoveToFront(e)
			return
		}
	}

	if b.contacts.Len() < CONST_K {
		b.contacts.PushFront(c)
	} else {
		// Bucket is full, Kademlia suggests PINGing the least recently seen node.
		b.contacts.MoveToBack(b.contacts.Front())
	}
}

// GetContacts returns a list of contacts in the bucket.
func (b *KBucket) GetContacts() []Contact {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var contacts []Contact
	for e := b.contacts.Front(); e != nil; e = e.Next() {
		contacts = append(contacts, e.Value.(Contact))
	}
	return contacts
}
