package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/mattn/go-mastodon"
)

func main() {
	flag.Parse()
	if len(flag.Args()) < 1 {
		panic("need more args")
	}

	app, err := mastodon.RegisterApp(context.Background(), &mastodon.AppConfig{
		Server:     flag.Arg(0),
		ClientName: "hls-await",
		Scopes:     "read write follow",
		Website:    "https://github.com/WIZARDISHUNGRY/hls-await",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("client-id    : %s\n", app.ClientID)
	fmt.Printf("client-secret: %s\n", app.ClientSecret)
}
