package operations

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	uueMaxBytesPerLine = 45
	xxeAlphabet        = "+-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	mimeLineLength     = 76
	mimeRawChunkSize   = 57 // 57 raw bytes -> 76 base64 characters
)

var xxeTable = func() [256]int8 {
	var table [256]int8
	for i := range table {
		table[i] = -1
	}
	for i := 0; i < len(xxeAlphabet); i++ {
		table[xxeAlphabet[i]] = int8(i)
	}
	return table
}()

func uuEncodeByte(b byte) byte {
	b &= 0x3f
	if b == 0 {
		return '`'
	}
	return b + 32
}

func uuDecodeByte(b byte) byte {
	if b == '`' || b == ' ' {
		return 0
	}
	return (b - 32) & 0x3f
}

func readCleanLine(br *bufio.Reader) (string, error) {
	line, err := br.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// UUEncode encodes data from src to Unix-to-Unix format and writes to dst.
func UUEncode(src io.Reader, dst io.Writer, filename string, mode uint32) error {
	if mode == 0 {
		mode = 0644
	}
	cleanName := filepath.Base(filename)
	if cleanName == "" || cleanName == "." {
		cleanName = "file"
	}

	if _, err := fmt.Fprintf(dst, "begin %03o %s\n", mode&0777, cleanName); err != nil {
		return err
	}

	buf := make([]byte, uueMaxBytesPerLine)
	out := make([]byte, 4)

	for {
		n, err := io.ReadFull(src, buf)
		if n > 0 {
			lenByte := uuEncodeByte(byte(n))
			if _, werr := dst.Write([]byte{lenByte}); werr != nil {
				return werr
			}

			for i := 0; i < n; i += 3 {
				var b0, b1, b2 byte
				b0 = buf[i]
				if i+1 < n {
					b1 = buf[i+1]
				}
				if i+2 < n {
					b2 = buf[i+2]
				}

				c0 := (b0 >> 2) & 0x3f
				c1 := ((b0 << 4) | (b1 >> 4)) & 0x3f
				c2 := ((b1 << 2) | (b2 >> 6)) & 0x3f
				c3 := b2 & 0x3f

				out[0] = uuEncodeByte(c0)
				out[1] = uuEncodeByte(c1)
				out[2] = uuEncodeByte(c2)
				out[3] = uuEncodeByte(c3)

				if _, werr := dst.Write(out); werr != nil {
					return werr
				}
			}

			if _, werr := dst.Write([]byte{'\n'}); werr != nil {
				return werr
			}
		}

		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}
	}

	_, err := dst.Write([]byte("`\nend\n"))
	return err
}

// UUDecode decodes a UUEncoded stream from src, saving the file to dstDir.
func UUDecode(src io.Reader, dstDir string) (outPath string, err error) {
	br := bufio.NewReader(src)

	var (
		modeVal  uint32 = 0644
		filename string
		found    bool
	)

	for {
		line, rerr := readCleanLine(br)
		if rerr != nil {
			return "", fmt.Errorf("uuencode header not found: %w", rerr)
		}
		if strings.HasPrefix(line, "begin ") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 3 {
				parsedMode, perr := strconv.ParseUint(parts[1], 8, 32)
				if perr == nil && parsedMode > 0 {
					modeVal = uint32(parsedMode)
				}
				filename = parts[2]
				found = true
				break
			}
		}
	}

	if !found {
		return "", fmt.Errorf("uuencode header not found")
	}

	cleanName := filepath.Base(filepath.Clean(filename))
	if cleanName == "." || cleanName == "/" || cleanName == "\\" || cleanName == "" {
		cleanName = "decoded_file"
	}

	if err = os.MkdirAll(dstDir, 0755); err != nil {
		return "", err
	}

	targetPath := filepath.Join(dstDir, cleanName)
	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(modeVal&0777))
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := outFile.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	_ = os.Chmod(targetPath, os.FileMode(modeVal&0777))

	for {
		line, rerr := readCleanLine(br)
		if rerr != nil && len(line) == 0 {
			if rerr == io.EOF {
				return "", fmt.Errorf("unexpected EOF before end marker")
			}
			return "", rerr
		}

		if line == "end" {
			break
		}
		if len(line) == 0 {
			continue
		}

		lenChar := line[0]
		if lenChar == '`' || lenChar == ' ' {
			continue
		}

		lineLen := int((lenChar - 32) & 0x3f)
		if lineLen <= 0 {
			continue
		}

		dataStr := line[1:]
		dataBytes := make([]byte, 0, lineLen)

		for i := 0; i < len(dataStr) && len(dataBytes) < lineLen; i += 4 {
			var c0, c1, c2, c3 byte = '`', '`', '`', '`'
			c0 = dataStr[i]
			if i+1 < len(dataStr) {
				c1 = dataStr[i+1]
			}
			if i+2 < len(dataStr) {
				c2 = dataStr[i+2]
			}
			if i+3 < len(dataStr) {
				c3 = dataStr[i+3]
			}

			v0 := uuDecodeByte(c0)
			v1 := uuDecodeByte(c1)
			v2 := uuDecodeByte(c2)
			v3 := uuDecodeByte(c3)

			b0 := (v0 << 2) | (v1 >> 4)
			b1 := (v1 << 4) | (v2 >> 2)
			b2 := (v2 << 6) | v3

			dataBytes = append(dataBytes, b0)
			if len(dataBytes) < lineLen {
				dataBytes = append(dataBytes, b1)
			}
			if len(dataBytes) < lineLen {
				dataBytes = append(dataBytes, b2)
			}
		}

		if _, werr := outFile.Write(dataBytes); werr != nil {
			return "", werr
		}
	}

	return targetPath, nil
}

func xxDecodeByte(b byte) (byte, error) {
	val := xxeTable[b]
	if val < 0 {
		return 0, fmt.Errorf("invalid xxe char: %q", b)
	}
	return byte(val), nil
}

// XXEncode encodes data from src using XXEncode algorithm and writes to dst.
func XXEncode(src io.Reader, dst io.Writer, filename string, mode uint32) error {
	if mode == 0 {
		mode = 0644
	}
	cleanName := filepath.Base(filename)
	if cleanName == "" || cleanName == "." {
		cleanName = "file"
	}

	if _, err := fmt.Fprintf(dst, "begin %03o %s\n", mode&0777, cleanName); err != nil {
		return err
	}

	buf := make([]byte, uueMaxBytesPerLine)
	out := make([]byte, 4)

	for {
		n, err := io.ReadFull(src, buf)
		if n > 0 {
			lenByte := xxeAlphabet[n]
			if _, werr := dst.Write([]byte{lenByte}); werr != nil {
				return werr
			}

			for i := 0; i < n; i += 3 {
				var b0, b1, b2 byte
				b0 = buf[i]
				if i+1 < n {
					b1 = buf[i+1]
				}
				if i+2 < n {
					b2 = buf[i+2]
				}

				c0 := (b0 >> 2) & 0x3f
				c1 := ((b0 << 4) | (b1 >> 4)) & 0x3f
				c2 := ((b1 << 2) | (b2 >> 6)) & 0x3f
				c3 := b2 & 0x3f

				out[0] = xxeAlphabet[c0]
				out[1] = xxeAlphabet[c1]
				out[2] = xxeAlphabet[c2]
				out[3] = xxeAlphabet[c3]

				if _, werr := dst.Write(out); werr != nil {
					return werr
				}
			}

			if _, werr := dst.Write([]byte{'\n'}); werr != nil {
				return werr
			}
		}

		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}
	}

	_, err := dst.Write([]byte("+\nend\n"))
	return err
}

// XXDecode decodes an XXEncoded stream from src, saving the file to dstDir.
func XXDecode(src io.Reader, dstDir string) (outPath string, err error) {
	br := bufio.NewReader(src)

	var (
		modeVal  uint32 = 0644
		filename string
		found    bool
	)

	for {
		line, rerr := readCleanLine(br)
		if rerr != nil {
			return "", fmt.Errorf("xxencode header not found: %w", rerr)
		}
		if strings.HasPrefix(line, "begin ") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) >= 3 {
				parsedMode, perr := strconv.ParseUint(parts[1], 8, 32)
				if perr == nil && parsedMode > 0 {
					modeVal = uint32(parsedMode)
				}
				filename = parts[2]
				found = true
				break
			}
		}
	}

	if !found {
		return "", fmt.Errorf("xxencode header not found")
	}

	cleanName := filepath.Base(filepath.Clean(filename))
	if cleanName == "." || cleanName == "/" || cleanName == "\\" || cleanName == "" {
		cleanName = "decoded_file"
	}

	if err = os.MkdirAll(dstDir, 0755); err != nil {
		return "", err
	}

	targetPath := filepath.Join(dstDir, cleanName)
	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(modeVal&0777))
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := outFile.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	_ = os.Chmod(targetPath, os.FileMode(modeVal&0777))

	for {
		line, rerr := readCleanLine(br)
		if rerr != nil && len(line) == 0 {
			if rerr == io.EOF {
				return "", fmt.Errorf("unexpected EOF before end marker")
			}
			return "", rerr
		}

		if line == "end" {
			break
		}
		if len(line) == 0 {
			continue
		}

		lenVal := xxeTable[line[0]]
		if lenVal < 0 {
			return "", fmt.Errorf("invalid xxe length char: %q", line[0])
		}
		lineLen := int(lenVal)
		if lineLen == 0 {
			continue
		}

		dataStr := line[1:]
		dataBytes := make([]byte, 0, lineLen)

		for i := 0; i < len(dataStr) && len(dataBytes) < lineLen; i += 4 {
			var c0, c1, c2, c3 byte = '+', '+', '+', '+'
			c0 = dataStr[i]
			if i+1 < len(dataStr) {
				c1 = dataStr[i+1]
			}
			if i+2 < len(dataStr) {
				c2 = dataStr[i+2]
			}
			if i+3 < len(dataStr) {
				c3 = dataStr[i+3]
			}

			v0, err0 := xxDecodeByte(c0)
			v1, err1 := xxDecodeByte(c1)
			v2, err2 := xxDecodeByte(c2)
			v3, err3 := xxDecodeByte(c3)
			if err0 != nil || err1 != nil || err2 != nil || err3 != nil {
				return "", fmt.Errorf("invalid character in xxe data")
			}

			b0 := (v0 << 2) | (v1 >> 4)
			b1 := (v1 << 4) | (v2 >> 2)
			b2 := (v2 << 6) | v3

			dataBytes = append(dataBytes, b0)
			if len(dataBytes) < lineLen {
				dataBytes = append(dataBytes, b1)
			}
			if len(dataBytes) < lineLen {
				dataBytes = append(dataBytes, b2)
			}
		}

		if _, werr := outFile.Write(dataBytes); werr != nil {
			return "", werr
		}
	}

	return targetPath, nil
}

type whitespaceFilterReader struct {
	r   io.Reader
	eof bool
}

func (f *whitespaceFilterReader) Read(p []byte) (int, error) {
	if f.eof {
		return 0, io.EOF
	}

	buf := make([]byte, len(p))
	for {
		n, err := f.r.Read(buf)
		if err != nil {
			if err == io.EOF {
				f.eof = true
			} else {
				return 0, err
			}
		}

		w := 0
		for i := 0; i < n; i++ {
			b := buf[i]
			if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
				p[w] = b
				w++
			}
		}

		if w > 0 {
			return w, nil
		}
		if f.eof {
			return 0, io.EOF
		}
	}
}

// MIMEEncode encodes a stream into Base64, wrapping lines at 76 characters.
func MIMEEncode(src io.Reader, dst io.Writer) error {
	buf := make([]byte, mimeRawChunkSize)
	encBuf := make([]byte, mimeLineLength+2)

	for {
		n, err := io.ReadFull(src, buf)
		if n > 0 {
			encLen := base64.StdEncoding.EncodedLen(n)
			base64.StdEncoding.Encode(encBuf, buf[:n])
			encBuf[encLen] = '\r'
			encBuf[encLen+1] = '\n'
			if _, werr := dst.Write(encBuf[:encLen+2]); werr != nil {
				return werr
			}
		}

		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}
	}

	return nil
}

// MIMEDecode decodes Base64 data from src to dst, ignoring whitespace and newlines.
func MIMEDecode(src io.Reader, dst io.Writer) error {
	filter := &whitespaceFilterReader{r: src}
	dec := base64.NewDecoder(base64.StdEncoding, filter)
	buf := make([]byte, 4096)
	_, err := io.CopyBuffer(dst, dec, buf)
	return err
}