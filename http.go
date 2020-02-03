package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

var maxAddressesInRequest int

func spawnHTTPServer() {
	maxAddressesInRequest = config.GetInt("http.maxAddressesInRequest")
	secretKey := config.GetString("http.secretKey")
	if err := daemon.BackgroundWorker("http-server", func(shutdownSignal <-chan struct{}) {
		engine := echo.New()
		engine.HideBanner = true

		// routes
		engine.POST("/", routeSpentAddrQuery)
		engine.Group("/admin", middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
			KeyLookup: "query:api-key",
			Validator: func(key string, c echo.Context) (bool, error) {
				return key == secretKey, nil
			},
		})).GET("/export", routeExport)

		// start listening for requests
		go func() {
			if err := engine.Start(config.GetString("http.bindAddress")); err != nil {
				log.Info("http server shut down")
			}
		}()
		<-shutdownSignal
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Errorf("couldn't shutdown http server cleanly: %s", err)
		}
	}, PriorityHTTPServer); err != nil {
		log.Fatal(err)
	}
}

type request struct {
	Addresses trinary.Hashes `json:"addresses"`
}

type response struct {
	States []bool `json:"states"`
}

func routeSpentAddrQuery(c echo.Context) error {
	req := &request{}
	if err := c.Bind(req); err != nil {
		return c.String(http.StatusBadRequest, "invalid parameters")
	}

	if len(req.Addresses) == 0 {
		return c.String(http.StatusBadRequest, "no addresses supplied")
	}

	if len(req.Addresses) > maxAddressesInRequest {
		return c.String(http.StatusBadRequest, fmt.Sprintf("query limit exceeded, max %d addresses per call", maxAddressesInRequest))
	}

	res := &response{States: make([]bool, len(req.Addresses))}
	for i, addr := range req.Addresses {
		spent, err := wasAddressSpentFrom(addr)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		res.States[i] = spent
	}

	return c.JSON(http.StatusOK, res)
}

// creates an export in the local folder of the spent addresses
func routeExport(c echo.Context) error {
	fileName, err := exportSpentAddresses()
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("error while exporting spent addresses: %s", err))
	}
	return c.String(http.StatusOK, fmt.Sprintf("successfully created spent addresses export file: %s", fileName))
}
