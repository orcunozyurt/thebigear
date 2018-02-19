package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/thebigear/utils"
)

func mmain() {

	fmt.Println("Connecting my ear to Twitter...")

	consumerKey := utils.GetEnvOrDefault("TWITTER_CONSUMER_KEY", "")
	consumerSecret := utils.GetEnvOrDefault("TWITTER_CONSUMER_SECRET", "")

	accessToken := utils.GetEnvOrDefault("TWITTER_ACCESS_TOKEN", "")
	accessSecret := utils.GetEnvOrDefault("TWITTER_ACCESS_SECRET", "")

	fmt.Println(consumerKey, consumerSecret, accessToken, accessSecret)

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	// OAuth1 http.Client will automatically authorize Requests
	httpClient := config.Client(oauth1.NoContext, token)

	// Twitter Client
	client := twitter.NewClient(httpClient)
	// Convenience Demux demultiplexed stream messages
	demux := twitter.NewSwitchDemux()
	demux.Tweet = func(tweet *twitter.Tweet) {
		fmt.Println(tweet.Text)
	}

	fmt.Println("Starting Stream...")
	// FILTER
	filterparams := &twitter.StreamFilterParams{
		Track:         []string{"bitcoin"},
		StallWarnings: twitter.Bool(true),
	}
	stream, err := client.Streams.Filter(filterparams)
	if err != nil {
		fmt.Println(err)
	}

	// Receive messages until stopped or stream quits
	for message := range stream.Messages {
		fmt.Println(message)
	}
	// Wait for SIGINT and SIGTERM (HIT CTRL-C)
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch)

	fmt.Println("Stopping Stream...")
	stream.Stop()

}
