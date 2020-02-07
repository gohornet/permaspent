package main

import (
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/database"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/parameter"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	AppVersion = "0.1.0"
	config     = viper.New()
	db         database.Database
	log        *logger.Logger
)

func init() {
	flag.StringSlice("nodes", []string{"main.manapotion.io:5556"}, "the nodes from which to collect spent addresses from")
	flag.String("database", "spent_addresses", "the name of the database folder")
	flag.String("importFile", "spent_addresses.bin", "the name of the import file containing the initial spent addresses")
	flag.Int("cache.size", 100000, "the size of the cache containing spent addresses")
	flag.Int("cache.batchEvictionSize", 10000, "the amount of addresses to evict together from the cache")
	flag.String("http.bindAddress", "0.0.0.0:8081", "the bind address for the HTTP calls")
	flag.Int("http.maxAddressesInRequest", 1000, "the maximum amount of spent addresses within a single HTTP call")
	flag.String("http.secretKey", "1337", "the secret key used to authenticate for certain HTTP calls")
}

func main() {
	var err error

	// init config
	must(parameter.LoadConfigFile(config, ".", "config", true, false))

	// init common
	must(logger.InitGlobalLogger(config))
	log = logger.NewLogger("App")
	initCache()

	// check whether we're running with a clean database and if so, require a spent-addresses import file
	importFileName := config.GetString("importFile")
	isFirstRun := firstRun()
	if isFirstRun {
		if _, err := os.Stat(importFileName); err != nil {
			if os.IsNotExist(err) {
				log.Fatalf("you must supply an import file '%s', if you're running the program for the first time", importFileName)
			}
			log.Fatalf("unable to check existence of import file '%s': %s", importFileName, err)
		}
	}

	dbDir := config.GetString("database")
	badgerDB, err := database.CreateDB(dbDir, getDatabaseOpts(dbDir))
	if err != nil {
		log.Fatalf("unable to instantiate database: %s", err)
	}

	// init database
	db, err = database.Get(0, badgerDB)
	must(err)
	// make sure to close the database when the program exits
	defer func() {
		log.Info("closing database...")
		if err := badgerDB.Close(); err != nil {
			log.Error(err)
		}
		log.Info("closing database...done")
	}()
	spawnBadgerGC()

	// do import routine
	if isFirstRun {
		importSpentAddresses(importFileName)
	}

	// register shutdown handler
	registerShutdownCleanupHook()

	// spawn collectors
	for _, node := range config.GetStringSlice("nodes") {
		spawnCollector(node)
	}

	// spawn http server
	spawnHTTPServer()

	// execute
	daemon.Start()

	shutdownSignal := make(chan os.Signal)
	signal.Notify(shutdownSignal, syscall.SIGTERM)
	signal.Notify(shutdownSignal, syscall.SIGINT)
	<-shutdownSignal

	// shutdown
	daemon.ShutdownAndWait()
}

func firstRun() bool {
	fileInfos, err := ioutil.ReadDir(config.GetString("database"))
	if err != nil {
		// panic on other errors, for example permission related
		if !os.IsNotExist(err) {
			panic(err)
		}
		return true
	}
	if len(fileInfos) == 0 {
		return true
	}
	return false
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
