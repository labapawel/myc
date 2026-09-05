package operations

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generateBinaryData(size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte((i*37 + 13) % 256)
	}
	return data
}

func TestUUEncodeDecodeRoundtrip(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		mode     uint32
		data     []byte
	}{
		{
			name:     "empty",
			filename: "empty.dat",
			mode:     0644,
			data:     []byte{},
		},
		{
			name:     "single byte",
			filename: "single.bin",
			mode:     0600,
			data:     []byte{0x42},
		},
		{
			name:     "short text",
			filename: "hello.txt",
			mode:     0644,
			data:     []byte("Hello, World!\nUUEncode roundtrip test.\n"),
		},
		{
			name:     "exact 45 bytes (one line)",
			filename: "exact45.dat",
			mode:     0755,
			data:     generateBinaryData(45),
		},
		{
			name:     "46 bytes (two lines boundary)",
			filename: "boundary46.dat",
			mode:     0644,
			data:     generateBinaryData(46),
		},
		{
			name:     "full byte range 0-255",
			filename: "allbytes.bin",
			mode:     0644,
			data: func() []byte {
				b := make([]byte, 256)
				for i := 0; i < 256; i++ {
					b[i] = byte(i)
				}
				return b
			}(),
		},
		{
			name:     "large binary payload",
			filename: "large.bin",
			mode:     0644,
			data:     generateBinaryData(16384),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var encoded bytes.Buffer
			err := UUEncode(bytes.NewReader(tc.data), &encoded, tc.filename, tc.mode)
			if err != nil {
				t.Fatalf("UUEncode failed: %v", err)
			}

			// Verify header structure
			encStr := encoded.String()
			if !strings.HasPrefix(encStr, "begin ") {
				t.Fatalf("encoded stream missing begin header: %q", encStr[:min(len(encStr), 30)])
			}
			if !strings.HasSuffix(encStr, "end\n") {
				t.Fatalf("encoded stream missing end trailer: %q", encStr)
			}

			dstDir := t.TempDir()
			decodedPath, err := UUDecode(bytes.NewReader(encoded.Bytes()), dstDir)
			if err != nil {
				t.Fatalf("UUDecode failed: %v", err)
			}

			expectedPath := filepath.Join(dstDir, tc.filename)
			if decodedPath != expectedPath {
				t.Errorf("decoded path mismatch: got %q, want %q", decodedPath, expectedPath)
			}

			decodedData, err := os.ReadFile(decodedPath)
			if err != nil {
				t.Fatalf("failed reading decoded file: %v", err)
			}

			if !bytes.Equal(tc.data, decodedData) {
				t.Errorf("data mismatch: expected %d bytes, got %d bytes", len(tc.data), len(decodedData))
			}
		})
	}
}

func TestXXEncodeDecodeRoundtrip(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		mode     uint32
		data     []byte
	}{
		{
			name:     "empty",
			filename: "empty.bin",
			mode:     0644,
			data:     []byte{},
		},
		{
			name:     "plain text",
			filename: "test.txt",
			mode:     0644,
			data:     []byte("XXEncode safe alphabet roundtrip verification.\r\nLine two."),
		},
		{
			name:     "all bytes",
			filename: "spectrum.bin",
			mode:     0755,
			data: func() []byte {
				b := make([]byte, 256)
				for i := 0; i < 256; i++ {
					b[i] = byte(i)
				}
				return b
			}(),
		},
		{
			name:     "large binary",
			filename: "archive.dat",
			mode:     0600,
			data:     generateBinaryData(8192),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var encoded bytes.Buffer
			err := XXEncode(bytes.NewReader(tc.data), &encoded, tc.filename, tc.mode)
			if err != nil {
				t.Fatalf("XXEncode failed: %v", err)
			}

			encStr := encoded.String()
			if !strings.HasPrefix(encStr, "begin ") {
				t.Fatalf("encoded stream missing begin header")
			}
			if !strings.HasSuffix(encStr, "+\nend\n") && !strings.HasSuffix(encStr, "end\n") {
				t.Fatalf("encoded stream missing proper end termination")
			}

			dstDir := t.TempDir()
			decodedPath, err := XXDecode(bytes.NewReader(encoded.Bytes()), dstDir)
			if err != nil {
				t.Fatalf("XXDecode failed: %v", err)
			}

			expectedPath := filepath.Join(dstDir, tc.filename)
			if decodedPath != expectedPath {
				t.Errorf("decoded path mismatch: got %q, want %q", decodedPath, expectedPath)
			}

			decodedData, err := os.ReadFile(decodedPath)
			if err != nil {
				t.Fatalf("failed reading decoded file: %v", err)
			}

			if !bytes.Equal(tc.data, decodedData) {
				t.Errorf("data mismatch: expected %d bytes, got %d bytes", len(tc.data), len(decodedData))
			}
		})
	}
}

func TestMIMEEncodeDecodeRoundtrip(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "empty",
			data: []byte{},
		},
		{
			name: "short string",
			data: []byte("Hello, MIME Base64!"),
		},
		{
			name: "exact 57 bytes (produces 76 chars line)",
			data: generateBinaryData(57),
		},
		{
			name: "100 bytes (spans across multiple lines)",
			data: generateBinaryData(100),
		},
		{
			name: "large binary stream",
			data: generateBinaryData(10000),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var encoded bytes.Buffer
			err := MIMEEncode(bytes.NewReader(tc.data), &encoded)
			if err != nil {
				t.Fatalf("MIMEEncode failed: %v", err)
			}

			// Validate line wrapping at 76 chars
			lines := strings.Split(encoded.String(), "\r\n")
			for i, line := range lines {
				if i == len(lines)-1 && line == "" {
					continue
				}
				if len(line) > 76 {
					t.Errorf("line %d exceeds 76 characters: got %d", i, len(line))
				}
			}

			var decoded bytes.Buffer
			err = MIMEDecode(&encoded, &decoded)
			if err != nil {
				t.Fatalf("MIMEDecode failed: %v", err)
			}

			if !bytes.Equal(tc.data, decoded.Bytes()) {
				t.Fatalf("roundtrip data mismatch: expected %d bytes, got %d bytes", len(tc.data), decoded.Len())
			}
		})
	}

	t.Run("ignore whitespaces and enters", func(t *testing.T) {
		rawOriginal := []byte("Testing MIME decoding robustness with noise.")
		var normalEncoded bytes.Buffer
		if err := MIMEEncode(bytes.NewReader(rawOriginal), &normalEncoded); err != nil {
			t.Fatalf("MIMEEncode failed: %v", err)
		}

		// Inject spaces, tabs, carriage returns, and newlines
		var dirty strings.Builder
		for i, r := range normalEncoded.String() {
			dirty.WriteRune(r)
			if i%5 == 0 {
				dirty.WriteString("  \t \r\n  ")
			}
		}

		var decoded bytes.Buffer
		if err := MIMEDecode(strings.NewReader(dirty.String()), &decoded); err != nil {
			t.Fatalf("MIMEDecode failed with dirty input: %v", err)
		}

		if !bytes.Equal(rawOriginal, decoded.Bytes()) {
			t.Errorf("dirty decoded mismatch: got %q, want %q", decoded.String(), string(rawOriginal))
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}