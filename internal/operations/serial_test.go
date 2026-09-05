package operations

import (
	"bytes"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// pipePort łączy parę io.Pipe w jeden io.ReadWriter, symulując jeden
// koniec kabla szeregowego null-modem.
type pipePort struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func (p *pipePort) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p *pipePort) Write(b []byte) (int, error) { return p.w.Write(b) }

// newNullModemPair tworzy dwa końce dwukierunkowego, symulowanego
// kabla null-modem oparte na dwóch io.Pipe.
func newNullModemPair() (*pipePort, *pipePort) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	portA := &pipePort{r: r1, w: w2}
	portB := &pipePort{r: r2, w: w1}
	return portA, portB
}

func TestXmodemTransferChecksum(t *testing.T) {
	testXmodemTransfer(t, false)
}

func TestXmodemTransferCRC(t *testing.T) {
	testXmodemTransfer(t, true)
}

func testXmodemTransfer(t *testing.T, useCRC bool) {
	t.Helper()

	// Rozmiar będący dokładną wielokrotnością 128 B, aby uniknąć
	// dopełnienia SUB na końcu i uprościć porównanie danych.
	data := make([]byte, 128*3)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	senderPort, receiverPort := newNullModemPair()

	var wg sync.WaitGroup
	var sendErr, recvErr error
	var out bytes.Buffer

	wg.Add(2)
	go func() {
		defer wg.Done()
		sendErr = XmodemSend(senderPort, bytes.NewReader(data), useCRC, nil)
	}()
	go func() {
		defer wg.Done()
		recvErr = XmodemReceive(receiverPort, &out, useCRC, nil)
	}()
	wg.Wait()

	if sendErr != nil {
		t.Fatalf("XmodemSend error: %v", sendErr)
	}
	if recvErr != nil {
		t.Fatalf("XmodemReceive error: %v", recvErr)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("data mismatch: got %d bytes, want %d bytes", out.Len(), len(data))
	}
}

func TestYmodemTransfer(t *testing.T) {
	// Rozmiar niewyrównany do bloku 1024 B, aby przetestować obcinanie
	// dopełnienia na podstawie zadeklarowanego rozmiaru pliku.
	data := make([]byte, 1024*2+500)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	const filename = "testfile.bin"

	dstDir := t.TempDir()

	senderPort, receiverPort := newNullModemPair()

	var wg sync.WaitGroup
	var sendErr, recvErr error
	var receivedPath string

	wg.Add(2)
	go func() {
		defer wg.Done()
		sendErr = YmodemSend(senderPort, filename, bytes.NewReader(data), int64(len(data)), nil)
	}()
	go func() {
		defer wg.Done()
		receivedPath, recvErr = YmodemReceive(receiverPort, dstDir, nil)
	}()
	wg.Wait()

	if sendErr != nil {
		t.Fatalf("YmodemSend error: %v", sendErr)
	}
	if recvErr != nil {
		t.Fatalf("YmodemReceive error: %v", recvErr)
	}

	wantPath := filepath.Join(dstDir, filename)
	if receivedPath != wantPath {
		t.Fatalf("received path = %q, want %q", receivedPath, wantPath)
	}

	got, err := os.ReadFile(receivedPath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("data mismatch: got %d bytes, want %d bytes", len(got), len(data))
	}
}

func TestXmodemProgressCallback(t *testing.T) {
	data := make([]byte, 128*2)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	senderPort, receiverPort := newNullModemPair()

	var wg sync.WaitGroup
	var sendErr, recvErr error
	var out bytes.Buffer
	var sentProgress, recvProgress int64

	wg.Add(2)
	go func() {
		defer wg.Done()
		sendErr = XmodemSend(senderPort, bytes.NewReader(data), true, func(sent int64) {
			sentProgress = sent
		})
	}()
	go func() {
		defer wg.Done()
		recvErr = XmodemReceive(receiverPort, &out, true, func(received int64) {
			recvProgress = received
		})
	}()
	wg.Wait()

	if sendErr != nil {
		t.Fatalf("XmodemSend error: %v", sendErr)
	}
	if recvErr != nil {
		t.Fatalf("XmodemReceive error: %v", recvErr)
	}
	if sentProgress != int64(len(data)) {
		t.Fatalf("sentProgress = %d, want %d", sentProgress, len(data))
	}
	if recvProgress != int64(len(data)) {
		t.Fatalf("recvProgress = %d, want %d", recvProgress, len(data))
	}
}