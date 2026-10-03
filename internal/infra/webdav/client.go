// Package webdav is the HTTP client for the server: PROPFIND, GET, PUT, MKCOL, MOVE, COPY, DELETE and the SFTPGo quota lookup.
package webdav

import (
	"cmp"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"soteria/internal/domain"
)

type Client struct {
	Base       *url.URL
	User, Pass string
	HTTP       *http.Client
	put        *http.Client
}

func New(s domain.Server, password string) (*Client, error) {
	u, err := url.Parse(strings.TrimSpace(s.URL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid server address %q", s.URL)
	}
	t := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // the stdlib default is always a *Transport
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: s.Insecure}
	t.ResponseHeaderTimeout = 30 * time.Second
	t.MaxIdleConnsPerHost, t.MaxConnsPerHost = 16, 16
	// Uploads stay on HTTP/1.1: a Go HTTP/2 server (SFTPGo) gives each connection one 1 MiB window for every PUT on it.
	t1 := t.Clone()
	t1.Protocols = new(http.Protocols)
	t1.Protocols.SetHTTP1(true)
	return &Client{Base: u, User: s.Username, Pass: password, HTTP: &http.Client{Transport: t}, put: &http.Client{Transport: t1}}, nil
}

func (c *Client) URL(p string) string { return c.Base.JoinPath(p).String() }

func (c *Client) Do(ctx context.Context, method, p string, body io.Reader, size int64, hdr ...string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.URL(p), body)
	if err != nil {
		return nil, err
	}
	if size > 0 {
		req.ContentLength = size
	}
	req.SetBasicAuth(c.User, c.Pass)
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	hc := c.HTTP
	if method == http.MethodPut {
		hc = c.put
	}
	resp, err := hc.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, domain.ErrOffline
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, statusError(resp.StatusCode, method, p)
	}
	return resp, nil
}

func statusError(code int, method, p string) error {
	name := path.Base(p)
	text := fmt.Sprintf("%s %s: %d", method, p, code)
	switch {
	case code == http.StatusUnauthorized:
		text = "invalid username or password"
	case code == http.StatusForbidden:
		text = fmt.Sprintf("you don't have permission to change %q", name)
	case code == http.StatusNotFound:
		text = fmt.Sprintf("%q no longer exists on the server", name)
	case code == http.StatusConflict:
		text = "the destination folder doesn't exist"
	case code == http.StatusPreconditionFailed:
		text = "something with that name already exists there"
	case code == http.StatusLocked:
		text = fmt.Sprintf("%q is locked by another app on the server, try again in a moment", name)
	case code == http.StatusInsufficientStorage:
		text = "the server is out of storage"
	case code >= 500:
		text = fmt.Sprintf("the server had a problem (%d), try again", code)
	}
	return &domain.DavError{Code: code, Text: text}
}

func (c *Client) exec(ctx context.Context, method, p string, hdr ...string) error {
	resp, err := c.Do(ctx, method, p, nil, 0, hdr...)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

const (
	entryProps = `<?xml version="1.0"?><propfind xmlns="DAV:"><prop><resourcetype/><getcontentlength/><getlastmodified/><creationdate/><getcontenttype/><getetag/></prop></propfind>`
	quotaProps = `<?xml version="1.0"?><propfind xmlns="DAV:"><prop><quota-used-bytes/><quota-available-bytes/></prop></propfind>`
)

type propstat struct {
	Status string `xml:"DAV: status"`
	Prop   struct {
		Size     int64  `xml:"DAV: getcontentlength"`
		Modified string `xml:"DAV: getlastmodified"`
		Created  string `xml:"DAV: creationdate"`
		Type     string `xml:"DAV: getcontenttype"`
		ETag     string `xml:"DAV: getetag"`
		Resource struct {
			Collection *struct{} `xml:"DAV: collection"`
		} `xml:"DAV: resourcetype"`
		Used  *int64 `xml:"DAV: quota-used-bytes"`
		Avail *int64 `xml:"DAV: quota-available-bytes"`
	} `xml:"DAV: prop"`
}

type response struct {
	Href  string     `xml:"DAV: href"`
	Stats []propstat `xml:"DAV: propstat"`
}

func (r response) ok() *propstat {
	for i := range r.Stats {
		if strings.Contains(r.Stats[i].Status, " 200 ") {
			return &r.Stats[i]
		}
	}
	return nil
}

// propfind streams one <response> at a time so a whole-tree answer is never held in memory.
func (c *Client) propfind(ctx context.Context, p, depth, props string, each func(response)) error {
	resp, err := c.Do(ctx, "PROPFIND", p, strings.NewReader(props), 0, "Depth", depth, "Content-Type", "application/xml")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := xml.NewDecoder(resp.Body)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("not a WebDAV server: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name == (xml.Name{Space: "DAV:", Local: "response"}) {
			var r response
			if err := dec.DecodeElement(&r, &se); err != nil {
				return fmt.Errorf("not a WebDAV server: %w", err)
			}
			each(r)
		}
	}
}

// Ping is the cheapest round trip that also validates the login.
func (c *Client) Ping(ctx context.Context) error {
	return c.propfind(ctx, "/", "0", entryProps, func(response) {})
}

func (c *Client) entry(r response) (domain.Entry, bool) {
	ps := r.ok()
	if ps == nil {
		return domain.Entry{}, false
	}
	href := r.Href
	if u, err := url.Parse(href); err == nil {
		href = u.Path
	}
	p := path.Clean("/" + strings.TrimPrefix(href, strings.TrimSuffix(c.Base.Path, "/")))
	e := domain.Entry{Name: path.Base(p), Path: p, Dir: ps.Prop.Resource.Collection != nil, Size: ps.Prop.Size, ContentType: ps.Prop.Type, ETag: ps.Prop.ETag}
	if t, err := http.ParseTime(ps.Prop.Modified); err == nil {
		e.Modified = t
	}
	if t, err := time.Parse(time.RFC3339, ps.Prop.Created); err == nil {
		e.Created = t
	}
	return e, true
}

func (c *Client) List(ctx context.Context, p string) ([]domain.Entry, error) {
	self := path.Clean("/" + p)
	out := []domain.Entry{}
	err := c.propfind(ctx, p, "1", entryProps, func(r response) {
		if e, ok := c.entry(r); ok && e.Path != self && e.Path != domain.TrashDir {
			out = append(out, e)
		}
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b domain.Entry) int {
		if a.Dir != b.Dir {
			if a.Dir {
				return -1
			}
			return 1
		}
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

// Tree returns everything under root, walking folder by folder when Depth: infinity is refused or downgraded (SabreDAV).
func (c *Client) Tree(ctx context.Context, root string, progress func(domain.IndexStatus)) ([]domain.Entry, error) {
	root = path.Clean("/" + root)
	out := []domain.Entry{}
	var dirs []string
	deep := false
	err := c.propfind(ctx, root, "infinity", entryProps, func(r response) {
		if e, ok := c.entry(r); ok && e.Path != root && !strings.HasPrefix(e.Path, domain.TrashDir) {
			out = append(out, e)
			deep = deep || path.Dir(e.Path) != root
			if e.Dir {
				dirs = append(dirs, e.Path)
			}
		}
	})
	if err == nil && deep {
		return out, nil
	}
	queue := dirs
	if err != nil {
		out, queue = out[:0], []string{root}
	}
	last := time.Now()
	for done := 1; len(queue) > 0; done++ {
		list, err := c.List(ctx, queue[0])
		queue = queue[1:]
		if de, ok := errors.AsType[*domain.DavError](err); ok && (de.Code == http.StatusForbidden || de.Code == http.StatusNotFound) {
			continue
		}
		// A dropped connection must fail the walk, not pass for a smaller tree.
		if err != nil {
			return nil, err
		}
		for _, e := range list {
			out = append(out, e)
			if e.Dir {
				queue = append(queue, e.Path)
			}
		}
		if progress != nil && time.Since(last) > 300*time.Millisecond {
			last = time.Now()
			progress(domain.IndexStatus{Count: len(out), Folders: done, Pending: len(queue)})
		}
	}
	return out, nil
}

func (c *Client) Quota(ctx context.Context) (domain.Quota, error) {
	q := domain.Quota{Used: -1, Available: -1}
	err := c.propfind(ctx, "/", "0", quotaProps, func(r response) {
		if ps := r.ok(); ps != nil {
			if ps.Prop.Used != nil {
				q.Used = *ps.Prop.Used
			}
			if ps.Prop.Avail != nil {
				q.Available = *ps.Prop.Avail
			}
		}
	})
	if err != nil {
		return q, err
	}
	if q.Used < 0 {
		if rq, ok := c.restQuota(ctx); ok {
			return rq, nil
		}
	}
	return q, nil
}

// restQuota reads the SFTPGo user a reverse proxy exposes at /api/quota/<user>; quota_size 0 means unlimited.
func (c *Client) restQuota(ctx context.Context) (domain.Quota, bool) {
	u := *c.Base
	u.Path, u.RawQuery = "/api/quota/"+url.PathEscape(c.User), ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return domain.Quota{}, false
	}
	req.SetBasicAuth(c.User, c.Pass)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return domain.Quota{}, false
	}
	defer resp.Body.Close()
	var v struct {
		Username string `json:"username"`
		Size     int64  `json:"quota_size"`
		Used     int64  `json:"used_quota_size"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&v) != nil || v.Username != c.User {
		return domain.Quota{}, false
	}
	q := domain.Quota{Used: v.Used, Available: -1}
	if v.Size > 0 {
		q.Available = max(v.Size-v.Used, 0)
	}
	return q, true
}

func (c *Client) Mkcol(ctx context.Context, p string) error  { return c.exec(ctx, "MKCOL", p) }
func (c *Client) Remove(ctx context.Context, p string) error { return c.exec(ctx, "DELETE", p) }

func (c *Client) Move(ctx context.Context, from, to string) error {
	return c.exec(ctx, "MOVE", from, "Destination", c.URL(to), "Overwrite", "F")
}

func (c *Client) Copy(ctx context.Context, from, to string) error {
	return c.exec(ctx, "COPY", from, "Destination", c.URL(to), "Overwrite", "F")
}

// Mkdirs creates dir and its parents, ignoring the ones that already exist.
func (c *Client) Mkdirs(ctx context.Context, dir string) {
	p := "/"
	for seg := range strings.SplitSeq(strings.Trim(dir, "/"), "/") {
		if seg != "" {
			p = path.Join(p, seg)
			_ = c.Mkcol(ctx, p)
		}
	}
}

func (c *Client) Get(ctx context.Context, p string) (*http.Response, error) {
	return c.Do(ctx, "GET", p, nil, 0)
}

func (c *Client) Put(ctx context.Context, p string, r io.Reader, size int64) error {
	if size == 0 {
		r = http.NoBody
	}
	resp, err := c.Do(ctx, "PUT", p, r, size)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
