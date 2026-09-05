// Dzielenie plików na części oraz ich ponowne łączenie z weryfikacją
// sumy kontrolnej CRC32 w formacie Total Commander / Midnight Commander.
package operations

import (
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Typowe rozmiary części odpowiadające popularnym nośnikom danych.
const (
	ChunkSizeFloppy144 = 1457664    // 1.44 MB (dyskietka 3,5")
	ChunkSizeZip100    = 100431872  // 100 MB (Iomega Zip 100)
	ChunkSizeCD650     = 681574400  // 650 MB (CD-R 74 min)
	ChunkSizeCD700     = 734003200  // 700 MB (CD-R 80 min)
	ChunkSizeDVD47     = 4700000000 // 4.7 GB (DVD-5)
)

// copyBufferSize ogranicza zużycie pamięci: nawet wielogigabajtowe części
// kopiowane są strumieniowo, małym buforem.
const copyBufferSize = 1 << 20 // 1 MiB

// crcFileInfo zawiera dane odczytane z pliku sumy kontrolnej .crc.
type crcFileInfo struct {
	filename string
	size     int64
	crc32    uint32
}

// SplitFile dzieli plik srcPath na części o maksymalnym rozmiarze chunkSize
// bajtów i zapisuje je w katalogu dstDir pod nazwami <nazwa>.001, <nazwa>.002
// itd. (numeracja od 1, format %03d).
//
// Suma kontrolna CRC32 (IEEE) całego oryginalnego pliku obliczana jest
// równolegle z zapisem części — w jednym przebiegu, bez ponownego odczytu
// źródła — i zapisywana do pliku <nazwa>.crc w formacie
// Total Commander / Midnight Commander:
//
//	filename=<nazwa>
//	size=<rozmiar w bajtach>
//	crc32=<suma jako 8 cyfr szesnastkowych>
//
// onProgress (o ile nie nil) wywoływane jest po zapisaniu każdej części
// z jej numerem oraz liczbą zapisanych w niej bajtów.
//
// Zwraca ścieżki utworzonych części (bez pliku .crc). W razie błędu części
// utworzone do tej pory są usuwane.
func SplitFile(srcPath, dstDir string, chunkSize int64, onProgress func(part int, bytesWritten int64)) ([]string, error) {
	if chunkSize <= 0 {
		return nil, fmt.Errorf("operations: chunk size must be positive, got %d", chunkSize)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("operations: cannot open source file %s: %w", srcPath, err)
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return nil, fmt.Errorf("operations: cannot stat source file %s: %w", srcPath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("operations: source %s is a directory", srcPath)
	}

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return nil, fmt.Errorf("operations: cannot create destination directory %s: %w", dstDir, err)
	}

	baseName := filepath.Base(srcPath)
	totalSize := info.Size()
	hasher := crc32.NewIEEE()
	buf := make([]byte, copyBufferSize)

	parts := make([]string, 0, 8)
	cleanup := func() {
		for _, p := range parts {
			_ = os.Remove(p)
		}
	}

	// Pętla wykonuje się co najmniej raz, więc nawet pusty plik źródłowy
	// daje jedną (pustą) część .001 i poprawnie składa się z powrotem.
	partNum := 0
	remaining := totalSize
	for remaining > 0 || partNum == 0 {
		partNum++
		want := chunkSize
		if remaining < want {
			want = remaining
		}

		partPath := filepath.Join(dstDir, fmt.Sprintf("%s.%03d", baseName, partNum))
		part, err := os.Create(partPath)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("operations: cannot create part %s: %w", partPath, err)
		}
		parts = append(parts, partPath)

		// MultiWriter: CRC32 liczone równolegle z zapisem części na dysk.
		written, copyErr := io.CopyBuffer(io.MultiWriter(part, hasher), io.LimitReader(src, want), buf)
		closeErr := part.Close()
		switch {
		case copyErr != nil:
			cleanup()
			return nil, fmt.Errorf("operations: cannot write part %s: %w", partPath, copyErr)
		case closeErr != nil:
			cleanup()
			return nil, fmt.Errorf("operations: cannot close part %s: %w", partPath, closeErr)
		case written != want:
			cleanup()
			return nil, fmt.Errorf("operations: unexpected end of %s while writing %s: %w", srcPath, partPath, io.ErrUnexpectedEOF)
		}

		remaining -= written
		if onProgress != nil {
			onProgress(partNum, written)
		}
	}

	crcPath := filepath.Join(dstDir, baseName+".crc")
	crcContent := fmt.Sprintf("filename=%s\nsize=%d\ncrc32=%08X\n", baseName, totalSize, hasher.Sum32())
	if err := os.WriteFile(crcPath, []byte(crcContent), 0o644); err != nil {
		cleanup()
		return nil, fmt.Errorf("operations: cannot write checksum file %s: %w", crcPath, err)
	}

	return parts, nil
}

// JoinFiles składa części <nazwa>.001, <nazwa>.002, ... (aż do pierwszego
// brakującego numeru) w jeden plik <nazwa> w katalogu dstDir.
//
// firstPartPath może wskazywać pierwszą część (.001) albo plik sumy
// kontrolnej (.crc); części są szukane w katalogu, w którym znajduje się
// firstPartPath. Jeśli w tym katalogu istnieje plik .crc, rozmiar i suma
// CRC32 połączonego pliku są weryfikowane — niezgodność powoduje błąd,
// a wadliwy plik wynikowy jest usuwany.
//
// onProgress (o ile nie nil) wywoływane jest po odczytaniu każdej części
// z jej numerem oraz liczbą odczytanych z niej bajtów.
//
// Zwraca ścieżkę połączonego pliku.
func JoinFiles(firstPartPath, dstDir string, onProgress func(part int, bytesRead int64)) (string, error) {
	srcDir := filepath.Dir(firstPartPath)
	name := filepath.Base(firstPartPath)

	var baseName, crcPath string
	switch {
	case len(name) > 4 && strings.EqualFold(name[len(name)-4:], ".crc"):
		baseName = name[:len(name)-4]
		crcPath = firstPartPath
	case len(name) > 4 && isPartExtension(name[len(name)-4:]):
		baseName = name[:len(name)-4]
		crcPath = filepath.Join(srcDir, baseName+".crc")
	default:
		return "", fmt.Errorf("operations: %s is neither a .crc checksum file nor a .001 part", firstPartPath)
	}

	firstPart := filepath.Join(srcDir, baseName+".001")
	if _, err := os.Stat(firstPart); err != nil {
		return "", fmt.Errorf("operations: cannot access first part %s: %w", firstPart, err)
	}

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", fmt.Errorf("operations: cannot create destination directory %s: %w", dstDir, err)
	}

	outPath := filepath.Join(dstDir, baseName)
	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("operations: cannot create output file %s: %w", outPath, err)
	}

	hasher := crc32.NewIEEE()
	buf := make([]byte, copyBufferSize)
	var totalRead int64

	for partNum := 1; ; partNum++ {
		partPath := filepath.Join(srcDir, fmt.Sprintf("%s.%03d", baseName, partNum))
		part, err := os.Open(partPath)
		if err != nil {
			if os.IsNotExist(err) {
				break // brak kolejnej części — koniec składania
			}
			out.Close()
			os.Remove(outPath)
			return "", fmt.Errorf("operations: cannot open part %s: %w", partPath, err)
		}

		// MultiWriter: CRC32 liczone równolegle z zapisem wyniku.
		n, copyErr := io.CopyBuffer(io.MultiWriter(out, hasher), part, buf)
		closeErr := part.Close()
		if copyErr != nil || closeErr != nil {
			out.Close()
			os.Remove(outPath)
			if copyErr != nil {
				return "", fmt.Errorf("operations: cannot read part %s: %w", partPath, copyErr)
			}
			return "", fmt.Errorf("operations: cannot close part %s: %w", partPath, closeErr)
		}

		totalRead += n
		if onProgress != nil {
			onProgress(partNum, n)
		}
	}

	if err := out.Close(); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("operations: cannot close output file %s: %w", outPath, err)
	}

	// Weryfikacja względem pliku .crc, jeśli istnieje w katalogu części.
	if _, err := os.Stat(crcPath); err == nil {
		expected, err := parseCRCFile(crcPath)
		if err != nil {
			os.Remove(outPath)
			return "", err
		}
		if expected.size != totalRead {
			os.Remove(outPath)
			return "", fmt.Errorf("operations: size mismatch for %s: expected %d bytes, got %d", baseName, expected.size, totalRead)
		}
		if expected.crc32 != hasher.Sum32() {
			os.Remove(outPath)
			return "", fmt.Errorf("operations: CRC32 mismatch for %s: expected %08X, got %08X", baseName, expected.crc32, hasher.Sum32())
		}
	}

	return outPath, nil
}

// isPartExtension zgłasza, czy rozszerzenie składa się z kropki i dokładnie
// trzech cyfr (np. ".001", ".042").
func isPartExtension(ext string) bool {
	if len(ext) != 4 || ext[0] != '.' {
		return false
	}
	for i := 1; i < len(ext); i++ {
		if ext[i] < '0' || ext[i] > '9' {
			return false
		}
	}
	return true
}

// parseCRCFile odczytuje plik sumy kontrolnej w formacie
// Total Commander / Midnight Commander (linie „klucz=wartość”).
func parseCRCFile(path string) (crcFileInfo, error) {
	var info crcFileInfo
	data, err := os.ReadFile(path)
	if err != nil {
		return info, fmt.Errorf("operations: cannot read checksum file %s: %w", path, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "filename":
			info.filename = strings.TrimSpace(value)
		case "size":
			size, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return info, fmt.Errorf("operations: invalid size in checksum file %s: %w", path, err)
			}
			info.size = size
		case "crc32":
			sum, err := strconv.ParseUint(strings.TrimSpace(value), 16, 32)
			if err != nil {
				return info, fmt.Errorf("operations: invalid crc32 in checksum file %s: %w", path, err)
			}
			info.crc32 = uint32(sum)
		}
	}
	return info, nil
}