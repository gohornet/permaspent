package main

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/iotaledger/hive.go/backoff"
	"github.com/iotaledger/hive.go/daemon"
)

var initialRetryDelay = time.Second * 1
var maxRetryDelay = time.Second * 10

func spawnCollector(mqttURL string) {
	name := fmt.Sprintf("collector-%s", mqttURL)
	collectorLog := log.Named(name)
	if err := daemon.BackgroundWorker(name, func(shutdownSignal <-chan struct{}) {
		collectorLog.Infof("spawned collector")
		defer collectorLog.Infof("collector stopped")

		connLost := make(chan error, 1)
   		msgChan := make(chan mqtt.Message, 1)

		connectCollector := func(c mqtt.Client) error {
			if token := c.Connect(); token.Wait() && token.Error() != nil {
				return fmt.Errorf("could not connect: %v", token.Error())
			}
			if token := c.Subscribe("spent_address", 0, func(client mqtt.Client, message mqtt.Message) {
				msgChan <- message
			}); token.Wait() && token.Error() != nil {
				return fmt.Errorf("could not subscribe: %w", token.Error())
			}
			return nil
		}

		// mqtt client opts
		opts := mqtt.NewClientOptions().AddBroker(fmt.Sprintf("tcp://%s", mqttURL))
		opts.SetClientID(fmt.Sprintf("client-%d", rand.Int()%1000))
		opts.SetCleanSession(true)
		opts.SetConnectionLostHandler(func(c mqtt.Client, err error) { connLost <- err })

		// connect
		c := mqtt.NewClient(opts)

		// 1, 2, 4, 8 seconds delay
		collectorLog.Info("connecting...")
		backOffPolicy := backoff.ExponentialBackOff(initialRetryDelay, 2).
			With(backoff.MaxInterval(maxRetryDelay))
		if err := backoff.Retry(backOffPolicy, func() error {
			if err := connectCollector(c); err != nil {
				collectorLog.Error(err)
				return err
			}
			return nil
		}); err != nil {
			// note that this can never be hit, since we have no max retries
			collectorLog.Errorf("stopped collector due to: %s", err)
		}
		collectorLog.Info("connected")

		var collected int64
		go func() {
			ticker := time.NewTicker(time.Second * 10)
			defer ticker.Stop()
			for {
				select {
				case <-shutdownSignal:
					return
				case <-ticker.C:
					collectorLog.Infof("collected %d addresses", atomic.LoadInt64(&collected))
				}
			}
		}()

		for {
			select {
			case <-shutdownSignal:
				c.Disconnect(0)
				return
			case err := <-connLost:
				collectorLog.Warnf("lost connection: '%s', trying to reconnect...", err)
				if err := backoff.Retry(backOffPolicy, func() error {
					if err := connectCollector(c); err != nil {
						collectorLog.Error(err)
						return err
					}
					return nil
				}); err != nil {
					// note that this can never be hit, since we have no max retries
					collectorLog.Errorf("stopped collector due to: %s", err)
					return
				}
				collectorLog.Info("connected")
			case msg := <-msgChan:
				atomic.AddInt64(&collected, 1)
				spentAddrCache.Set(string(msg.Payload()), true)
			}
		}
	}, PriorityCollector); err != nil {
		log.Fatal(err)
	}
}
