package vfs

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testFiles to zawartość plików umieszczanych w testowych archiwach.
var testFiles = map[string]string{
	"readme.md":        "README zawartość\n",
	"dir/hello.txt":    "Hello, TAR!",
	"dir/sub/deep.txt": "głęboka zawartość",
}

// createTestTar tworzy archiwum TAR (opcjonalnie skompresowane gzipem)
// zawierające katalogi jawne ("dir/", "empty/") oraz katalog niejawny
// ("dir/sub" — bez własnego nagłówka).
func createTestTar(t *testing.T, dst string, useGzip bool) {
	t.Helper()

	f, err := os.Create(dst)
	if err != nil {
		t.Fatalf("nie można utworzyć %s: %v", dst, err)
	}
	defer f.Close()

	var w io.Writer = f
	var gz *gzip.Writer
	if useGzip {
		gz = gzip.NewWriter(f)
		w = gz
	}

	tw := tar.NewWriter(w)
	now := time.Now().Truncate(time.Second)

	writeDir := func(name string) {
		t.Helper()
		hdr := &tar.Header{
			Name:     name,
			Typeflag: tar.TypeDir,
			Mode:     0o755,
			ModTime:  now,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%s): %v", name, err)
		}
	}

	writeFile := func(name, content string) {
		t.Helper()
		hdr := &tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
			ModTime:  now,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%s): %v", name, err)
		}
		if _, err := io.WriteString(tw, content); err != nil {
			t.Fatalf("Write(%s): %v", name, err)
		}
	}

	writeFile("./readme.md", testFiles["readme.md"]) // celowo z prefiksem "./"
	writeDir("dir/")
	writeFile("dir/hello.txt", testFiles["dir/hello.txt"])
	writeFile("dir/sub/deep.txt", testFiles["dir/sub/deep.txt"])
	writeDir("empty/")

	if err := tw.Close(); err != nil {
		t.Fatalf("tar.Close: %v", err)
	}
	if gz != nil {
		if err := gz.Close(); err != nil {
			t.Fatalf("gzip.Close: %v", err)
		}
	}
}

// findEntry wyszukuje wpis po nazwie.
func findEntry(entries []*FileEntry, name string) *FileEntry {
	for _, e := range entries {
		if e.Name == name {
			return e
		}
	}
	return nil
}

func names(entries []*FileEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

func TestNewTarVFS_PlainTar(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	if !v.IsArchive() {
		t.Error("IsArchive() = false, oczekiwano true")
	}
	if got, want := v.Path(), archive+"::/"; got != want {
		t.Errorf("Path() = %q, oczekiwano %q", got, want)
	}
}

func TestNewTarVFS_GzipTar(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"test.tar.gz", "test.tgz"} {
		archive := filepath.Join(dir, name)
		createTestTar(t, archive, true)

		v, err := NewTarVFS(archive)
		if err != nil {
			t.Fatalf("NewTarVFS(%s): %v", name, err)
		}

		entries, err := v.List()
		if err != nil {
			t.Fatalf("List(%s): %v", name, err)
		}
		if len(entries) != 3 {
			t.Errorf("%s: List() = %v, oczekiwano 3 wpisów", name, names(entries))
		}

		rc, err := v.Open("dir/hello.txt")
		if err != nil {
			t.Fatalf("%s: Open: %v", name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("%s: ReadAll: %v", name, err)
		}
		if string(data) != testFiles["dir/hello.txt"] {
			t.Errorf("%s: zawartość = %q, oczekiwano %q", name, data, testFiles["dir/hello.txt"])
		}
		v.Close()
	}
}

func TestNewTarVFS_SignatureDetection(t *testing.T) {
	// Plik z rozszerzeniem ".tar", ale faktycznie skompresowany gzipem —
	// format powinien zostać rozpoznany po sygnaturze.
	dir := t.TempDir()
	archive := filepath.Join(dir, "mislabeled.tar")
	createTestTar(t, archive, true)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	if _, err := v.Stat("readme.md"); err != nil {
		t.Errorf("Stat(readme.md): %v", err)
	}
}

func TestTarVFS_ListRoot(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	entries, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("List() = %v, oczekiwano [dir empty readme.md]", names(entries))
	}
	if findEntry(entries, "..") != nil {
		t.Error("korzeń archiwum nie powinien zawierać wpisu '..'")
	}

	// Katalogi przed plikami.
	if !entries[0].IsDir || !entries[1].IsDir || entries[2].IsDir {
		t.Errorf("nieprawidłowa kolejność sortowania: %v", names(entries))
	}

	readme := findEntry(entries, "readme.md")
	if readme == nil {
		t.Fatal("brak wpisu readme.md")
	}
	if readme.IsDir {
		t.Error("readme.md nie powinien być katalogiem")
	}
	if want := int64(len(testFiles["readme.md"])); readme.Size != want {
		t.Errorf("readme.md Size = %d, oczekiwano %d", readme.Size, want)
	}
	if want := archive + "::/readme.md"; readme.Path != want {
		t.Errorf("readme.md Path = %q, oczekiwano %q", readme.Path, want)
	}

	empty := findEntry(entries, "empty")
	if empty == nil || !empty.IsDir {
		t.Errorf("oczekiwano katalogu 'empty', otrzymano %v", names(entries))
	}
}

func TestTarVFS_SetPathAndList(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	if err := v.SetPath("dir"); err != nil {
		t.Fatalf("SetPath(dir): %v", err)
	}
	if got, want := v.Path(), archive+"::/dir"; got != want {
		t.Errorf("Path() = %q, oczekiwano %q", got, want)
	}

	entries, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("List() = %v, oczekiwano 3 wpisów", names(entries))
	}
	if entries[0].Name != ".." || !entries[0].IsDir {
		t.Errorf("pierwszy wpis = %q, oczekiwano '..'", entries[0].Name)
	}
	if e := findEntry(entries, "hello.txt"); e == nil || e.IsDir {
		t.Errorf("brak pliku hello.txt w %v", names(entries))
	}
	// Katalog wirtualny — brak własnego nagłówka w archiwum.
	if e := findEntry(entries, "sub"); e == nil || !e.IsDir {
		t.Errorf("brak katalogu wirtualnego 'sub' w %v", names(entries))
	}

	// Pusty katalog listuje się poprawnie (tylko "..").
	if err := v.SetPath("empty"); err != nil {
		t.Fatalf("SetPath(empty): %v", err)
	}
	entries, err = v.List()
	if err != nil {
		t.Fatalf("List(empty): %v", err)
	}
	if len(entries) != 1 || entries[0].Name != ".." {
		t.Errorf("List(empty) = %v, oczekiwano tylko '..'", names(entries))
	}

	// Powrót do korzenia.
	if err := v.SetPath(""); err != nil {
		t.Fatalf("SetPath(\"\"): %v", err)
	}
	if got, want := v.Path(), archive+"::/"; got != want {
		t.Errorf("Path() = %q, oczekiwano %q", got, want)
	}
}

func TestTarVFS_SetPathErrors(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	if err := v.SetPath("nie/ma/takiego"); err == nil {
		t.Error("SetPath na nieistniejący katalog powinien zwrócić błąd")
	}
	if err := v.SetPath("readme.md"); err == nil {
		t.Error("SetPath na plik powinien zwrócić błąd")
	}
}

func TestTarVFS_Parent(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	if err := v.SetPath("dir/sub"); err != nil {
		t.Fatalf("SetPath(dir/sub): %v", err)
	}
	if got, want := v.Path(), archive+"::/dir/sub"; got != want {
		t.Errorf("Path() = %q, oczekiwano %q", got, want)
	}

	if err := v.Parent(); err != nil {
		t.Fatalf("Parent: %v", err)
	}
	if got, want := v.Path(), archive+"::/dir"; got != want {
		t.Errorf("po Parent() Path() = %q, oczekiwano %q", got, want)
	}

	if err := v.Parent(); err != nil {
		t.Fatalf("Parent: %v", err)
	}
	if got, want := v.Path(), archive+"::/"; got != want {
		t.Errorf("po Parent() Path() = %q, oczekiwano %q", got, want)
	}

	if err := v.Parent(); err == nil {
		t.Error("Parent() w korzeniu archiwum powinien zwrócić błąd")
	}
}

func TestTarVFS_Open(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	// Odczyt po ścieżce względem korzenia.
	for name, want := range testFiles {
		rc, err := v.Open(name)
		if err != nil {
			t.Fatalf("Open(%s): %v", name, err)
		}
		got, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("ReadAll(%s): %v", name, err)
		}
		if string(got) != want {
			t.Errorf("zawartość %s = %q, oczekiwano %q", name, got, want)
		}
	}

	// Odczyt po nazwie względnej w bieżącym podkatalogu.
	if err := v.SetPath("dir/sub"); err != nil {
		t.Fatalf("SetPath: %v", err)
	}
	rc, err := v.Open("deep.txt")
	if err != nil {
		t.Fatalf("Open(deep.txt): %v", err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != testFiles["dir/sub/deep.txt"] {
		t.Errorf("zawartość = %q, oczekiwano %q", got, testFiles["dir/sub/deep.txt"])
	}

	// Wielokrotny odczyt tego samego pliku musi dawać ten sam wynik.
	rc2, err := v.Open("deep.txt")
	if err != nil {
		t.Fatalf("ponowny Open: %v", err)
	}
	got2, _ := io.ReadAll(rc2)
	rc2.Close()
	if string(got2) != string(got) {
		t.Errorf("ponowny odczyt = %q, oczekiwano %q", got2, got)
	}
}

func TestTarVFS_OpenErrors(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	if _, err := v.Open("nie_ma.txt"); err == nil {
		t.Error("Open na nieistniejący plik powinien zwrócić błąd")
	}
	if _, err := v.Open("dir"); err == nil {
		t.Error("Open na katalog powinien zwrócić błąd")
	}
}

func TestTarVFS_Stat(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	fe, err := v.Stat("dir/hello.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fe.Name != "hello.txt" {
		t.Errorf("Name = %q, oczekiwano \"hello.txt\"", fe.Name)
	}
	if fe.IsDir {
		t.Error("IsDir = true, oczekiwano false")
	}
	if want := int64(len(testFiles["dir/hello.txt"])); fe.Size != want {
		t.Errorf("Size = %d, oczekiwano %d", fe.Size, want)
	}

	if fe, err = v.Stat("dir/sub"); err != nil {
		t.Fatalf("Stat(dir/sub): %v", err)
	}
	if !fe.IsDir {
		t.Error("dir/sub powinien być katalogiem")
	}

	if _, err := v.Stat("brak.txt"); err == nil {
		t.Error("Stat na nieistniejący wpis powinien zwrócić błąd")
	}
}

func TestTarVFS_ReadOnlyOperations(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	const want = "archiwum TAR jest tylko do odczytu"

	if _, err := v.Create("nowy.txt"); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Create err = %v, oczekiwano %q", err, want)
	}
	if err := v.Mkdir("nowy"); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Mkdir err = %v, oczekiwano %q", err, want)
	}
	if err := v.Remove("readme.md"); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Remove err = %v, oczekiwano %q", err, want)
	}
	if err := v.RemoveAll("dir"); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("RemoveAll err = %v, oczekiwano %q", err, want)
	}
	if err := v.Rename("readme.md", "x.md"); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Rename err = %v, oczekiwano %q", err, want)
	}
}

func TestNewTarVFS_Errors(t *testing.T) {
	dir := t.TempDir()

	if _, err := NewTarVFS(filepath.Join(dir, "nie_ma.tar")); err == nil {
		t.Error("NewTarVFS na nieistniejący plik powinien zwrócić błąd")
	}

	if _, err := NewTarVFS(dir); err == nil {
		t.Error("NewTarVFS na katalog powinien zwrócić błąd")
	}

	broken := filepath.Join(dir, "broken.tar")
	if err := os.WriteFile(broken, []byte("to nie jest archiwum tar"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := NewTarVFS(broken); err == nil {
		t.Error("NewTarVFS na uszkodzone archiwum powinien zwrócić błąd")
	}

	brokenGz := filepath.Join(dir, "broken.tar.gz")
	if err := os.WriteFile(brokenGz, []byte("nie gzip"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := NewTarVFS(brokenGz); err == nil {
		t.Error("NewTarVFS na uszkodzony gzip powinien zwrócić błąd")
	}
}

func TestTarVFS_Close(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	if err := v.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if v.entries != nil || v.subDirs != nil {
		t.Error("Close() powinien zwolnić indeks archiwum")
	}
	if _, err := v.List(); err == nil {
		t.Error("List() po Close() powinien zwrócić błąd")
	}
	if _, err := v.Open("readme.md"); err == nil {
		t.Error("Open() po Close() powinien zwrócić błąd")
	}
}

func TestTarVFS_ListReturnsCopies(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "test.tar")
	createTestTar(t, archive, false)

	v, err := NewTarVFS(archive)
	if err != nil {
		t.Fatalf("NewTarVFS: %v", err)
	}
	defer v.Close()

	first, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	first[0].Selected = true

	second, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if second[0].Selected {
		t.Error("modyfikacja wyniku List() nie powinna wpływać na indeks archiwum")
	}
}

func TestNormalizeArchivePath(t *testing.T) {
	cases := map[string]string{
		"":              "",
		".":             "",
		"./":            "",
		"/":             "",
		"./dir/":        "dir",
		"dir//sub/":     "dir/sub",
		"/dir/plik.txt": "dir/plik.txt",
		"..":            "",
		"../etc/passwd": "",
		"dir/../plik":   "plik",
	}
	for in, want := range cases {
		if got := normalizeArchivePath(in); got != want {
			t.Errorf("normalizeArchivePath(%q) = %q, oczekiwano %q", in, got, want)
		}
	}
}

func TestParentDir(t *testing.T) {
	cases := map[string]string{
		"":            "",
		"plik.txt":    "",
		"dir/plik":    "dir",
		"a/b/c/plik":  "a/b/c",
	}
	for in, want := range cases {
		if got := parentDir(in); got != want {
			t.Errorf("parentDir(%q) = %q, oczekiwano %q", in, got, want)
		}
	}
}

func TestDetectCompression(t *testing.T) {
	cases := []struct {
		name string
		sig  []byte
		want compression
	}{
		{"a.tar", nil, compNone},
		{"a.tar.gz", nil, compGzip},
		{"a.tgz", nil, compGzip},
		{"a.tar.bz2", nil, compBzip2},
		{"a.tbz2", nil, compBzip2},
		{"a.tar", []byte{0x1f, 0x8b, 0x08, 0x00}, compGzip},
		{"a.tar", []byte{'B', 'Z', 'h', '9'}, compBzip2},
	}
	for _, c := range cases {
		if got := detectCompression(c.name, c.sig); got != c.want {
			t.Errorf("detectCompression(%q) = %v, oczekiwano %v", c.name, got, c.want)
		}
	}
}