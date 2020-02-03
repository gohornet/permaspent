package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-zeromq/zmq4"
	"github.com/iotaledger/hive.go/backoff"
	"github.com/iotaledger/hive.go/daemon"
)

var initialRetryDelay = time.Second * 1
var maxRetryDelay = time.Second * 16

func spawnCollector(zmqURL string) {
	collectLog := log.Named(zmqURL)
	if err := daemon.BackgroundWorker(fmt.Sprintf("collector-%s", zmqURL), func(shutdownSignal <-chan struct{}) {
		collectLog.Infof("spawned collector for %s", zmqURL)
		defer collectLog.Infof("collector for %s stopped", zmqURL)

		// 1, 2, 4, 8, 16 seconds delay
		backOff := backoff.ExponentialBackOff(initialRetryDelay, 2).
			With(backoff.MaxInterval(maxRetryDelay))

		if err := backoff.Retry(backOff, func() error {
			ctx, cancelFunc := context.WithTimeout(context.Background(), time.Duration(2)*time.Second)
			sub := zmq4.NewSub(ctx)
			cancelFunc()

			select {
			case <-shutdownSignal:
				return nil
			default:
			}

			if err := sub.Dial(fmt.Sprintf("tcp://%s", zmqURL)); err != nil {
				collectLog.Errorf("coul not dial: %s", err)
				return err
			}

			if err := sub.SetOption(zmq4.OptionSubscribe, "spent_address"); err != nil {
				collectLog.Errorf("coul not dial: %s", err)
				return err
			}

			for {
				msg, err := sub.Recv()
				if err != nil {
					collectLog.Errorf("could not receive message: %s", err)
					sub.Close()
					return err
				}

				select {
				case <-shutdownSignal:
					break
				default:
				}

				spentAddrCache.Set(msg.String(), true)
			}
		}); err != nil {
			log.Fatalf("stopped collector for %s due to: %s", zmqURL, err)
		}
	}, PriorityCollector); err != nil {
		log.Fatal(err)
	}
}
