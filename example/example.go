package main

import (
	"fmt"

	rgb_lib "github.com/RGB-OS/rgb-lib-go"
)

func main() {
	fmt.Println("Bitcoin Network:", rgb_lib.BitcoinNetworkRegtest)
	mnemonic := "mushroom pilot innocent trouble position hard forward sudden desert throw front found"
	keychain := uint8(1)
	walletData := rgb_lib.WalletData{
		DataDir:               "./data",
		BitcoinNetwork:        rgb_lib.BitcoinNetworkRegtest,                                                                                     // or .Mainnet, .Testnet
		DatabaseType:          rgb_lib.DatabaseTypeSqlite,                                                                                        // or .Postgres
		AccountXpubVanilla:    "tpubDDYnz6z5aG8k73T9Vg2Lmj5F1R4tdwPFjoBgeccJrQzDr44kgwdjqc3ghFpJu86Ksyj85aobCb1ffB6i4FrsnRSRm7RKatwdJ9HdK5zRrRZ", // your extended pubkey string
		AccountXpubColored:    "tpubDDYnz6z5aG8k73T9Vg2Lmj5F1R4tdwPFjoBgeccJrQzDr44kgwdjqc3ghFpJu86Ksyj85aobCb1ffB6i4FrsnRSRm7RKatwdJ9HdK5zRrRZ", // your extended pubkey string
		Mnemonic:              &mnemonic,
		MaxAllocationsPerUtxo: 1,
		VanillaKeychain:       &keychain, // use appropriate key string
	}

	wallet, err := rgb_lib.NewWallet(walletData)
	if err != nil {
		panic(fmt.Sprintf("Wallet error: %v", err))
	}

	fmt.Println("Preparing online")
	online, err := wallet.GoOnline(false, "tcp://electrum.rgbtools.org:50041")
	if err != nil {
		panic(fmt.Sprintf("Go online failed: %v", err))
	}
	address, err := wallet.GetAddress()
	if err != nil {
		panic(fmt.Sprintf("fail get address error: %v", err))
	}
	fmt.Println("UTXO psbt:", address)

	num := uint8(1)
	size := uint32(1000)
	// Create PSBT
	psbt, err := wallet.CreateUtxosBegin(online, false, &num, &size, 1, false)
	if err != nil {
		panic(fmt.Sprintf("Create_utxos_begin error: %v", err))
	}

	fmt.Println("UTXO psbt:", psbt)
	signed, err := wallet.SignPsbt("cHNidP8BAIkCAAAAASj0AJOZH+xAY/dHSEu+SyOHB1EViKboxP56CjBZ4NeqAAAAAAD9////Ap2cBwAAAAAAIlEgv6297p82nVp0nPyXsGQcqJhHwlfT7pXevdfYea0z+9boAwAAAAAAACJRIBffAUBPy4qBY1KhwWuWmUI1Er/mgBG2kEcBAvCwSG7rJ3kEAAABASsgoQcAAAAAACJRIAKSBzNr16YuyXiNYwEpiH3YO13I/e6wL75kTcJ28iBNIRbP6MxfGn6eVbEEvFdKgmN30sgC9IrtejxyYmiNvKAlAQ0Ahla7TwEAAAAYAAAAARcgz+jMXxp+nlWxBLxXSoJjd9LIAvSK7Xo8cmJojbygJQEAAQUgKcxpjWH7BXW8zA9/EENFXs9Xv3qS72khD/I1Z/nBU0whBynMaY1h+wV1vMwPfxBDRV7PV796ku9pIQ/yNWf5wVNMDQCGVrtPAQAAAAAAAAAAAQUgZEtyBpMbMWU1k2RSj5O8GukOAzxe7rhAau17aY5DoQohB2RLcgaTGzFlNZNkUo+TvBrpDgM8Xu64QGrte2mOQ6EKDQCGVrtPCQAAAAAAAAAA")
	if err != nil {
		panic(fmt.Sprintf("Sign error: %v", err))
	}
	fmt.Println("Signed PSBT:", signed)
}
