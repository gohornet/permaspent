package main

import "github.com/iotaledger/hive.go/daemon"

const (
	PriorityCleanup = iota
	PriorityBadgerGC
	PriorityCollector
	PriorityHTTPServer
)

func registerShutdownCleanupHook() {
	if err := daemon.BackgroundWorker("shutdown-cleanup-hook", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		log.Info("flushing cache...")
		// flush caches to database
		spentAddrCache.DeleteAll()
		log.Info("flushing cache...done")
	}, PriorityCleanup); err != nil {
		log.Fatal(err)
	}
}
