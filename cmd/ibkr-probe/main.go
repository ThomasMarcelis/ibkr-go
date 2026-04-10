package main

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4101", "probe address")
	mode := flag.String("mode", "bootstrap", "probe mode: bootstrap|bootstrap-scan|ascii|hex")
	minVersion := flag.Int("min-version", 100, "bootstrap min version")
	maxVersion := flag.Int("max-version", 200, "bootstrap max version")
	terminator := flag.String("terminator", "none", "bootstrap terminator: none|null|newline|null-newline")
	lengthPrefix := flag.Bool("length-prefix", false, "prefix the payload with a 4-byte big-endian length")
	bootstrapExtra := flag.String("bootstrap-extra", "", "escaped suffix appended to the bootstrap payload")
	useTLS := flag.Bool("tls", false, "dial the target using TLS")
	nullTerminate := flag.Bool("null-terminate", false, "deprecated alias for -terminator=null")
	newline := flag.Bool("newline", false, "deprecated alias for -terminator=newline")
	asciiPayload := flag.String("ascii", "", "escaped payload for mode=ascii")
	hexPayload := flag.String("hex", "", "hex payload for mode=hex")
	readTimeout := flag.Duration("read-timeout", 1500*time.Millisecond, "read timeout after the final write")
	flag.Parse()

	if *nullTerminate && *newline {
		*terminator = "null-newline"
	} else if *nullTerminate {
		*terminator = "null"
	} else if *newline {
		*terminator = "newline"
	}

	switch *mode {
	case "bootstrap":
		extra, err := parseEscaped(*bootstrapExtra)
		if err != nil {
			log.Fatalf("parse bootstrap extra: %v", err)
		}
		payload, err := buildBootstrapPayload(*minVersion, *maxVersion, extra, *terminator, *lengthPrefix)
		if err != nil {
			log.Fatalf("build bootstrap payload: %v", err)
		}
		if err := runSingle(*addr, payload, *readTimeout, *useTLS); err != nil {
			log.Fatal(err)
		}
	case "bootstrap-scan":
		if err := runScan(*addr, bootstrapVariants(*minVersion, *maxVersion), *readTimeout, *useTLS); err != nil {
			log.Fatal(err)
		}
	case "ascii":
		payload, err := parseEscaped(*asciiPayload)
		if err != nil {
			log.Fatalf("parse ascii payload: %v", err)
		}
		if err := runSingle(*addr, payload, *readTimeout, *useTLS); err != nil {
			log.Fatal(err)
		}
	case "hex":
		payload, err := hex.DecodeString(stripWhitespace(*hexPayload))
		if err != nil {
			log.Fatalf("decode hex payload: %v", err)
		}
		if err := runSingle(*addr, payload, *readTimeout, *useTLS); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unsupported mode %q", *mode)
	}
}

func runScan(addr string, variants []probeVariant, readTimeout time.Duration, useTLS bool) error {
	return scanVariants(variants, func(variant probeVariant) error {
		return runVariant(addr, variant, readTimeout, useTLS)
	})
}

func runSingle(addr string, payload []byte, readTimeout time.Duration, useTLS bool) error {
	return runVariant(addr, probeVariant{
		Name:   "single",
		Chunks: [][]byte{payload},
	}, readTimeout, useTLS)
}

func scanVariants(variants []probeVariant, run func(probeVariant) error) error {
	var failed bool
	for i, variant := range variants {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("variant: %s\n", variant.Name)
		if err := run(variant); err != nil {
			failed = true
			fmt.Fprintf(os.Stderr, "variant %s failed: %v\n", variant.Name, err)
		}
	}
	if failed {
		return fmt.Errorf("one or more probe variants failed")
	}
	return nil
}

func runVariant(addr string, variant probeVariant, readTimeout time.Duration, useTLS bool) error {
	response, readState, err := probeOnce(addr, variant.Chunks, variant.Gap, readTimeout, useTLS)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}

	total := 0
	for _, chunk := range variant.Chunks {
		total += len(chunk)
	}
	fmt.Fprintf(os.Stdout, "sent %d bytes\n", total)
	if len(variant.Chunks) == 1 {
		fmt.Fprintf(os.Stdout, "sent hex: %s\n", hex.EncodeToString(variant.Chunks[0]))
	} else {
		for idx, chunk := range variant.Chunks {
			fmt.Fprintf(os.Stdout, "sent chunk[%d] hex: %s\n", idx, hex.EncodeToString(chunk))
		}
	}
	fmt.Fprintf(os.Stdout, "received %d bytes\n", len(response))
	fmt.Fprintf(os.Stdout, "received hex: %s\n", hex.EncodeToString(response))
	fmt.Fprintf(os.Stdout, "read state: %s\n", readState)
	if len(response) > 0 {
		fmt.Fprintf(os.Stdout, "received quoted: %q\n", string(response))
	}
	for _, line := range describeResponse(response) {
		fmt.Fprintln(os.Stdout, line)
	}
	return nil
}

func probeOnce(addr string, chunks [][]byte, gap time.Duration, readTimeout time.Duration, useTLS bool) ([]byte, string, error) {
	conn, err := dialTarget(addr, useTLS)
	if err != nil {
		return nil, "", fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	for idx, chunk := range chunks {
		if _, err := conn.Write(chunk); err != nil {
			return nil, "", fmt.Errorf("write chunk %d: %w", idx, err)
		}
		if gap > 0 && idx < len(chunks)-1 {
			time.Sleep(gap)
		}
	}
	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return nil, "", fmt.Errorf("set read deadline: %w", err)
	}
	response, readState, err := readResponse(conn)
	if err != nil {
		return nil, "", fmt.Errorf("read: %w", err)
	}
	return response, readState, nil
}

func dialTarget(addr string, useTLS bool) (net.Conn, error) {
	if !useTLS {
		return net.Dial("tcp", addr)
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true,
	})
}

type probeVariant struct {
	Name   string
	Chunks [][]byte
	Gap    time.Duration
}

func bootstrapVariants(minVersion, maxVersion int) []probeVariant {
	specs := []struct {
		name         string
		terminator   string
		lengthPrefix bool
	}{
		{name: "plain", terminator: "none"},
		{name: "null", terminator: "null"},
		{name: "newline", terminator: "newline"},
		{name: "null-newline", terminator: "null-newline"},
		{name: "length-prefixed", terminator: "none", lengthPrefix: true},
		{name: "length-prefixed-null", terminator: "null", lengthPrefix: true},
		{name: "length-prefixed-newline", terminator: "newline", lengthPrefix: true},
		{name: "length-prefixed-null-newline", terminator: "null-newline", lengthPrefix: true},
	}

	variants := make([]probeVariant, 0, len(specs))
	for _, spec := range specs {
		payload, err := buildBootstrapPayload(minVersion, maxVersion, nil, spec.terminator, spec.lengthPrefix)
		if err != nil {
			log.Fatalf("build bootstrap variant %q: %v", spec.name, err)
		}
		variants = append(variants, probeVariant{
			Name:   spec.name,
			Chunks: [][]byte{payload},
		})
	}

	apiPrefix, err := parseEscaped("API\\0")
	if err != nil {
		log.Fatalf("build bootstrap prefix: %v", err)
	}
	for _, spec := range []struct {
		name       string
		terminator string
	}{
		{name: "split-api-version", terminator: "none"},
		{name: "split-api-version-null", terminator: "null"},
		{name: "split-api-version-newline", terminator: "newline"},
		{name: "split-api-version-null-newline", terminator: "null-newline"},
	} {
		versionChunk, err := buildBootstrapVersionChunk(minVersion, maxVersion, nil, spec.terminator)
		if err != nil {
			log.Fatalf("build bootstrap split variant %q: %v", spec.name, err)
		}
		variants = append(variants, probeVariant{
			Name:   spec.name,
			Chunks: [][]byte{apiPrefix, versionChunk},
			Gap:    100 * time.Millisecond,
		})
	}
	return variants
}

func buildBootstrapPayload(minVersion, maxVersion int, extra []byte, terminator string, lengthPrefix bool) ([]byte, error) {
	payload := codec.EncodeHandshakePrefix()
	versionChunk, err := buildBootstrapVersionChunk(minVersion, maxVersion, extra, terminator)
	if err != nil {
		return nil, err
	}
	payload = append(payload, versionChunk...)

	if lengthPrefix {
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
		payload = append(header[:], payload...)
	}
	return payload, nil
}

func buildBootstrapVersionChunk(minVersion, maxVersion int, extra []byte, terminator string) ([]byte, error) {
	payload := codec.EncodeVersionRange(minVersion, maxVersion)
	payload = append(payload, extra...)

	switch terminator {
	case "none":
	case "null":
		payload = append(payload, 0)
	case "newline":
		payload = append(payload, '\n')
	case "null-newline":
		payload = append(payload, 0, '\n')
	default:
		return nil, fmt.Errorf("unsupported terminator %q", terminator)
	}
	return payload, nil
}

func parseEscaped(raw string) ([]byte, error) {
	out := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			out = append(out, raw[i])
			continue
		}
		i++
		if i >= len(raw) {
			return nil, errors.New("trailing backslash")
		}
		switch raw[i] {
		case '\\':
			out = append(out, '\\')
		case '0':
			out = append(out, 0)
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'x':
			if i+2 >= len(raw) {
				return nil, errors.New("short hex escape")
			}
			value, err := strconv.ParseUint(raw[i+1:i+3], 16, 8)
			if err != nil {
				return nil, fmt.Errorf("invalid hex escape %q: %w", raw[i-1:i+3], err)
			}
			out = append(out, byte(value))
			i += 2
		default:
			return nil, fmt.Errorf("unsupported escape \\%c", raw[i])
		}
	}
	return out, nil
}

func readResponse(conn net.Conn) ([]byte, string, error) {
	var response []byte
	buf := make([]byte, 8192)

	for {
		n, err := conn.Read(buf)
		if n > 0 {
			response = append(response, buf[:n]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return response, "eof", nil
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return response, "timeout", nil
			}
			return response, "error", err
		}
	}
}

func describeResponse(response []byte) []string {
	if len(response) == 0 {
		return nil
	}

	lines := make([]string, 0, 8)
	if fields, ok := nulFields(response); ok {
		lines = append(lines, fmt.Sprintf("nul fields: %q", fields))
	}
	if frames := framedPayloads(response); len(frames) > 0 {
		for idx, frame := range frames {
			lines = append(lines, fmt.Sprintf("frame[%d] len=%d hex=%s", idx, len(frame), hex.EncodeToString(frame)))
			if fields, ok := nulFields(frame); ok {
				lines = append(lines, fmt.Sprintf("frame[%d] nul fields: %q", idx, fields))
			}
		}
	}
	return lines
}

func nulFields(payload []byte) ([]string, bool) {
	fields, err := wire.ParseFields(payload)
	if err != nil {
		return nil, false
	}
	return fields, true
}

func framedPayloads(response []byte) [][]byte {
	reader := bytes.NewReader(response)
	frames := make([][]byte, 0, 2)
	for reader.Len() > 0 {
		if reader.Len() < 4 {
			return nil
		}
		payload, err := wire.ReadFrame(reader)
		if err != nil {
			return nil
		}
		frames = append(frames, payload)
	}
	return frames
}

func stripWhitespace(value string) string {
	var out strings.Builder
	out.Grow(len(value))
	for _, r := range value {
		switch r {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
