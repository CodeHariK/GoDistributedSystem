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
	"math"
	"math/big"
	"math/bits"
	"net/http"
	"os"

	"github.com/cespare/xxhash/v2"
	"github.com/codeharik/kademlia/api"
	"github.com/ethereum/go-ethereum/crypto"
)

// _________________ KKey

func NewKKey(domain, id string, content []byte, domainKey, idKey *ecdsa.PrivateKey) (KKey, error) {
	var kkey KKey

	{
		domainHash, err := HashKey([]byte(domain), domainKey)
		if err != nil {
			return KKey{}, err
		}
		copy(kkey[0:8], domainHash[:])
	}

	{
		lat, lon, err := GetLocationFromIP()
		if err != nil {
			return KKey{}, err
		}
		latByte := uint8(math.Round(float64(lat+90) / 180 * 255))
		lonByte := uint8(math.Round(float64(lon+180) / 360 * 255))
		kkey[8] = latByte
		kkey[9] = lonByte
	}

	if len(content) != 0 {
		ss := hash(content)
		copy(kkey[10:], ss[:])
	} else {
		idPubKey, err := ECDSAMarshal(&idKey.PublicKey)
		if err != nil {
			return KKey{}, err
		}
		idPubKey = append(idPubKey, []byte(id)...)
		idHash, err := HashKey(idPubKey, domainKey)
		if err != nil {
			return KKey{}, err
		}
		copy(kkey[10:], idHash[:])
	}

	return kkey, nil
}

func (id KKey) BitString() string {
	return fmt.Sprintf("%08b-%08b-%08b", id[:8], id[8:10], id[10:])
}

func (id KKey) HexString() string {
	key := hex.EncodeToString(id[:])
	return fmt.Sprintf("%s-%s-%s", key[:16], key[16:20], key[20:])
}

func (id KKey) ApiKKey() (*api.KKey, error) {
	if len(id) != CONST_KKEY_BYTE_COUNT {
		return nil, ERR_INVALID_KEY
	}
	key := api.KKey{Key: id[:]}
	return &key, nil
}

func ToKKey(k *api.KKey) (KKey, error) {
	if len(k.Key) != CONST_KKEY_BYTE_COUNT {
		return KKey{}, ERR_INVALID_KEY
	}
	var id KKey
	copy(id[:], k.Key)
	return id, nil
}

// _________________ Distance

// Xor calculates XOR distance between two NodeIDs.
func (id KKey) Xor(other KKey) KKey {
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
	return CONST_KKEY_BIT_COUNT // All zero case (extremely rare)
}

func (id KKey) Distance(other KKey) int {
	return id.Xor(other).LeadingZeros()
}

// _________________ Contact

func (contact Contact) ApiContact() (*api.Contact, error) {
	contactKey, err := contact.key.ApiKKey()
	if err != nil {
		return nil, err
	}

	domainKey, drr := ECDSAMarshal(contact.domainKey)
	idKey, irr := ECDSAMarshal(contact.idKey)
	if drr != nil || irr != nil {
		return nil, errors.Join(drr, irr)
	}

	return &api.Contact{
		Key:  contactKey,
		Addr: contact.Addr,

		Domain:    contact.domain,
		DomainKey: domainKey,
		Id:        contact.id,
		IdKey:     idKey,
	}, nil
}

func ToContact(c *api.Contact) (Contact, error) {
	if c == nil {
		return Contact{}, errors.New("Null Contact")
	}

	contactKey, err := ToKKey(c.Key)
	if err != nil {
		return Contact{}, err
	}

	domainkey, domainerr := BytesToECDSAPublicKey(c.DomainKey)
	idkey, iderr := BytesToECDSAPublicKey(c.IdKey)
	if domainerr != nil || iderr != nil {
		return Contact{}, errors.Join(domainerr, iderr)
	}

	return Contact{
		key:       contactKey,
		Addr:      c.Addr,
		domain:    c.Domain,
		id:        c.Id,
		domainKey: domainkey,
		idKey:     idkey,
	}, nil
}

func (c *Node) ToContact() (Contact, error) {
	return Contact{
		key:       c.Key,
		Addr:      c.Addr,
		domain:    c.domain,
		id:        c.id,
		domainKey: &c.domainKey.PublicKey,
		idKey:     &c.idKey.PublicKey,
	}, nil
}

// _________________ Api

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

// _________________ Encrypt

// ECDSAUnmarshal converts a compressed public key (33 bytes) into *ecdsa.PublicKey.
func ECDSAUnmarshal(pubKey []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), pubKey)
	if x == nil || y == nil {
		return nil, errors.New("invalid compressed public key")
	}

	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}, nil
}

// ECDSAMarshal converts an *ecdsa.PublicKey into a compressed 33-byte format.
func ECDSAMarshal(pubKey *ecdsa.PublicKey) ([]byte, error) {
	if pubKey == nil || pubKey.X == nil || pubKey.Y == nil {
		return nil, errors.New("invalid ECDSA public key")
	}
	return elliptic.MarshalCompressed(elliptic.P256(), pubKey.X, pubKey.Y), nil
}

// BytesToECDSAPublicKey converts a compressed or uncompressed []byte public key to an ecdsa.PublicKey.
func BytesToECDSAPublicKey(pubKeyBytes []byte) (*ecdsa.PublicKey, error) {
	curve := elliptic.P256()

	switch len(pubKeyBytes) {
	case 33: // Compressed format
		x, y := elliptic.UnmarshalCompressed(curve, pubKeyBytes)
		if x == nil || y == nil {
			return nil, fmt.Errorf("invalid compressed public key: %v", pubKeyBytes)
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil

	case 65: // Uncompressed format (0x04 + X + Y)
		x, y := elliptic.Unmarshal(curve, pubKeyBytes)
		if x == nil || y == nil {
			return nil, fmt.Errorf("invalid uncompressed public key: %v", pubKeyBytes)
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil

	case 64: // Raw `X || Y` format (like your function originally expected)
		x := new(big.Int).SetBytes(pubKeyBytes[:32])
		y := new(big.Int).SetBytes(pubKeyBytes[32:])
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil

	default:
		return nil, fmt.Errorf("invalid public key length: %d : %v", len(pubKeyBytes), pubKeyBytes)
	}
}

func SignMessage(privKey *ecdsa.PrivateKey, message []byte) (sig []byte, err error) {
	hash := sha256.Sum256(message)

	return crypto.Sign(hash[:], privKey)
}

func VerifySignature(pubkey *ecdsa.PublicKey, digestHash []byte, signature []byte) bool {
	mkey, err := ECDSAMarshal(pubkey)
	if err != nil {
		return false
	}
	return crypto.VerifySignature(mkey, digestHash, signature)
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

func hash(r []byte) [8]byte {
	dataHash := xxhash.Sum64(r)

	return [8]byte{
		byte(dataHash >> 56), byte(dataHash >> 48), byte(dataHash >> 40), byte(dataHash >> 32),
		byte(dataHash >> 24), byte(dataHash >> 16), byte(dataHash >> 8), byte(dataHash),
	}
}

func HashKey(str []byte, key *ecdsa.PrivateKey) ([8]byte, error) {
	r, err := SignMessage(key, str)
	if err != nil {
		return [8]byte{}, err
	}
	return hash(r), nil
}
