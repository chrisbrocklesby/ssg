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

	// ExtractInlineStyles extracts <style>...</style> blocks from generated HTML pages,
	// writes them to a single merged CSS file under Out, and replaces the inline
	// <style> blocks with a <link rel="stylesheet"> tag pointing at that file.
	//
	// The link href is computed relative to each page so it works under URL prefixes.
	ExtractInlineStyles bool
	// ExtractInlineStylesOut is the output path relative to Out for the merged CSS.
	// Default (when empty) is "css/inline.css".
	ExtractInlineStylesOut string
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
