package main

import (
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/database"
	"github.com/iotaledger/hive.go/lru_cache"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/iota.go/trinary"
)

var spentAddrCache *lru_cache.LRUCache

func initCache() {
	spentAddrCache = lru_cache.NewLRUCache(config.GetInt("cache.size"), &lru_cache.LRUCacheOptions{
		EvictionCallback:  onEvictSpentAddresses,
		EvictionBatchSize: config.GetUint64("cache.batchEvictionSize"),
	})
}

func getDatabaseOpts(dbDir string) badger.Options {
	opts := badger.DefaultOptions(dbDir)
	opts.CompactL0OnClose = false
	opts.KeepL0InMemory = false
	opts.VerifyValueChecksum = false
	opts.ZSTDCompressionLevel = 1
	opts.Compression = options.None
	opts.MaxCacheSize = 50000000
	opts.Truncate = false
	opts.EventLogging = false
	opts.Logger = nil
	return opts
}

func spawnBadgerGC() {
	if err := daemon.BackgroundWorker("badger-db-gc", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(func() {
			db := database.GetBadgerInstance()
			log.Info("running BadgerDB garbage collection")
			var err error
			for err == nil {
				err = db.RunValueLogGC(0.7)
			}
		}, 5*time.Minute, shutdownSignal)
	}, PriorityBadgerGC); err != nil {
		log.Fatal(err)
	}
}

func wasAddressSpentFrom(address trinary.Hash) (result bool, err error) {
	if spentAddrCache.Contains(address) {
		result = true
	} else {
		result, err = spentDatabaseContainsAddress(address)
	}
	return
}

func onEvictSpentAddresses(keys interface{}, _ interface{}) {
	keyT := keys.([]interface{})

	var addresses []trinary.Hash
	for _, obj := range keyT {
		addresses = append(addresses, obj.(trinary.Hash))
	}

	if err := storeSpentAddressesInDatabase(addresses); err != nil {
		panic(err)
	}
}

func databaseKeyForAddress(address trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(address)
}

func spentDatabaseContainsAddress(address trinary.Hash) (bool, error) {
	if contains, err := db.Contains(databaseKeyForAddress(address)); err != nil {
		return contains, fmt.Errorf("failed to check if the address exists in the spent addresses database: %w", err)
	} else {
		return contains, nil
	}
}

func storeSpentAddressesInDatabase(spent []trinary.Hash) error {

	var entries []database.Entry

	for _, address := range spent {
		key := databaseKeyForAddress(address)

		entries = append(entries, database.Entry{
			Key:   key,
			Value: []byte{},
		})
	}

	// Now batch insert/delete all entries
	if err := db.Apply(entries, []database.Key{}); err != nil {
		return fmt.Errorf("failed to mark addresses as spent: %w", err)
	}

	return nil
}
