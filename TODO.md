- Use latest go version (1.21.0)
- Check out Bitcoin tools:
  - [Bitcoin UTXO Dump](https://github.com/in3rsha/bitcoin-utxo-dump)
- Use spf13/cobra for command-line parsing
  - bash/fish/... completions
- Use latest BadgerDB (dgraph-io/badger) as key-value store for the blockchain
- Implement everything to make initial sync:
  - command-line parsing
  - blockchain (memory and persistent DB)
  - sync protocol (p2p network)
