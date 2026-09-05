package operations

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Znaki kontrolne protokołów XMODEM / YMODEM.
const (
	SOH = 0x01 // Start of Header (blok 128 B)
	STX = 0x02 // Start of Header (blok 1024 B - YMODEM / XMODEM-1K)
	EOT = 0x04 // End of Transmission
	ACK = 0x06 // Acknowledge
	NAK = 0x15 // Negative Acknowledge
	CAN = 0x18 // Cancel
	CRC = 0x43 // 'C' - żądanie trybu CRC-16
	SUB = 0x1A // Padding byte
)

// maxRetries określa liczbę prób retransmisji pojedynczego bloku
// zanim transfer zostanie uznany za nieudany.
const maxRetries = 10

// crc16ccitt liczy CRC-16-CCITT (wielomian 0x1021, init 0x0000) używany
// przez XMODEM/YMODEM w trybie CRC.
func crc16ccitt(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// checksum8 liczy prostą sumę kontrolną modulo 256 używaną w trybie
// klasycznym (bez CRC).
func checksum8(data []byte) byte {
	var sum byte
	for _, b := range data {
		sum += b
	}
	return sum
}

// waitForByte czyta z portu pojedyncze bajty, aż napotka jeden z bajtów
// z listy valid. Zwraca ten bajt.
func waitForByte(port io.Reader, valid ...byte) (byte, error) {
	buf := make([]byte, 1)
	for {
		n, err := port.Read(buf)
		if err != nil {
			return 0, err
		}
		if n == 0 {
			continue
		}
		for _, v := range valid {
			if buf[0] == v {
				return buf[0], nil
			}
		}
	}
}

// sendAndWaitAck wysyła gotowy pakiet i czeka na ACK, retransmitując
// w razie NAK, aż do maxRetries.
func sendAndWaitAck(port io.ReadWriter, packet []byte) error {
	for i := 0; i < maxRetries; i++ {
		if _, err := port.Write(packet); err != nil {
			return err
		}
		resp, err := waitForByte(port, ACK, NAK, CAN)
		if err != nil {
			return err
		}
		switch resp {
		case ACK:
			return nil
		case CAN:
			return fmt.Errorf("xmodem: transfer cancelled by receiver")
		}
		// NAK -> retry
	}
	return fmt.Errorf("xmodem: exceeded max retries")
}

// sendXmodemBlock buduje pakiet [kind, seq, 255-seq, dane, crc/suma]
// i wysyła go, czekając na potwierdzenie.
func sendXmodemBlock(port io.ReadWriter, kind byte, seq byte, data []byte, useCRC bool) error {
	packet := make([]byte, 0, 3+len(data)+2)
	packet = append(packet, kind, seq, 255-seq)
	packet = append(packet, data...)
	if useCRC {
		crc := crc16ccitt(data)
		packet = append(packet, byte(crc>>8), byte(crc))
	} else {
		packet = append(packet, checksum8(data))
	}
	return sendAndWaitAck(port, packet)
}

// sendEOT wysyła EOT i czeka na potwierdzenie ACK, powtarzając w razie NAK.
func sendEOT(port io.ReadWriter) error {
	for i := 0; i < maxRetries; i++ {
		if _, err := port.Write([]byte{EOT}); err != nil {
			return err
		}
		resp, err := waitForByte(port, ACK, NAK)
		if err != nil {
			return err
		}
		if resp == ACK {
			return nil
		}
	}
	return fmt.Errorf("xmodem: EOT not acknowledged")
}

// xheader reprezentuje odebrany pakiet danych/EOT/CAN.
type xheader struct {
	kind byte
	seq  byte
	data []byte
}

// readXmodemPacket czyta i weryfikuje pojedynczy pakiet XMODEM/YMODEM.
// Dla EOT/CAN zwraca xheader z samym polem kind.
func readXmodemPacket(port io.Reader, useCRC bool) (*xheader, error) {
	var kindBuf [1]byte
	if _, err := io.ReadFull(port, kindBuf[:]); err != nil {
		return nil, err
	}
	kind := kindBuf[0]
	if kind == EOT || kind == CAN {
		return &xheader{kind: kind}, nil
	}
	if kind != SOH && kind != STX {
		return nil, fmt.Errorf("xmodem: unexpected byte 0x%02X", kind)
	}

	blockLen := 128
	if kind == STX {
		blockLen = 1024
	}

	hdr := make([]byte, 2)
	if _, err := io.ReadFull(port, hdr); err != nil {
		return nil, err
	}
	data := make([]byte, blockLen)
	if _, err := io.ReadFull(port, data); err != nil {
		return nil, err
	}

	crcLen := 1
	if useCRC {
		crcLen = 2
	}
	crcBuf := make([]byte, crcLen)
	if _, err := io.ReadFull(port, crcBuf); err != nil {
		return nil, err
	}

	if hdr[1] != 255-hdr[0] {
		return nil, fmt.Errorf("xmodem: bad sequence complement")
	}
	if useCRC {
		crc := crc16ccitt(data)
		if byte(crc>>8) != crcBuf[0] || byte(crc) != crcBuf[1] {
			return nil, fmt.Errorf("xmodem: crc mismatch")
		}
	} else {
		if checksum8(data) != crcBuf[0] {
			return nil, fmt.Errorf("xmodem: checksum mismatch")
		}
	}

	return &xheader{kind: kind, seq: hdr[0], data: data}, nil
}

// XmodemSend wysyła strumień danych z r przez port zgodnie z protokołem
// XMODEM (blok 128 B). Czeka na początkowy NAK lub 'C', dostosowując
// tryb CRC do żądania odbiorcy.
func XmodemSend(port io.ReadWriter, r io.Reader, useCRC bool, onProgress func(sent int64)) error {
	b, err := waitForByte(port, NAK, CRC)
	if err != nil {
		return err
	}
	useCRC = b == CRC

	br := bufio.NewReader(r)
	seq := byte(1)
	var sent int64

	for {
		buf := make([]byte, 128)
		n, rerr := io.ReadFull(br, buf)
		if rerr == io.EOF {
			break
		}
		if rerr != nil && rerr != io.ErrUnexpectedEOF {
			return rerr
		}
		if n < len(buf) {
			for i := n; i < len(buf); i++ {
				buf[i] = SUB
			}
		}
		if err := sendXmodemBlock(port, SOH, seq, buf, useCRC); err != nil {
			return err
		}
		sent += int64(n)
		if onProgress != nil {
			onProgress(sent)
		}
		seq++
		if rerr == io.ErrUnexpectedEOF {
			break
		}
	}

	return sendEOT(port)
}

// XmodemReceive odbiera dane z portu zapisując je do w, zgodnie
// z protokołem XMODEM. Inicjuje transfer wysyłając 'C' (tryb CRC)
// lub NAK (tryb sumy kontrolnej).
func XmodemReceive(port io.ReadWriter, w io.Writer, useCRC bool, onProgress func(received int64)) error {
	initByte := byte(NAK)
	if useCRC {
		initByte = CRC
	}
	if _, err := port.Write([]byte{initByte}); err != nil {
		return err
	}

	expSeq := byte(1)
	var received int64

	for {
		pkt, err := readXmodemPacket(port, useCRC)
		if err != nil {
			if _, werr := port.Write([]byte{NAK}); werr != nil {
				return werr
			}
			continue
		}

		switch pkt.kind {
		case EOT:
			if _, err := port.Write([]byte{ACK}); err != nil {
				return err
			}
			return nil
		case CAN:
			return fmt.Errorf("xmodem: transfer cancelled by sender")
		default:
			if pkt.seq != expSeq {
				// duplikat poprzedniego bloku - potwierdź ponownie
				if _, err := port.Write([]byte{ACK}); err != nil {
					return err
				}
				continue
			}
			if _, err := w.Write(pkt.data); err != nil {
				return err
			}
			received += int64(len(pkt.data))
			if onProgress != nil {
				onProgress(received)
			}
			if _, err := port.Write([]byte{ACK}); err != nil {
				return err
			}
			expSeq++
		}
	}
}

// parseYmodemHeader wyciąga nazwę pliku i rozmiar z bloku nagłówkowego
// YMODEM (blok 0). Pusta nazwa oznacza koniec wsadu transferu.
func parseYmodemHeader(data []byte) (name string, size int64, err error) {
	idx := bytes.IndexByte(data, 0)
	if idx < 0 {
		return "", 0, fmt.Errorf("ymodem: invalid header block")
	}
	name = string(data[:idx])
	if name == "" {
		return "", 0, nil
	}

	rest := data[idx+1:]
	idx2 := bytes.IndexByte(rest, 0)
	var infoStr string
	if idx2 >= 0 {
		infoStr = string(rest[:idx2])
	} else {
		infoStr = string(rest)
	}

	fields := strings.Fields(infoStr)
	if len(fields) == 0 {
		return name, 0, nil
	}
	size, err = strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("ymodem: invalid size field: %w", err)
	}
	return name, size, nil
}

// YmodemSend wysyła pojedynczy plik protokołem YMODEM: najpierw blok 0
// z metadanymi (nazwa\0rozmiar), następnie dane w blokach 1024 B (STX),
// EOT, oraz pusty blok kończący wsad.
func YmodemSend(port io.ReadWriter, filename string, r io.Reader, size int64, onProgress func(sent int64)) error {
	b, err := waitForByte(port, NAK, CRC)
	if err != nil {
		return err
	}
	useCRC := b == CRC

	// Blok 0: nazwa pliku i rozmiar.
	header := make([]byte, 128)
	name := []byte(filepath.Base(filename))
	copy(header, name)
	header[len(name)] = 0
	sizeStr := strconv.FormatInt(size, 10)
	copy(header[len(name)+1:], []byte(sizeStr))

	if err := sendXmodemBlock(port, SOH, 0, header, useCRC); err != nil {
		return err
	}

	// Odbiorca żąda rozpoczęcia transmisji danych.
	b2, err := waitForByte(port, NAK, CRC)
	if err != nil {
		return err
	}
	useCRC = b2 == CRC

	br := bufio.NewReader(r)
	seq := byte(1)
	var sent int64
	const blockSize = 1024

	for {
		buf := make([]byte, blockSize)
		n, rerr := io.ReadFull(br, buf)
		if rerr == io.EOF {
			break
		}
		if rerr != nil && rerr != io.ErrUnexpectedEOF {
			return rerr
		}
		if n < len(buf) {
			for i := n; i < len(buf); i++ {
				buf[i] = SUB
			}
		}
		if err := sendXmodemBlock(port, STX, seq, buf, useCRC); err != nil {
			return err
		}
		sent += int64(n)
		if onProgress != nil {
			onProgress(sent)
		}
		seq++
		if rerr == io.ErrUnexpectedEOF {
			break
		}
	}

	if err := sendEOT(port); err != nil {
		return err
	}

	// Odbiorca żąda kolejnego nagłówka - wysyłamy pusty blok kończący wsad.
	if _, err := waitForByte(port, NAK, CRC); err != nil {
		return err
	}
	empty := make([]byte, 128)
	if err := sendXmodemBlock(port, SOH, 0, empty, useCRC); err != nil {
		return err
	}

	return nil
}

// YmodemReceive odbiera pojedynczy plik protokołem YMODEM i zapisuje go
// w katalogu dstDir pod nazwą przesłaną w bloku 0. Zwraca pełną ścieżkę
// zapisanego pliku.
func YmodemReceive(port io.ReadWriter, dstDir string, onProgress func(received int64)) (string, error) {
	const initByte = CRC
	useCRC := true

	if _, err := port.Write([]byte{initByte}); err != nil {
		return "", err
	}

	pkt, err := readXmodemPacket(port, useCRC)
	if err != nil {
		return "", err
	}
	if pkt.kind != SOH && pkt.kind != STX {
		return "", fmt.Errorf("ymodem: expected header block, got 0x%02X", pkt.kind)
	}
	name, size, err := parseYmodemHeader(pkt.data)
	if err != nil {
		return "", err
	}
	if name == "" {
		if _, werr := port.Write([]byte{ACK}); werr != nil {
			return "", werr
		}
		return "", fmt.Errorf("ymodem: no file offered by sender")
	}
	if _, err := port.Write([]byte{ACK}); err != nil {
		return "", err
	}

	// Zażądaj rozpoczęcia transmisji danych.
	if _, err := port.Write([]byte{initByte}); err != nil {
		return "", err
	}

	dstPath := filepath.Join(dstDir, name)
	f, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	expSeq := byte(1)
	var received int64

receiveLoop:
	for {
		pkt, err := readXmodemPacket(port, useCRC)
		if err != nil {
			if _, werr := port.Write([]byte{NAK}); werr != nil {
				return "", werr
			}
			continue
		}

		switch pkt.kind {
		case EOT:
			if _, err := port.Write([]byte{ACK}); err != nil {
				return "", err
			}
			break receiveLoop
		case CAN:
			return "", fmt.Errorf("ymodem: transfer cancelled by sender")
		default:
			if pkt.seq != expSeq {
				if _, err := port.Write([]byte{ACK}); err != nil {
					return "", err
				}
				continue
			}
			toWrite := pkt.data
			if size > 0 {
				remaining := size - received
				if remaining < 0 {
					remaining = 0
				}
				if int64(len(toWrite)) > remaining {
					toWrite = toWrite[:remaining]
				}
			}
			if _, err := f.Write(toWrite); err != nil {
				return "", err
			}
			received += int64(len(toWrite))
			if onProgress != nil {
				onProgress(received)
			}
			if _, err := port.Write([]byte{ACK}); err != nil {
				return "", err
			}
			expSeq++
		}
	}

	// Zażądaj kolejnego nagłówka - sender odpowie pustym blokiem kończącym wsad.
	if _, err := port.Write([]byte{initByte}); err != nil {
		return "", err
	}
	endPkt, err := readXmodemPacket(port, useCRC)
	if err != nil {
		return "", err
	}
	_ = endPkt
	if _, err := port.Write([]byte{ACK}); err != nil {
		return "", err
	}

	return dstPath, nil
}