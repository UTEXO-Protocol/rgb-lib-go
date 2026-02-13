// Test: key generation, wallet from .env, go online.
// Keys and network are read from .env.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	rgb_lib "github.com/UTEXO-Protocol/rgb-lib-go"
	"github.com/joho/godotenv"
)

const defaultDataDir = "./data"

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBitcoinNetwork(s string) rgb_lib.BitcoinNetwork {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "mainnet":
		return rgb_lib.BitcoinNetworkMainnet
	case "testnet":
		return rgb_lib.BitcoinNetworkTestnet
	case "signet":
		return rgb_lib.BitcoinNetworkSignet
	case "regtest":
		return rgb_lib.BitcoinNetworkRegtest
	default:
		return rgb_lib.BitcoinNetworkSignet
	}
}

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load(filepath.Join(".", ".env"))

	networkStr := getEnv("BITCOIN_NETWORK", "signet")
	network := parseBitcoinNetwork(networkStr)

	mnemonic := getEnv("MNEMONIC", "")
	accountXpubVanilla := getEnv("ACCOUNT_XPUB_VANILLA", "")
	accountXpubColored := getEnv("ACCOUNT_XPUB_COLORED", "")
	masterFingerprint := getEnv("MASTER_FINGERPRINT", "")
	dataDir := getEnv("DATA_DIR", defaultDataDir)
	indexerURL := getEnv("INDEXER_URL", "")

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			log.Fatalf("create data dir: %v", err)
		}
		log.Printf("created data dir: %s", dataDir)
	}

	keys := rgb_lib.GenerateKeys(network)
	fmt.Println("generate_keys")
	fmt.Printf("  network=%s\n", networkStr)
	fmt.Printf("  mnemonic=%s\n", keys.Mnemonic)
	fmt.Printf("  xpub=%s\n", keys.Xpub)
	fmt.Printf("  account_xpub_vanilla=%s\n", keys.AccountXpubVanilla)
	fmt.Printf("  account_xpub_colored=%s\n", keys.AccountXpubColored)
	fmt.Printf("  master_fingerprint=%s\n", keys.MasterFingerprint)

	keychain := uint8(1)
	walletData := rgb_lib.WalletData{
		DataDir:               dataDir,
		BitcoinNetwork:        network,
		DatabaseType:          rgb_lib.DatabaseTypeSqlite,
		MaxAllocationsPerUtxo: 1,
		AccountXpubVanilla:    accountXpubVanilla,
		AccountXpubColored:    accountXpubColored,
		Mnemonic:              &mnemonic,
		MasterFingerprint:     masterFingerprint,
		VanillaKeychain:       &keychain,
		SupportedSchemas: []rgb_lib.AssetSchema{
			rgb_lib.AssetSchemaNia,
			rgb_lib.AssetSchemaUda,
			rgb_lib.AssetSchemaCfa,
			rgb_lib.AssetSchemaIfa,
		},
	}

	wallet, err := rgb_lib.NewWallet(walletData)
	if err != nil {
		log.Fatalf("new wallet: %v", err)
	}
	fmt.Printf("\nwallet_data_dir=%s\n", wallet.GetWalletDir())

	online, err := wallet.GoOnline(false, indexerURL)
	if err != nil {
		log.Fatalf("go online: %v", err)
	}
	fmt.Printf("online_id=%d\n", online.Id)

	address, err := wallet.GetAddress()
	if err != nil {
		log.Fatalf("get address: %v", err)
	}
	fmt.Printf("address=%s\n", address)

	btcBalance, err := wallet.GetBtcBalance(&online, false)
	if err != nil {
		log.Fatalf("get btc balance: %v", err)
	}
	fmt.Printf("btc_balance_vanilla_settled=%d spendable=%d\n", btcBalance.Vanilla.Settled, btcBalance.Vanilla.Spendable)
	fmt.Printf("btc_balance_colored_settled=%d spendable=%d\n", btcBalance.Colored.Settled, btcBalance.Colored.Spendable)

	fmt.Println("\nok")
}
