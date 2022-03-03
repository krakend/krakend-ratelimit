package main

import (
	"flag"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	jujuproxy "github.com/devopsfaith/krakend-ratelimit/v2/juju/proxy"
	jujurouter "github.com/devopsfaith/krakend-ratelimit/v2/juju/router/gin"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
	krakendgin "github.com/luraproject/lura/v2/router/gin"
	"github.com/luraproject/lura/v2/transport/http/client"
	http "github.com/luraproject/lura/v2/transport/http/server"
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

	routerFactory := krakendgin.NewFactory(krakendgin.Config{
		Engine: gin.Default(),
		ProxyFactory: proxy.NewDefaultFactory(
			jujuproxy.BackendFactory(
				logger,
				proxy.CustomHTTPProxyFactory(client.NewHTTPClient),
			),
			logger,
		),
		Middlewares:    []gin.HandlerFunc{},
		Logger:         logger,
		HandlerFactory: jujurouter.HandlerFactory,
		RunServer:      http.RunServer,
	})

	routerFactory.New().Run(serviceConfig)
}
