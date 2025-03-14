package kademlia

func NewKeyValueStore() *KeyValueStore {
	return &KeyValueStore{data: make(map[KKey][]byte)}
}

func (store *KeyValueStore) Store(key KKey, value []byte) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.data[key] = value
}

func (store *KeyValueStore) Get(key KKey) ([]byte, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	val, exists := store.data[key]
	return val, exists
}
