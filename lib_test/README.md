# lib_test

Small test for **rgb-lib-go**: key generation, wallet from `.env`, and go online.

This script is set up for the current working binding version (see `go.mod`). To test with another version:

- **Another GitHub release**: change the `rgb-lib-go` version in `go.mod` (e.g. `v0.3.6` → your tag).
- **Local binding**: add a `replace` directive in `go.mod`, e.g.:
  ```go
  replace github.com/UTEXO-Protocol/rgb-lib-go => /path/to/your/local/rgb-lib-go
  ```
  From this repo, use `replace ... => ../` to test the parent module.

## Setup

1. **Env file**  
   Copy `.env.example` to `.env` and fill in your keys and settings:
   ```bash
   cp .env.example .env
   ```
   Edit `.env`: set `MNEMONIC`, `ACCOUNT_XPUB_VANILLA`, `ACCOUNT_XPUB_COLORED`, `MASTER_FINGERPRINT`, `BITCOIN_NETWORK`, and `INDEXER_URL`.

2. **Data directory**  
   The app creates `DATA_DIR` if it does not exist (default: `./data`). No need to create it by hand.

3. **Run**
   ```bash
   make tidy    # fetch deps, tidy go.mod
   make run     # tidy + run the test (or: make)
   ```

## Env vars (.env)

| Variable               | Description                                  |
|------------------------|----------------------------------------------|
| `MNEMONIC`             | BIP39 mnemonic                               |
| `ACCOUNT_XPUB_VANILLA` | Account xpub (vanilla)                        |
| `ACCOUNT_XPUB_COLORED` | Account xpub (colored)                        |
| `MASTER_FINGERPRINT`   | Master key fingerprint                       |
| `BITCOIN_NETWORK`      | `mainnet` \| `testnet` \| `signet` \| `regtest` |
| `DATA_DIR`             | Wallet data directory (default: `./data`)    |
| `INDEXER_URL`          | Electrum `tcp://host:port` or Esplora `https://...` |
