package blockchain

import soon_blockchain "github.com/NetSepio/erebrus/blockchain/solana"

func AllBlockChains(name string) {
	go soon_blockchain.SoonBlockchain(name)
}
