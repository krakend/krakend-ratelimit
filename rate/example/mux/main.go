package main

import (
	"flag"
	"log"
	"os"

	rateproxy "github.com/devopsfaith/krakend-ratelimit/rate/proxy"
	raterouter "github.com/devopsfaith/krakend-ratelimit/rate/router/mux"

	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
	"github.com/luraproject/lura/proxy"
	luragorilla "github.com/luraproject/lura/router/gorilla"
	luramux "github.com/luraproject/lura/router/mux"
	"github.com/luraproject/lura/transport/http/client"
	http "github.com/luraproject/lura/transport/http/server"
)

func main() {
	port := flag.Int("p", 0, "Port of the service")
	logLevel := flag.String("l", "ERROR", "Logging level")
	debug := flag.Bool("d", false, "Enable the debug")
	configFile := flag.String("c", "/etc/krakend/configuration.json", "Path to the configuration filename")
	flag.Parse()

	parser := config.NewParser()
	serviceConfig, err := parser.Parse(*configFile)
	if err != nil {
		log.Fatal("ERROR:", err.Error())
	}
	serviceConfig.Debug = serviceConfig.Debug || *debug
	if *port != 0 {
		serviceConfig.Port = *port
	}

	logger, err := logging.NewLogger(*logLevel, os.Stdout, "[KRAKEND]")
	if err != nil {
		log.Fatal("ERROR:", err.Error())
	}

	factoryCfg := luragorilla.DefaultConfig(proxy.DefaultFactory(logger), logger)
	factoryCfg.HandlerFactory = raterouter.HandlerFactory
	factoryCfg.ProxyFactory = proxy.NewDefaultFactory(rateproxy.BackendFactory(proxy.CustomHTTPProxyFactory(client.NewHTTPClient)), logger)
	factoryCfg.RunServer = http.RunServer
	factoryCfg.Logger = logger

	routerFactory := luramux.NewFactory(factoryCfg)
	routerFactory.New().Run(serviceConfig)
}
