// Command preview runs the phone build on your computer: same embedded app,
// same single local account, same one-time sign-in link the Android shell uses.
//
//	cd backend/mobile && go run ./cmd/preview
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	larder "github.com/you/nutrition-app/mobile"
)

func main() {
	dir := flag.String("data", "preview-data", "where to keep the preview's data")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	url, err := larder.Start(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Open this link within 2 minutes (it works once):")
	fmt.Println(url)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	_ = larder.Stop()
}
