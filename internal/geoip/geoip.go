package geoip

import (
	"log/slog"
	"net"

	"github.com/oschwald/maxminddb-golang"
)

type Resolver struct {
	db *maxminddb.Reader
}

type geoResult struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
}

func New(dbPath string) (*Resolver, error) {
	if dbPath == "" {
		return &Resolver{}, nil
	}
	db, err := maxminddb.Open(dbPath)
	if err != nil {
		slog.Warn("geoip: failed to open database, geolocation disabled", "path", dbPath, "error", err)
		return &Resolver{}, nil
	}
	slog.Info("geoip: loaded database", "path", dbPath)
	return &Resolver{db: db}, nil
}

func (r *Resolver) Lookup(ipStr string) (country, city string) {
	if r.db == nil || ipStr == "" {
		return "", ""
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", ""
	}
	var result geoResult
	if err := r.db.Lookup(ip, &result); err != nil {
		return "", ""
	}
	return result.Country.ISOCode, result.City.Names["en"]
}

func (r *Resolver) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}
