package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"ply/internal/core"
)

func main() {
	url := flag.String("url", "", "ключ")
	split := flag.Bool("split", true, "РФ напрямую")
	out := flag.String("out", "", "config.json")
	fd := flag.Int("fd", -1, "tun fd (Android)")
	mtu := flag.Int("mtu", 0, "MTU 1280, 1400 или 1500")
	node := flag.String("node", "", "id узла")
	bypass := flag.String("bypass", "", "процессы или пакеты мимо VPN")
	list := flag.Bool("list", false, "json-список узлов")
	pull := flag.Bool("pull", false, "обновить подписку и напечатать новый адрес")
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
		cards, _, _, err := core.ParseCatalog(src)
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
	if *pull {
		low := strings.ToLower(strings.TrimSpace(src))
		if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
			fmt.Fprintln(os.Stderr, "обновляется только ссылка подписки")
			os.Exit(1)
		}
		cards, err := core.RefreshCatalog(src)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		u := core.ReadURL()
		if u == "" {
			u = src
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{"url": u, "nodes": cards}); err != nil {
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
	b, err := core.RenderXrayTun(n, core.LocalPort, *split, *fd, *mtu, []string{*bypass})
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
