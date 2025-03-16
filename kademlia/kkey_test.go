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
			key:   []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18},
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
			t.Errorf("%v, Decoded NodeID does not match original. Got %v, expected %v", err, nodeId, test.key)
		}
	}
}

func TestXor(t *testing.T) {
	id1 := KKey{0xFF, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	id2 := KKey{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	expected := KKey{0xFF, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	dist := id1.Xor(id2)
	if dist != expected {
		t.Errorf("Expected distance %v, got %v", expected, dist)
	}
}

func TestLeadingZeros(t *testing.T) {
	tests := []struct {
		input    KKey
		expected int
	}{
		{
			KKey{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			144,
		},
		{
			KKey{0b10000000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			0,
		},
		{
			KKey{0b00100000, 0b01000000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			2,
		},
		{
			KKey{0b00000000, 0b01000000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			9,
		},
	}

	for _, test := range tests {
		result := test.input.LeadingZeros()
		if result != test.expected {
			t.Errorf("LeadingZeros() for %v = %d, want %d", test.input, result, test.expected)
		}
	}
}
