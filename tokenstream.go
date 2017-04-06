package json

import (
	//"fmt"
	"io"
)

type TokenStream struct {
	dec   *Decoder
	state TokenStreamState
}

type TokenStreamState int

const (
	Prepare TokenStreamState = iota
	Reading
	Done
)

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func NewTokenStream(dec *Decoder) *TokenStream {
	return &TokenStream{dec: dec, state: Prepare}
}

func (stream *TokenStream) Read(p []byte) (n int, err error) {
	if stream.state == Prepare {
		if stream.dec.tokenState != tokenObjectColon && stream.dec.tokenState != tokenArrayComma {
			return 0, &SyntaxError{"can't stream unless an object value is the next token!", 0}
		}

		err = stream.dec.tokenPrepareForDecode()
		if err != nil {
			return 0, err
		}
		stream.dec.scan.reset()
		stream.state = Reading
	}

	if stream.state == Done {
		return 0, io.EOF
	}

	// at this point, state should be Reading

	if len(p) < 1 {
		return 0, nil
	}

	//fill up enough of the json buffer to copy some data for the caller
	var maxRead int
	err = nil
	for {
		maxRead = min(len(p), len(stream.dec.buf)-stream.dec.scanp)
		if maxRead > 0 {
			if err == io.EOF {
				//if there's more bytes to read, process them
				//instead of just failing with EOF, we'll get
				//an EOF again next attempt to read.
				err = nil
			}
			break
		}

		if err != nil {
			stream.state = Done
			return 0, err
		}

		err = stream.dec.refill()
	}

	maxScanp := stream.dec.scanp + maxRead
	scanp := stream.dec.scanp

	j := 0

	// Look in the buffer for a new value.
	for _, c := range stream.dec.buf[scanp:maxScanp] {
		stream.dec.scan.bytes++

		v := stream.dec.scan.step(&stream.dec.scan, c)
		//fmt.Printf("reading byte %c, v=%v\n", c, v)
		if v == scanEnd {
			stream.dec.tokenState = tokenObjectComma
			j -= 1
			stream.state = Done
			goto end
		}
		// scanEnd is delayed one byte.
		// We might block trying to get that byte from src,
		// so instead invent a space byte.
		if v == scanError {
			stream.dec.tokenState = tokenObjectComma
			stream.state = Done
			stream.dec.err = stream.dec.scan.err
			return 0, stream.dec.scan.err
		}
		stream.dec.scanp++

		if v == 0 {
			p[j] = c
			j++
		}
	}

	// Did the last read have an error?
	// Delayed until now to allow buffer scan.
	if err != nil {
		stream.state = Done
		stream.dec.err = err
		return 0, err
	}

end:
	//fmt.Printf("Returning %v bytes\n", j)
	if j < len(p) && stream.state == Done {
		return j, io.EOF
	} else {
		return j, nil
	}
}
