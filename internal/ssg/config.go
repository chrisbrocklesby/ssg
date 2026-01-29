package ssg

import "time"

type Config struct {
	Src      string
	Out      string
	Cache    string
	NoCache  bool
	Clean    bool
	FetchTTL time.Duration
	Jobs     int
}

type ServeConfig struct {
	Config
	Addr     string
	Watch    bool
	Interval time.Duration
}

type InitConfig struct {
	SrcDir string
}

type FetchReq struct {
	URL string
	As  string // auto|json|text
}
