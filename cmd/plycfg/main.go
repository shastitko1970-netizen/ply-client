package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"ply/internal/core"
)

func main() {
	url := flag.String("url", "", "ключ")
	split := flag.Bool("split", true, "РФ напрямую")
	out := flag.String("out", "", "config.json")
	fd := flag.Int("fd", -1, "tun fd (Android)")
	mtu := flag.Int("mtu", 0, "MTU 1280, 1400 или 1500")
	node := flag.String("node", "", "id узла")
	list := flag.Bool("list", false, "json-список узлов")
	flag.Parse()
	src := *url
	if src == "" && flag.NArg() > 0 {
		src = flag.Arg(0)
	}
	if src == "" {
		fmt.Fprintln(os.Stderr, "нужен ключ")
		os.Exit(2)
	}
	if *list {
		cards, _, err := core.ParseCatalog(src)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cards); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}
	n, _, _, err := core.ResolveChosen(src, *node)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	b, err := core.RenderXrayTun(n, core.LocalPort, *split, *fd, *mtu)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if *out == "" {
		os.Stdout.Write(b)
		return
	}
	if err := os.WriteFile(*out, b, 0600); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
