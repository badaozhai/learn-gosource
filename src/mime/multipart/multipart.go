// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

/*
Package multipart implements MIME multipart parsing, as defined in RFC
2046.

The implementation is sufficient for HTTP (RFC 2388) and the multipart
bodies generated by popular browsers.
*/
package multipart

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/quotedprintable"
	"net/textproto"
)

var emptyParams = make(map[string]string)

// This constant needs to be at least 76 for this package to work correctly.
// This is because \r\n--separator_of_len_70- would fill the buffer and it
// wouldn't be safe to consume a single byte from it.
const peekBufferSize = 4096

// A Part represents a single part in a multipart body.
type Part struct {
	// The headers of the body, if any, with the keys canonicalized
	// in the same fashion that the Go http.Request headers are.
	// For example, "foo-bar" changes case to "Foo-Bar"
	//
	// As a special case, if the "Content-Transfer-Encoding" header
	// has a value of "quoted-printable", that header is instead
	// hidden from this map and the body is transparently decoded
	// during Read calls.
	Header textproto.MIMEHeader

	buffer    *bytes.Buffer
	mr        *Reader
	bytesRead int

	disposition       string
	dispositionParams map[string]string

	// r is either a reader directly reading from mr, or it's a
	// wrapper around such a reader, decoding the
	// Content-Transfer-Encoding
	r io.Reader
}

// FormName returns the name parameter if p has a Content-Disposition
// of type "form-data".  Otherwise it returns the empty string.
func (p *Part) FormName() string {
	// See http://tools.ietf.org/html/rfc2183 section 2 for EBNF
	// of Content-Disposition value format.
	if p.dispositionParams == nil {
		p.parseContentDisposition()
	}
	if p.disposition != "form-data" {
		return ""
	}
	return p.dispositionParams["name"]
}

// FileName returns the filename parameter of the Part's
// Content-Disposition header.
func (p *Part) FileName() string {
	if p.dispositionParams == nil {
		p.parseContentDisposition()
	}
	return p.dispositionParams["filename"]
}

func (p *Part) parseContentDisposition() {
	v := p.Header.Get("Content-Disposition")
	var err error
	p.disposition, p.dispositionParams, err = mime.ParseMediaType(v)
	if err != nil {
		p.dispositionParams = emptyParams
	}
}

// NewReader creates a new multipart Reader reading from r using the
// given MIME boundary.
//
// The boundary is usually obtained from the "boundary" parameter of
// the message's "Content-Type" header. Use mime.ParseMediaType to
// parse such headers.
func NewReader(r io.Reader, boundary string) *Reader {
	b := []byte("\r\n--" + boundary + "--")
	return &Reader{
		bufReader:        bufio.NewReaderSize(r, peekBufferSize),
		nl:               b[:2],
		nlDashBoundary:   b[:len(b)-2],
		dashBoundaryDash: b[2:],
		dashBoundary:     b[2 : len(b)-2],
	}
}

func newPart(mr *Reader) (*Part, error) {
	bp := &Part{
		Header: make(map[string][]string),
		mr:     mr,
		buffer: new(bytes.Buffer),
	}
	if err := bp.populateHeaders(); err != nil {
		return nil, err
	}
	bp.r = partReader{bp}
	const cte = "Content-Transfer-Encoding"
	if bp.Header.Get(cte) == "quoted-printable" {
		bp.Header.Del(cte)
		bp.r = quotedprintable.NewReader(bp.r)
	}
	return bp, nil
}

func (bp *Part) populateHeaders() error {
	r := textproto.NewReader(bp.mr.bufReader)
	header, err := r.ReadMIMEHeader()
	if err == nil {
		bp.Header = header
	}
	return err
}

// Read reads the body of a part, after its headers and before the
// next part (if any) begins.
func (p *Part) Read(d []byte) (n int, err error) {
	return p.r.Read(d)
}

// partReader implements io.Reader by reading raw bytes directly from the
// wrapped *Part, without doing any Transfer-Encoding decoding.
type partReader struct {
	p *Part
}

func (pr partReader) Read(d []byte) (n int, err error) {
	p := pr.p
	defer func() {
		p.bytesRead += n
	}()
	if p.buffer.Len() >= len(d) {
		// Internal buffer of unconsumed data is large enough for
		// the read request.  No need to parse more at the moment.
		return p.buffer.Read(d)
	}
	peek, err := p.mr.bufReader.Peek(peekBufferSize) // TODO(bradfitz): add buffer size accessor

	// Look for an immediate empty part without a leading \r\n
	// before the boundary separator.  Some MIME code makes empty
	// parts like this. Most browsers, however, write the \r\n
	// before the subsequent boundary even for empty parts and
	// won't hit this path.
	if p.bytesRead == 0 && p.mr.peekBufferIsEmptyPart(peek) {
		return 0, io.EOF
	}
	unexpectedEOF := err == io.EOF
	if err != nil && !unexpectedEOF {
		return 0, fmt.Errorf("multipart: Part Read: %v", err)
	}
	if peek == nil {
		panic("nil peek buf")
	}
	// Search the peek buffer for "\r\n--boundary". If found,
	// consume everything up to the boundary. If not, consume only
	// as much of the peek buffer as cannot hold the boundary
	// string.
	nCopy := 0
	foundBoundary := false
	if idx, isEnd := p.mr.peekBufferSeparatorIndex(peek); idx != -1 {
		nCopy = idx
		foundBoundary = isEnd
		if !isEnd && nCopy == 0 {
			nCopy = 1 // make some progress.
		}
	} else if safeCount := len(peek) - len(p.mr.nlDashBoundary); safeCount > 0 {
		nCopy = safeCount
	} else if unexpectedEOF {
		// If we've run out of peek buffer and the boundary
		// wasn't found (and can't possibly fit), we must have
		// hit the end of the file unexpectedly.
		return 0, io.ErrUnexpectedEOF
	}
	if nCopy > 0 {
		if _, err := io.CopyN(p.buffer, p.mr.bufReader, int64(nCopy)); err != nil {
			return 0, err
		}
	}
	n, err = p.buffer.Read(d)
	if err == io.EOF && !foundBoundary {
		// If the boundary hasn't been reached there's more to
		// read, so don't pass through an EOF from the buffer
		err = nil
	}
	return
}

func (p *Part) Close() error {
	io.Copy(ioutil.Discard, p)
	return nil
}

// Reader is an iterator over parts in a MIME multipart body.
// Reader's underlying parser consumes its input as needed.  Seeking
// isn't supported.
type Reader struct {
	bufReader *bufio.Reader

	currentPart *Part
	partsRead   int

	nl               []byte // "\r\n" or "\n" (set after seeing first boundary line)
	nlDashBoundary   []byte // nl + "--boundary"
	dashBoundaryDash []byte // "--boundary--"
	dashBoundary     []byte // "--boundary"
}

// NextPart returns the next part in the multipart or an error.
// When there are no more parts, the error io.EOF is returned.
func (r *Reader) NextPart() (*Part, error) {
	if r.currentPart != nil {
		r.currentPart.Close()
	}

	expectNewPart := false
	for {
		line, err := r.bufReader.ReadSlice('\n')

		if err == io.EOF && r.isFinalBoundary(line) {
			// If the buffer ends in "--boundary--" without the
			// trailing "\r\n", ReadSlice will return an error
			// (since it's missing the '\n'), but this is a valid
			// multipart EOF so we need to return io.EOF instead of
			// a fmt-wrapped one.
			return nil, io.EOF
		}
		if err != nil {
			return nil, fmt.Errorf("multipart: NextPart: %v", err)
		}

		if r.isBoundaryDelimiterLine(line) {
			r.partsRead++
			bp, err := newPart(r)
			if err != nil {
				return nil, err
			}
			r.currentPart = bp
			return bp, nil
		}

		if r.isFinalBoundary(line) {
			// Expected EOF
			return nil, io.EOF
		}

		if expectNewPart {
			return nil, fmt.Errorf("multipart: expecting a new Part; got line %q", string(line))
		}

		if r.partsRead == 0 {
			// skip line
			continue
		}

		// Consume the "\n" or "\r\n" separator between the
		// body of the previous part and the boundary line we
		// now expect will follow. (either a new part or the
		// end boundary)
		if bytes.Equal(line, r.nl) {
			expectNewPart = true
			continue
		}

		return nil, fmt.Errorf("multipart: unexpected line in Next(): %q", line)
	}
}

// isFinalBoundary reports whether line is the final boundary line
// indicating that all parts are over.
// It matches `^--boundary--[ \t]*(\r\n)?$`
func (mr *Reader) isFinalBoundary(line []byte) bool {
	if !bytes.HasPrefix(line, mr.dashBoundaryDash) {
		return false
	}
	rest := line[len(mr.dashBoundaryDash):]
	rest = skipLWSPChar(rest)
	return len(rest) == 0 || bytes.Equal(rest, mr.nl)
}

func (mr *Reader) isBoundaryDelimiterLine(line []byte) (ret bool) {
	// http://tools.ietf.org/html/rfc2046#section-5.1
	//   The boundary delimiter line is then defined as a line
	//   consisting entirely of two hyphen characters ("-",
	//   decimal value 45) followed by the boundary parameter
	//   value from the Content-Type header field, optional linear
	//   whitespace, and a terminating CRLF.
	if !bytes.HasPrefix(line, mr.dashBoundary) {
		return false
	}
	rest := line[len(mr.dashBoundary):]
	rest = skipLWSPChar(rest)

	// On the first part, see our lines are ending in \n instead of \r\n
	// and switch into that mode if so.  This is a violation of the spec,
	// but occurs in practice.
	if mr.partsRead == 0 && len(rest) == 1 && rest[0] == '\n' {
		mr.nl = mr.nl[1:]
		mr.nlDashBoundary = mr.nlDashBoundary[1:]
	}
	return bytes.Equal(rest, mr.nl)
}

// peekBufferIsEmptyPart reports whether the provided peek-ahead
// buffer represents an empty part. It is called only if we've not
// already read any bytes in this part and checks for the case of MIME
// software not writing the \r\n on empty parts. Some does, some
// doesn't.
//
// This checks that what follows the "--boundary" is actually the end
// ("--boundary--" with optional whitespace) or optional whitespace
// and then a newline, so we don't catch "--boundaryFAKE", in which
// case the whole line is part of the data.
func (mr *Reader) peekBufferIsEmptyPart(peek []byte) bool {
	// End of parts case.
	// Test whether peek matches `^--boundary--[ \t]*(?:\r\n|$)`
	if bytes.HasPrefix(peek, mr.dashBoundaryDash) {
		rest := peek[len(mr.dashBoundaryDash):]
		rest = skipLWSPChar(rest)
		return bytes.HasPrefix(rest, mr.nl) || len(rest) == 0
	}
	if !bytes.HasPrefix(peek, mr.dashBoundary) {
		return false
	}
	// Test whether rest matches `^[ \t]*\r\n`)
	rest := peek[len(mr.dashBoundary):]
	rest = skipLWSPChar(rest)
	return bytes.HasPrefix(rest, mr.nl)
}

// peekBufferSeparatorIndex returns the index of mr.nlDashBoundary in
// peek and whether it is a real boundary (and not a prefix of an
// unrelated separator). To be the end, the peek buffer must contain a
// newline after the boundary or contain the ending boundary (--separator--).
func (mr *Reader) peekBufferSeparatorIndex(peek []byte) (idx int, isEnd bool) {
	idx = bytes.Index(peek, mr.nlDashBoundary)
	if idx == -1 {
		return
	}

	peek = peek[idx+len(mr.nlDashBoundary):]
	if len(peek) == 0 || len(peek) == 1 && peek[0] == '-' {
		return idx, false
	}
	if len(peek) > 1 && peek[0] == '-' && peek[1] == '-' {
		return idx, true
	}
	peek = skipLWSPChar(peek)
	// Don't have a complete line after the peek.
	if bytes.IndexByte(peek, '\n') == -1 {
		return idx, false
	}
	if len(peek) > 0 && peek[0] == '\n' {
		return idx, true
	}
	if len(peek) > 1 && peek[0] == '\r' && peek[1] == '\n' {
		return idx, true
	}
	return idx, false
}

// skipLWSPChar returns b with leading spaces and tabs removed.
// RFC 822 defines:
//    LWSP-char = SPACE / HTAB
func skipLWSPChar(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t') {
		b = b[1:]
	}
	return b
}
