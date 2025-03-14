package kademlia

import (
	"bytes"
	"testing"

	"github.com/codeharik/kademlia/api"
)

func TestKKeyEncodingDecoding(t *testing.T) {
	keyTests := []struct {
		key   []byte
		error bool
	}{
		{
			key:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			error: false,
		},
		{
			key:   []byte{1, 2, 3, 4, 5, 6, 7},
			error: true,
		},
	}

	for _, test := range keyTests {
		key := api.KKey{Key: test.key}

		nodeId, err := ToKKey(&key)
		if (err != nil) != test.error {
			t.Errorf("Decoding failed: %v", err)
		}

		kkey, err := nodeId.ApiKKey()
		if (err != nil || bytes.Compare(kkey.Key, test.key) != 0) != test.error {
			t.Errorf("Decoded NodeID does not match original. Got %v, expected %v", nodeId, test.key)
		}
	}
}
