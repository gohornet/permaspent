package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/database"
	"github.com/iotaledger/iota.go/trinary"
)

func importSpentAddresses(importFileName string) {
	f, err := os.OpenFile(importFileName, os.O_RDONLY, 0660)
	if err != nil {
		log.Fatalf("unable to open import file '%s': %s", importFileName, err)
	}

	var amount int32
	must(binary.Read(f, binary.LittleEndian, &amount))

	log.Infof("will import %d spent addresses from '%s'", amount, importFileName)
	start := time.Now()
	buf := make([]byte, 49)
	for i := 0; i < int(amount); i++ {
		_, err := f.Read(buf)
		must(err)
		// write address into cache (this automatically creates batched writes in the db)
		spentAddrCache.Set(trinary.MustBytesToTrytes(buf)[:81], true)
		if i%1000 == 0 {
			fmt.Printf("importing %d%%...\r", int(float64(i+1)/float64(amount)*100))
		}
	}
	fmt.Println()
	log.Infof("imported %d spent addresses from '%s' in %v", amount, importFileName, time.Now().Sub(start))
}

var exportingMu = sync.Mutex{}

func exportSpentAddresses() (string, error) {
	exportingMu.Lock()
	defer exportingMu.Unlock()

	log.Info("flushing cache for creating a new export file...")

	// flush the cache first
	spentAddrCache.DeleteAll()

	fileName := fmt.Sprintf("spent_addresses_%d.bin", time.Now().Unix())
	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		return "", fmt.Errorf("unable to create export file '%s': %s", fileName, err)
	}
	defer f.Close()
	log.Infof("creating export file '%s'", fileName)

	var count int32
	var buf bytes.Buffer
	if err := db.StreamForEachKeyOnly(func(entry database.KeyOnlyEntry) error {
		buf.Write(entry.Key)
		count++
		return nil
	}); err != nil {
		return "", fmt.Errorf("unable to stream over entries for export file '%s': %s", fileName, err)
	}

	if err := binary.Write(f, binary.LittleEndian, count); err != nil {
		return "", fmt.Errorf("unable to write address count in export file '%s': %s", fileName, err)
	}

	if _, err := f.Write(buf.Bytes()); err != nil {
		return "", fmt.Errorf("couldn't write all data into the export file '%s': %s", fileName, err)
	}

	return fileName, nil
}
