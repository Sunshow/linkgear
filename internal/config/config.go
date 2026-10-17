package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DSN             string
	HTTPAddr        string
	PublicURL       string
	DeliveryWorkers int
	DeliveryPoll    time.Duration
}

func Load() Config {
	dsn := strings.TrimSpace(os.Getenv("LINKGEAR_DSN"))
	if dsn == "" {
		dsn = "sqlite://./data/linkgear.db"
	}
	addr := strings.TrimSpace(os.Getenv("LINKGEAR_HTTP_ADDR"))
	if addr == "" {
		addr = ":8080"
	}
	public := strings.TrimSpace(os.Getenv("LINKGEAR_PUBLIC_URL"))
	if public == "" {
		public = "http://localhost:8080"
	}
	public = strings.TrimRight(public, "/")

	workers := 8
	if v := strings.TrimSpace(os.Getenv("LINKGEAR_DELIVERY_WORKERS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workers = n
		}
	}
	poll := 2 * time.Second
	if v := strings.TrimSpace(os.Getenv("LINKGEAR_DELIVERY_POLL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			poll = d
		}
	}

	return Config{
		DSN:             dsn,
		HTTPAddr:        addr,
		PublicURL:       public,
		DeliveryWorkers: workers,
		DeliveryPoll:    poll,
	}
}
