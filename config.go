package main

type CompressionLevel int

type Config struct {
	CompressionLevel   int    `split_words:"true" default:"-1"` // -1 is DEFLATE's default compression
	Port               string `default:"http"`
	BinanceHostPort    string `split_words:"true" default:"stream.binance.com:443"`
	InsecureSkipVerify bool   `split_words:"true" default:"false"`
}
