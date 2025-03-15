package kademlia

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"math/bits"
	"net/http"
	"os"

	"github.com/codeharik/kademlia/api"

	"github.com/cespare/xxhash/v2"
)

// _________________ KKey

// SignMessage signs a message using ECDSA.
func SignMessage(privKey *ecdsa.PrivateKey, message []byte) ([]byte, []byte) {
	hash := sha256.Sum256(message)
	r, s, _ := ecdsa.Sign(rand.Reader, privKey, hash[:])
	return r.Bytes(), s.Bytes() // Return signature parts
}

// VerifyMessage verifies a signed message.
func VerifyMessage(pubKey *ecdsa.PublicKey, message, rBytes, sBytes []byte) bool {
	hash := sha256.Sum256(message)
	r := new(big.Int).SetBytes(rBytes)
	s := new(big.Int).SetBytes(sBytes)
	return ecdsa.Verify(pubKey, hash[:], r, s)
}

// LoadKeyPair loads or generates an ECDSA key pair.
func LoadKeyPair(keyPath string) (*ecdsa.PrivateKey, error) {
	// Try loading existing key
	if keyData, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(keyData)
		if block == nil {
			log.Println("Corrupt PEM file detected, regenerating key pair")
		} else {
			privKey, err := x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			return privKey, nil
		}
	}

	// Generate a new key if not found
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	// Encode and save the private key
	privBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return nil, err
	}
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	if err := os.WriteFile(keyPath, keyPem, 0o600); err != nil {
		return nil, err
	}

	return privKey, nil
}

func NewKKey(topic string, content []byte, publicKey *ecdsa.PublicKey) (KKey, error) {
	var kkey KKey

	lat, lon, err := GetLocationFromIP()
	if err != nil {
		return KKey{}, err
	}

	latByte := byte((lat + 90) / 180 * 255)  // Normalize -90 to 90 into 0 to 255
	lonByte := byte((lon + 180) / 360 * 255) // Normalize -180 to 180 into 0 to 255

	kkey[0] = latByte
	kkey[1] = lonByte

	topicHash := xxhash.Sum64([]byte(topic))
	copy(kkey[2:10], []byte{
		byte(topicHash >> 56), byte(topicHash >> 48), byte(topicHash >> 40), byte(topicHash >> 32),
		byte(topicHash >> 24), byte(topicHash >> 16), byte(topicHash >> 8), byte(topicHash),
	})

	pubKeyBytes := append(publicKey.X.Bytes(), publicKey.Y.Bytes()...) // Convert PublicKey to bytes

	hashKey := pubKeyBytes
	if len(content) != 0 {
		hashKey = content
	}
	contentHash := xxhash.Sum64(hashKey)
	copy(kkey[10:], []byte{
		byte(contentHash >> 56), byte(contentHash >> 48), byte(contentHash >> 40), byte(contentHash >> 32),
		byte(contentHash >> 24), byte(contentHash >> 16), byte(contentHash >> 8), byte(contentHash),
	})

	return kkey, nil
}

func (id KKey) HexString() string {
	key := hex.EncodeToString(id[:])
	return fmt.Sprintf("%s-%s-%s", key[:4], key[4:20], key[20:])
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
		NodeId:    contactKey,
		Addr:      contact.Addr,
		PublicKey: contact.PublicKey,
	}, nil
}

func ToContact(c *api.Contact) (Contact, error) {
	contactKey, err := ToKKey(c.NodeId)
	if err != nil {
		return Contact{}, err
	}
	return Contact{
		ID:        contactKey,
		Addr:      c.Addr,
		PublicKey: c.PublicKey,
	}, nil
}

//

func GetLocationFromIP() (float32, float32, error) {
	resp, err := http.Get("https://ipinfo.io/json")
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var data struct {
		Loc string `json:"loc"` // "lat,lon" format
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0, err
	}

	var lat, lon float32
	fmt.Sscanf(data.Loc, "%f,%f", &lat, &lon)
	return lat, lon, nil
}
