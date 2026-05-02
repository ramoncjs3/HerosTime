package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"text/tabwriter"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/game"
	"oldbeggar-refactor/internal/httputil"
)

func main() {
	configPath := flag.String("config", "configs/config.local.yaml", "path to config file")
	all := flag.Bool("all", false, "include disabled variants")
	raw := flag.Bool("raw", false, "print raw server names before rule mapping")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	variants := cfg.EnabledVariants()
	if *all {
		variants = cfg.Variants
	}

	resolver := game.NewServerResolver(httputil.New(cfg.HTTP))
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if *raw {
		fmt.Fprintln(writer, "VARIANT\tNAME")
		for _, variant := range variants {
			names, err := resolver.RawNames(context.Background(), variant)
			if err != nil {
				log.Printf("raw %s: %v", variant.Name, err)
				continue
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Fprintf(writer, "%s\t%s\n", variant.Name, name)
			}
		}
		if err := writer.Flush(); err != nil {
			log.Fatal(err)
		}
		return
	}

	fmt.Fprintln(writer, "VARIANT\tCODE\tNAME")
	for _, variant := range variants {
		endpoints, err := resolver.Resolve(context.Background(), variant)
		if err != nil {
			log.Printf("resolve %s: %v", variant.Name, err)
			continue
		}
		codes := make([]string, 0, len(endpoints))
		for code := range endpoints {
			codes = append(codes, code)
		}
		sort.Slice(codes, func(i, j int) bool {
			return naturalLess(codes[i], codes[j])
		})
		for _, code := range codes {
			fmt.Fprintf(writer, "%s\t%s\t%s\n", variant.Name, code, endpoints[code].Name)
		}
	}
	if err := writer.Flush(); err != nil {
		log.Fatal(err)
	}
}

func naturalLess(left, right string) bool {
	lp, ln := splitCode(left)
	rp, rn := splitCode(right)
	if lp != rp {
		return lp < rp
	}
	if ln != rn {
		return ln < rn
	}
	return left < right
}

func splitCode(code string) (string, int) {
	prefix := ""
	number := 0
	for _, r := range code {
		if r >= '0' && r <= '9' {
			number = number*10 + int(r-'0')
			continue
		}
		prefix += string(r)
	}
	return prefix, number
}
