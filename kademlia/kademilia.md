# Kademilia

* https://kelseyc18.github.io/kademlia_vis//basics/1/

* [IPFS](https://research.protocol.ai/publications/ipfs-content-addressed-versioned-p2p-file-system/benet2014.pdf)
* [Kademilia Paper](https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf)
* [Torrent](https://bittorrent.org/beps/bep_0005.html)
* [Distributed Hash Tables with Kademlia](https://codethechange.stanford.edu/guides/guide_kademlia.html#supporting-dynamic-leaves-and-joins)


* [Kademlia: A Peer-to-Peer Information System Based on the XOR Metric](https://www.youtube.com/watch?v=NxhZ_c8YX8E&list=PLL8woMHwr36F-1h7BE92ynHHOE3zebGpA&index=9)
* [Kademlia, Explained](https://www.youtube.com/watch?v=1QdKhNpsj8M)
* [Kademlia - a Distributed Hash Table implementation | Paper Dissection and Deep-dive](https://www.youtube.com/watch?v=_kCHOpINA5g&list=PLsdq-3Z1EPT1rNeq2GXpnivaWINnOaCd0&index=7)
* [Playlist](https://www.youtube.com/playlist?list=PLiYqQVdgdw_sSDkdIZzDRQR9xZlsukIxD)

* [IPFS Kademilia](https://docs.ipfs.tech/concepts/dht/#kademlia)
* [Bitswap : Google Docs](https://docs.google.com/presentation/d/1mbFFGIIKNvboHyLn-k26egOSWkt9nXjlNbxpmCEQfqQ/edit#slide=id.p)

* [Implementing Kademlia in Go](https://medium.com/princeton-systems-course/implementing-kademlia-in-go-65ec9e3c1735)

* https://github.com/libp2p/specs/blob/master/kad-dht/README.md
* https://github.com/libp2p/go-libp2p/blob/master/examples/libp2p-host/host.go
* https://github.com/libp2p/go-libp2p-kad-dht
* https://github.com/nictuku/dht
* https://github.com/pdelong/Kademlia
* https://github.com/prettymuchbryce/kademlia

* [Consistent Hashing with Bounded Loads](https://research.google/blog/consistent-hashing-with-bounded-loads/)
* https://github.com/buraksezer/consistent

###

You’re looking for a Kademlia DHT implementation in Golang. I’ll explain the key components and then provide an implementation outline.

🔹 Key Components of Kademlia in Golang
	1.	Node ID & XOR Distance
	•	Each node has a unique 160-bit ID.
	•	The distance between two nodes is calculated as XOR(NodeA, NodeB).
	2.	k-Buckets for Routing Table
	•	Each node maintains k closest nodes per distance range.
	•	Nodes are stored in Least Recently Used (LRU) order.
	3.	Lookup Algorithm (Recursive Search)
	•	A node finds the k closest peers to a target ID.
	•	Queries are performed in parallel to reduce latency.
	4.	Storing and Retrieving Values
	•	Values are stored at nodes closest to the key.
	•	Nodes must periodically refresh data.
	5.	Network Communication
	•	UDP or TCP for efficient message passing.
	•	Standard RPC messages: PING, STORE, FIND_NODE, FIND_VALUE.

📌 Golang Implementation Outline

We’ll implement:
	1.	Node struct – Holds the ID and network info.
	2.	Routing table (k-buckets) – Manages closest nodes.
	3.	Kademlia DHT – Implements storage, lookup, and messaging.


### K-Bucket

In Kademlia, each node maintains a routing table organized into 160 K-buckets (assuming a 160-bit NodeID, such as SHA-1 hashes). This structure ensures efficient lookups while keeping only a small subset of all nodes in memory.

⸻

Why 160 K-Buckets?

Each K-bucket corresponds to a specific bit prefix length in the XOR distance metric. The bucket index is determined by the number of leading zero bits in the XOR distance between the current node and the target node.

1. Efficient Lookup Using XOR Metric

Kademlia uses XOR distance to determine the “closeness” of nodes. Given two node IDs A and B, their distance is:
\text{distance}(A, B) = A \oplus B
Each bit in the XOR result tells us how similar or different two IDs are. If the first n bits match, the node falls into the nth K-bucket.
	•	K-bucket 0 stores nodes that have a totally different first bit.
	•	K-bucket 159 stores nodes that differ only in the last bit (i.e., very close nodes).

This ensures that a node has contacts spread across the network, which is crucial for efficient lookups.

⸻

2. Balanced Node Distribution

If we used only one bucket, it would be filled with randomly distributed nodes. But in Kademlia, the distribution follows a logarithmic pattern:
	•	Nodes that are far away (larger XOR distance) are rarely contacted but provide broad network coverage.
	•	Nodes that are close (small XOR distance) are frequently contacted for better routing accuracy.

Since nodes join the network dynamically, K-buckets naturally store more distant nodes than close ones.

⸻

3. Logarithmic Lookup Time

When performing a FindNode(targetID), the lookup follows the closest XOR distance first, reducing search time to:
O(\log N)
This is much faster than a naive linear search across all nodes.

⸻

4. Bucket-Based Node Aging

Each K-bucket has a limited size (K, typically 20). When a new node tries to enter a full bucket:
	•	If the oldest node in the bucket is unresponsive (Ping timeout), it gets replaced.
	•	If the oldest node is active, the new node is ignored.

This ensures that nodes with stable connections stay in the routing table, improving network reliability.

⸻

Conclusion
	•	160 K-buckets allow nodes to store contacts at different XOR distances, ensuring efficient lookups.
	•	Logarithmic search (O(log N)) speeds up DHT lookups.
	•	K-bucket aging ensures reliability by keeping long-lived nodes.

Would you like me to help implement iterative node lookups? 🚀
