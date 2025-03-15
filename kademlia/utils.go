package kademlia

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/bits"

	"github.com/codeharik/kademlia/api"
)

// NewNodeID generates a random NodeID.
func NewNodeID() KKey {
	var id KKey
	rand.Read(id[:])
	return id
}

// _________________ KKey

func (id KKey) HexString() string {
	return hex.EncodeToString(id[:])
}

func (id KKey) ApiKKey() (*api.KKey, error) {
	if len(id) != 20 {
		return nil, errors.New("Key length not 20")
	}
	key := api.KKey{Key: id[:]}
	return &key, nil
}

func ToKKey(k *api.KKey) (KKey, error) {
	if len(k.Key) != 20 {
		return KKey{}, errors.New("Key length not 20")
	}
	var id KKey
	copy(id[:], k.Key)
	return id, nil
}

// Distance calculates XOR distance between two NodeIDs.
func (id KKey) Distance(other KKey) KKey {
	var dist KKey
	for i := 0; i < len(id); i++ {
		dist[i] = id[i] ^ other[i]
	}
	return dist
}

// LeadingZeros returns the index of the first nonzero bit.
func (id KKey) LeadingZeros() int {
	for i := 0; i < len(id); i++ {
		if id[i] != 0 {
			return i*8 + bits.LeadingZeros8(id[i])
		}
	}
	return 160 // All zero case (extremely rare)
}

// _________________ Contact

func (contact Contact) ApiContact() (*api.Contact, error) {
	contactKey, err := contact.ID.ApiKKey()
	if err != nil {
		return nil, err
	}
	return &api.Contact{
		NodeId: contactKey,
		Addr:   contact.Addr,
	}, nil
}

func ToContact(c *api.Contact) (Contact, error) {
	contactKey, err := ToKKey(c.NodeId)
	if err != nil {
		return Contact{}, err
	}
	return Contact{
		ID:   contactKey,
		Addr: c.Addr,
	}, nil
}
