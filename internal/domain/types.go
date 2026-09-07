// Package domain holds the types every layer shares and the few rules that don't need I/O.
package domain

import "time"

type Entry struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Dir         bool      `json:"dir"`
	Size        int64     `json:"size"`
	Modified    time.Time `json:"modified"`
	Created     time.Time `json:"created"`
	ContentType string    `json:"contentType"`
	ETag        string    `json:"etag"`
}

// Quota in bytes; -1 when the server does not report it.
type Quota struct {
	Used      int64 `json:"used"`
	Available int64 `json:"available"`
}

type Server struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Insecure bool   `json:"insecure"`
	Remember bool   `json:"remember"`
	LastUsed int64  `json:"lastUsed"`
}

type Transfer struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Group  string `json:"group"`
	Name   string `json:"name"`
	Remote string `json:"remote"`
	Local  string `json:"local"`
	Done   int64  `json:"done"`
	Total  int64  `json:"total"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

type Conflict struct {
	Local         string    `json:"local"`
	Remote        string    `json:"remote"`
	Dir           bool      `json:"dir"`
	Size          int64     `json:"size"`
	Modified      time.Time `json:"modified"`
	LocalSize     int64     `json:"localSize"`
	LocalModified time.Time `json:"localModified"`
}

type TrashItem struct {
	Entry
	From    string    `json:"from"`
	Deleted time.Time `json:"deleted"`
}

type Drive struct {
	Path    string `json:"path"`
	Mounted bool   `json:"mounted"`
}

type IndexStatus struct {
	Count   int   `json:"count"`
	Folders int   `json:"folders"`
	Pending int   `json:"pending"`
	Done    bool  `json:"done"`
	At      int64 `json:"at"`
}
